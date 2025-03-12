package nonamevote

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"gitee.com/qiulaidongfeng/nonamevote/internal/vote"
)

const n = 500

func TestRace(t *testing.T) {
	var wg sync.WaitGroup
	sendRequest(t, &wg, "GET", "/", nil, 200, nil)
	sendRequest(t, &wg, "GET", "/register", nil, 200, nil)
	sendRequest(t, &wg, "POST", "/register", func(req *http.Request, v *url.Values) { v.Set("name", "") }, 400, func(s string) bool { return strings.Contains(s, "注册失败，因为没有提供用户名") })
	sendRequest(t, &wg, "POST", "/register", func(req *http.Request, v *url.Values) { v.Set("name", randStr()) }, 200, func(s string) bool { return strings.Contains(s, "注册成功") })
	sendRequest(t, &wg, "POST", "/createvote", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("name", randStr())
	}, 400, func(s string) bool {
		return strings.Contains(s, "创建投票失败") && strings.Contains(s, "投票介绍不能为空")
	})
	sendRequest(t, &wg, "GET", "/vote/k", nil, 200, nil)
	sendRequest(t, &wg, "POST", "/login", func(req *http.Request, v *url.Values) {
		cookie := &http.Cookie{
			Name:  "session",
			Value: cv,
		}
		req.AddCookie(cookie)
	}, 200, func(s string) bool { return strings.Contains(s, "已经登录") })
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
	}, 200, func(s string) bool { return strings.Contains(s, "创建投票成功") })
	sendRequest(t, &wg, "GET", "/vote/k", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
	}, 200, nil)
	sendRequest(t, &wg, "POST", "/vote/k", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("k", "0")
	}, 401, func(s string) bool { return strings.Contains(s, "投票失败：因为已经投过票了") })
	sendRequest(t, &wg, "GET", "/allvote", nil, 200, func(s string) bool { return strings.Contains(s, `<a href="/vote/k">`) })
	sendRequest(t, &wg, "POST", "/search", func(r *http.Request, v *url.Values) {
		v.Set("search", "k")
	}, 200, func(s string) bool { return strings.Contains(s, `/vote/k";`) })
	sendRequest(t, &wg, "POST", "/vote/k?comment=true", func(req *http.Request, v *url.Values) {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("commentValue", "0")
	}, 200, func(s string) bool { return strings.Contains(s, "/vote/k") })
	i := int64(0)
	sendRequest(t, &wg, "POST", "/vote/k", func(req *http.Request, v *url.Values) {
		//TODO:避免这里导致在redis模式，不能使用-count重复运行测试
		req1 := httptest.NewRequest("POST", "/register", nil)
		req1.PostForm = url.Values{
			"name": {"test" + strconv.Itoa(int(atomic.AddInt64(&i, 1)))},
		}

		w := httptest.NewRecorder()
		S.Handler().ServeHTTP(w, req1)

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

		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: cv,
		})
		v.Set("k", "0")
	}, 200, func(s string) bool { return strings.Contains(s, "投票成功") })
	wg.Wait()

	v := vote.Db.Find("/vote/k")
	if len(v.Comment) != n {
		t.Fail()
		t.Log(len(v.Comment))
		t.Log("数据竞争在增加评论时")
	}
	if v.Option[0].GotNum != n {
		t.Fail()
		t.Log(v.Option[0].GotNum)
		t.Log("数据竞争在增加得票数时")
	}
}

func sendRequest(t *testing.T, wg *sync.WaitGroup, method string, path string, form func(*http.Request, *url.Values), code int, check func(s string) bool) {
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
			S.Handler().ServeHTTP(w, req)
			b, _ := io.ReadAll(w.Body)
			s := unsafe.String(unsafe.SliceData(b), len(b))
			if w.Code != code || (check != nil && !check(s)) {
				t.Fail()
				t.Log(method, path, w.Code, s)
			}
		}()
	}
}

func TestMain(m *testing.M) {
	test_init()
	m.Run()
	data.IpCount.Clear()
	account.SessionDb.Clear()
	account.UserDb.Clear()
	vote.Db.Clear()
	vote.NameDb.Clear()
	account.LoginNumDb.Clear()
}
