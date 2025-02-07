package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"nonamevote/internal/account"
	"nonamevote/internal/data"
	"nonamevote/internal/vote"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

var cv string

func BenchmarkCreateVote(b *testing.B) {
	cookie := &http.Cookie{
		Name:  "session",
		Value: cv,
	}
	req := httptest.NewRequest("POST", "/createvote", nil)
	req.PostForm = make(url.Values)
	v := &req.PostForm
	v.Set("name", randStr())
	v.Set("date", "2029-1-1")
	v.Set("time", "11:00")
	v.Set("introduce", "l")
	v.Set("option", "k l")
	req.AddCookie(cookie)

	defer func(start time.Time) {
		total := time.Since(start).Seconds()
		b.ReportMetric(float64(b.N)/total, "reqs/s")
	}(time.Now())
	var wg sync.WaitGroup

	for range b.N {
		wg.Add(1)
		go func() {
			wg.Done()
			w := httptest.NewRecorder()
			req := req.Clone(req.Context())
			s.Handler().ServeHTTP(w, req)
			if w.Code != 200 {
				b.Fail()
				bs, err := io.ReadAll(w.Body)
				if err != nil {
					panic(err)
				}
				b.Log(string(bs))
			}
		}()

	}
	wg.Wait()
}

func init() {
	account.Test = true
	data.Test = true
	_, err := account.NewUser("k")
	if err != nil {
		panic(err)
	}
	if account.UserDb.Find("k").Name != "k" {
		panic("test user generate fail")
	}
	s = gin.New()
	Init()
	cv = logink(nil)
	_, add := vote.Db.Add(&vote.Info{
		Path:   "/vote/k",
		Option: []vote.Option{{Name: "0"}},
	})
	add()
	gin.SetMode(gin.ReleaseMode)
}

func logink(t testing.TB) string {
	u := account.UserDb.Find("k")
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
		body, _ := io.ReadAll(resp.Body)
		t.Logf("%+v\n", string(body))
		panic("no session")
	}
	var cv string
	for _, v := range cookies {
		if v.Name == "session" {
			cv = v.Value
		}
	}
	return cv
}

func randStr() string {
	b := make([]byte, 16)
	rand.Reader.Read(b)
	return string(b)
}
