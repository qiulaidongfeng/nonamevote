package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

const n = 500

func TestRace(t *testing.T) {
	var wg sync.WaitGroup
	sendRequest(t, &wg, "GET", "/", nil, 200)
	sendRequest(t, &wg, "GET", "/register", nil, 200)
	sendRequest(t, &wg, "POST", "/register", func(req *http.Request, v *url.Values) { v.Set("name", "") }, 400)
	sendRequest(t, &wg, "POST", "/register", func(req *http.Request, v *url.Values) { v.Set("name", randStr()) }, 200)
	sendRequest(t, &wg, "POST", "/createvote", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("name", randStr())
	}, 400)
	sendRequest(t, &wg, "GET", "/vote/k", nil, 200)
	sendRequest(t, &wg, "POST", "/login", func(req *http.Request, v *url.Values) {
		cookie := &http.Cookie{
			Name:  "session",
			Value: cv,
		}
		req.AddCookie(cookie)
	}, 200)
	sendRequest(t, &wg, "POST", "/createvote", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("name", randStr())
		v.Set("date", "2029-1-1")
		v.Set("time", "11:00")
		v.Set("introduce", "l")
		v.Set("option", "k l")
	}, 200)
	sendRequest(t, &wg, "GET", "/vote/k", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
	}, 200)
	sendRequest(t, &wg, "POST", "/vote/k", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("k", "0")
	}, 401)
	wg.Wait()
}

func sendRequest(t *testing.T, wg *sync.WaitGroup, method string, path string, form func(*http.Request, *url.Values), code int) {
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(method, path, nil)
			if form != nil {
				req.PostForm = make(url.Values)
				form(req, &req.PostForm)
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			if w.Code != code {
				t.Fail()
				b, _ := io.ReadAll(w.Body)
				t.Log(method, path, w.Code, string(b))
			}
		}()
	}
}
