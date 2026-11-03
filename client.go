package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/headzoo/surf/browser"
	"gopkg.in/headzoo/surf.v1"
)

type Address struct {
	Email   string `json:"email"`
	Memo    string `json:"memo"`
	Expires string `json:"expires"`
}

type Client struct {
	browser *browser.Browser
}

type FormPayload map[string]string

func NewClient(username, password string) (*Client, error) {
	client := &Client{browser: surf.NewBrowser()}
	client.browser.AddRequestHeader("Accept-Language", "en-US,en;q=0.5")

	err := client.browser.Open("https://manage.mailbox.org/login.php?redirect=account_disposable_aliases")
	if err != nil {
		return nil, err
	}
	if client.browser.StatusCode() != 200 {
		return nil, fmt.Errorf("open login page: HTTP %d", client.browser.StatusCode())
	}

	fm, err := client.browser.Form("#io-ox-login-form")
	if err != nil {
		return nil, fmt.Errorf("find login form: %w", err)
	}
	if err := fm.Input("username", username); err != nil {
		return nil, fmt.Errorf("set login username: %w", err)
	}
	if err := fm.Input("password", password); err != nil {
		return nil, fmt.Errorf("set login password: %w", err)
	}
	if err := fm.Submit(); err != nil {
		return nil, fmt.Errorf("submit login form: %w", err)
	}
	if client.browser.StatusCode() != 200 {
		return nil, fmt.Errorf("submit login form: HTTP %d", client.browser.StatusCode())
	}

	errorBox := client.browser.Find("#io-ox-login-container .error")

	if errorBox.Length() > 0 {
		return nil, fmt.Errorf("%s", strings.TrimSpace(errorBox.Text()))
	}
	if client.browser.Find("#io-ox-login-form").Length() > 0 {
		return nil, fmt.Errorf("login failed: login form is still present")
	}
	if client.browser.Find(".ox-list").Length() == 0 {
		return nil, fmt.Errorf("login failed: disposable-address list not found")
	}

	return client, nil
}

func (c *Client) List() []Address {
	addresses := []Address{}

	c.browser.Find(".ox-list li").Each(func(_ int, s *goquery.Selection) {
		addresses = append(addresses, Address{
			Email:   s.Find(".title").Text(),
			Memo:    s.Find(".memo #memo").AttrOr("value", ""),
			Expires: expiresTextToISO8061Date(s.Find(".content div").Text()),
		})
	})

	return addresses
}

func (c *Client) Create(memo string) (Address, error) {
	err := c.executeAction(FormPayload{"action": "create"})
	if err != nil {
		return Address{}, err
	}

	addresses := c.List()
	newAddress := addresses[len(addresses)-1]

	if memo != "" {
		_, err = c.SetMemo(newAddress.Email, memo)
		if err != nil {
			return Address{}, err
		}
	}

	errorBox := c.browser.Find("#content .error")
	if errorBox.Length() > 0 {
		return Address{}, fmt.Errorf("%s", strings.TrimSpace(errorBox.Text()))
	}

	return c.findAddressByEmail(newAddress.Email), nil
}

func (c *Client) Renew(email string) (Address, error) {
	err := c.executeAction(FormPayload{"action": "renew", "id": email})
	if err != nil {
		return Address{}, err
	}

	return c.findAddressByEmail(email), nil
}

func (c *Client) RenewAll() ([]Address, error) {
	// get all emails and fill into a slice
	emails := []string{}
	for _, address := range c.List() {
		emails = append(emails, address.Email)
	}

	// executeAction renew for each email
	for _, email := range emails {
		err := c.executeAction(FormPayload{"action": "renew", "id": email})
		if err != nil {
			return c.List(), err
		}
	}

	fmt.Println("Emails renewed:")
	return c.List(), nil

}

func (c *Client) SetMemo(email, memo string) (Address, error) {
	err := c.executeAction(FormPayload{"action": "edit_memo", "id": email, "memo": memo})
	if err != nil {
		return Address{}, err
	}

	return c.findAddressByEmail(email), nil
}

func (c *Client) Delete(email string) error {
	return c.executeAction(FormPayload{"action": "delete", "id": email})
}

func (c *Client) executeAction(data FormPayload) error {
	values := url.Values{}

	for key, val := range data {
		values.Set(key, val)
	}

	return c.browser.PostForm("https://manage.mailbox.org/index.php?p=account_disposable_aliases", values)
}

func (c *Client) findAddressByEmail(email string) Address {
	for _, address := range c.List() {
		if address.Email == email {
			return address
		}
	}

	return Address{}
}

const EXPIRES_DATE_LAYOUT = "2 Jan, 2006"
const ISO8061_DATE_LAYOUT = "2006-01-02"

var re *regexp.Regexp = regexp.MustCompile(`\d{1,2} \w{3}, \d{4}$`)

func expiresTextToISO8061Date(expires string) string {
	match := re.FindString(expires)
	t, _ := time.Parse(EXPIRES_DATE_LAYOUT, match)

	return t.Format(ISO8061_DATE_LAYOUT)
}
