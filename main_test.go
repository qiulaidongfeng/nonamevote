package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"nonamevote/internal/account"
	"nonamevote/internal/data"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestRace(t *testing.T) {
	var wg sync.WaitGroup
	sendRequest(t, &wg, "GET", "/", nil)
	sendRequest(t, &wg, "GET", "/register", nil)
	sendRequest(t, &wg, "POST", "/register", func(v *url.Values) { v.Set("name", "") })
	sendRequest(t, &wg, "POST", "/register", func(v *url.Values) { v.Set("name", randStr()) })
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			u := account.GetUser("k")
			k, _ := otp.NewKeyFromURL(u.TotpURL)
			code, _ := totp.GenerateCodeCustom(k.Secret(), time.Now(), totp.ValidateOpts{})
			data := url.Values{
				"name": {"k"},
				"totp": {code},
			}

			// 将表单数据编码为字符串
			dataString := data.Encode()
			body := bytes.NewBufferString(dataString)
			req := httptest.NewRequest("POST", "https://localhost:560/login", body)

			// 设置必要的HTTP头部
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Content-Length", fmt.Sprintf("%d", body.Len()))

			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			resp := w.Result()

			cookies := resp.Cookies()
			if len(cookies) == 0 {
				t.Logf("%+v\n", resp)
				t.Log(io.ReadAll(req.Body))
				panic("no session")
			}
			var cv string
			for _, v := range cookies {
				if v.Name == "session" {
					cv = v.Value
				}
			}

			req = httptest.NewRequest("GET", "/login", nil)
			req.PostForm = make(url.Values)
			req.PostForm.Set("session", cv)
			w = httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
		}()
	}
	wg.Wait()
}

func randStr() string {
	b := make([]byte, 16)
	rand.Reader.Read(b)
	return string(b)
}

func init() {
	account.Test = true
	data.Test = true
	_, err := account.NewUser("k")
	if err != nil {
		panic(err)
	}
	if account.GetUser("k").Name != "k" {
		panic("test user generate fail")
	}
}

func sendRequest(t *testing.T, wg *sync.WaitGroup, method string, path string, form func(*url.Values)) {
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(method, path, nil)
			if form != nil {
				req.PostForm = make(url.Values)
				form(&req.PostForm)
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
		}()
	}
}
