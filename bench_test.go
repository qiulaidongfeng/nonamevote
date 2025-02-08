package main

import (
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"nonamevote/internal/account"
	"nonamevote/internal/data"
	"nonamevote/internal/vote"
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

	benchmark(b, req, nil)
}

func BenchmarkIndex(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	benchmark(b, req, nil)
}

func BenchmarkRegister(b *testing.B) {
	req := httptest.NewRequest("POST", "/register", nil)

	benchmark(b, req, func(req *http.Request) {
		req.PostForm = make(url.Values)
		v := &req.PostForm
		v.Set("name", randStr())
	})
}

func BenchmarkSearch(b *testing.B) {
	req := httptest.NewRequest("POST", "/search", nil)

	origin1 := vote.Db
	origin2 := vote.NameDb
	defer func() { vote.Db = origin1; vote.NameDb = origin2 }()
	vote.Db = data.NewMapTable[*vote.Info]("", nil)
	vote.Db.AddKV("/vote/1", &vote.Info{})
	vote.NameDb = data.NewMapTable[*vote.NameAndPath]("", nil)
	vote.NameDb.AddKV("1", &vote.NameAndPath{Path: []string{"/vote/1"}})

	benchmark(b, req, func(req *http.Request) {
		req.PostForm = make(url.Values)
		v := &req.PostForm
		v.Set("search", "1")
	})
}

func benchmark(b *testing.B, req *http.Request, f func(*http.Request)) {
	defer func(start time.Time) {
		total := time.Since(start).Seconds()
		b.ReportMetric(float64(b.N)/total, "reqs/s")
	}(time.Now())

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			w := httptest.NewRecorder()
			req := req.Clone(req.Context())
			if f != nil {
				f(req)
			}
			s.Handler().ServeHTTP(w, req)
			if w.Code != 200 {
				b.Fail()
				bs, err := io.ReadAll(w.Body)
				if err != nil {
					panic(err)
				}
				b.Log(string(bs))
			}
		}
	})
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

	req := httptest.NewRequest("POST", "/login", nil)
	req.PostForm = url.Values{
		"name": {"k"},
		"totp": {code},
	}

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
