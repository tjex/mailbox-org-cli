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

	fm, _ := client.browser.Form("#io-ox-login-form")
	err = fm.Input("username", username)
	if fm.Submit() != nil {
		return nil, err
	}

	err = fm.Input("password", password)
	if fm.Submit() != nil {
		return nil, err
	}

	if fm.Submit() != nil {
		return nil, err
	}

	errorBox := client.browser.Find("#io-ox-login-container .error")

	if errorBox.Length() > 0 {
		return nil, fmt.Errorf(strings.TrimSpace(errorBox.Text()))
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
		return Address{}, fmt.Errorf(strings.TrimSpace(errorBox.Text()))
	}

	return c.findAddressByID(newAddress.Email), nil
}

func (c *Client) Renew(id string) (Address, error) {
	err := c.executeAction(FormPayload{"action": "renew", "id": id})
	if err != nil {
		return Address{}, err
	}

	return c.findAddressByID(id), nil
}

func (c *Client) SetMemo(id, memo string) (Address, error) {
	err := c.executeAction(FormPayload{"action": "edit_memo", "id": id, "memo": memo})
	if err != nil {
		return Address{}, err
	}

	return c.findAddressByID(id), nil
}

func (c *Client) Delete(id string) error {
	return c.executeAction(FormPayload{"action": "delete", "id": id})
}

func (c *Client) executeAction(data FormPayload) error {
	values := url.Values{}

	for key, val := range data {
		values.Set(key, val)
	}

	return c.browser.PostForm("https://manage.mailbox.org/index.php?p=account_disposable_aliases", values)
}

func (c *Client) findAddressByID(id string) Address {
	for _, address := range c.List() {
		if address.Email == id {
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
