package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

const loginFormHTML = `<div id="io-ox-login-container">
<form id="io-ox-login-form" action="login.php?redirect=account_disposable_aliases" method="post">
<input name="username"><input name="password" type="password">
<input name="submit" type="submit" value="Login">
</form></div>`

func TestNewClientSubmitsBothCredentialsOnce(t *testing.T) {
	var submissions atomic.Int32
	managementServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login.php" && r.Method == http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "test-session", Path: "/"})
			fmt.Fprint(w, loginFormHTML)
		case r.URL.Path == "/login.php" && r.Method == http.MethodPost:
			submissions.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			cookie, err := r.Cookie("PHPSESSID")
			if err != nil || cookie.Value != "test-session" ||
				r.PostForm.Get("username") != "user@example.com" ||
				r.PostForm.Get("password") != "test-password" {
				fmt.Fprint(w, strings.Replace(loginFormHTML, "</div>", `<p class="error">Invalid login</p></div>`, 1))
				return
			}
			http.Redirect(w, r, "/index.php?p=account_disposable_aliases", http.StatusSeeOther)
		case r.URL.Path == "/index.php" && r.URL.Query().Get("p") == "account_disposable_aliases":
			fmt.Fprint(w, `<main id="content"><ul class="ox-list"><li>
<span class="title">example@temp.mailbox.org</span>
<div class="memo"><input id="memo" value="test memo"></div>
<div class="content"><div>Expires: 3 Nov, 2026</div></div>
</li></ul></main>`)
		default:
			http.NotFound(w, r)
		}
	}))

	client, err := NewClient("user@example.com", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	if got := submissions.Load(); got != 1 {
		t.Errorf("login submissions = %d, want 1", got)
	}
	want := []Address{{
		Email:   "example@temp.mailbox.org",
		Memo:    "test memo",
		Expires: "2026-11-03",
	}}
	if got := client.List(); !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestNewClientRejectsFailedLogin(t *testing.T) {
	for _, tt := range []struct {
		name        string
		initialPage string
		loginPage   string
		status      int
		wantError   string
	}{
		{
			name:        "missing login form",
			initialPage: `<main>Login has moved</main>`,
			wantError:   "login form",
		},
		{
			name:        "invalid credentials",
			initialPage: loginFormHTML,
			loginPage:   `<div id="io-ox-login-container"><div class="error"> Password 100% invalid </div></div>`,
			status:      http.StatusOK,
			wantError:   "Password 100% invalid",
		},
		{
			name:        "login form remains",
			initialPage: loginFormHTML,
			loginPage:   loginFormHTML,
			status:      http.StatusOK,
			wantError:   "login",
		},
		{
			name:        "unexpected destination",
			initialPage: loginFormHTML,
			loginPage:   `<main>Additional authentication required</main>`,
			status:      http.StatusOK,
			wantError:   "disposable",
		},
		{
			name:        "initial HTTP failure",
			initialPage: `<main>Service unavailable</main>`,
			status:      http.StatusServiceUnavailable,
			wantError:   "503",
		},
		{
			name:        "submit HTTP failure",
			initialPage: loginFormHTML,
			loginPage:   `<main>Service unavailable</main>`,
			status:      http.StatusServiceUnavailable,
			wantError:   "503",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			managementServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					if tt.loginPage == "" && tt.status != 0 {
						w.WriteHeader(tt.status)
					}
					fmt.Fprint(w, tt.initialPage)
					return
				}
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.loginPage)
			}))

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("NewClient panicked instead of returning a login error: %v", recovered)
				}
			}()
			client, err := NewClient("user@example.com", "test-password")
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("NewClient() error = %v, want error containing %q", err, tt.wantError)
			}
			if client != nil {
				t.Error("NewClient() returned a client after failed login")
			}
		})
	}
}

func TestNewClientReturnsSubmitTransportError(t *testing.T) {
	managementServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, loginFormHTML)
	}))
	transport := http.DefaultTransport
	wantError := errors.New("connection failed during login")
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPost {
			return nil, wantError
		}
		return transport.RoundTrip(r)
	})

	client, err := NewClient("user@example.com", "test-password")
	if !errors.Is(err, wantError) {
		t.Errorf("NewClient() error = %v, want %v", err, wantError)
	}
	if client != nil {
		t.Error("NewClient() returned a client after a transport failure")
	}
}

func TestNewClientAcceptsEmptyAddressList(t *testing.T) {
	managementServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, loginFormHTML)
			return
		}
		fmt.Fprint(w, `<main id="content"><ul class="ox-list"></ul></main>`)
	}))

	client, err := NewClient("user@example.com", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	if got := client.List(); len(got) != 0 {
		t.Errorf("List() = %#v, want empty list", got)
	}
}

// Keep production URLs and cookie handling intact while routing all requests locally.
func managementServer(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "manage.mailbox.org" {
			return nil, fmt.Errorf("unexpected external request: %s", r.URL)
		}
		request := r.Clone(r.Context())
		request.URL.Scheme = target.Scheme
		request.URL.Host = target.Host
		response, err := transport.RoundTrip(request)
		if response != nil {
			response.Request = r
		}
		return response, err
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
