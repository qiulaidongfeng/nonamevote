//go:build race

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

const n = 1000

func TestRace(t *testing.T) {
	var wg sync.WaitGroup
	sendRequest(t, &wg, "GET", "/", nil)
	sendRequest(t, &wg, "GET", "/register", nil)
	sendRequest(t, &wg, "POST", "/register", func(req *http.Request, v *url.Values) { v.Set("name", "") })
	sendRequest(t, &wg, "POST", "/register", func(req *http.Request, v *url.Values) { v.Set("name", randStr()) })
	sendRequest(t, &wg, "POST", "/createvote", func(req *http.Request, v *url.Values) { v.Set("name", randStr()) })
	cv := logink(t)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cookie := &http.Cookie{
				Name:  "session",
				Value: cv,
			}

			req := httptest.NewRequest("POST", "/login", nil)
			req.PostForm = make(url.Values)
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
		}()
	}
	wg.Wait()
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
	})
	wg.Wait()
}

func sendRequest(t *testing.T, wg *sync.WaitGroup, method string, path string, form func(*http.Request, *url.Values)) {
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
		}()
	}
}
