package nonamevote

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"gitee.com/qiulaidongfeng/nonamevote/internal/safe"
	"gitee.com/qiulaidongfeng/nonamevote/internal/vote"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/qiulaidongfeng/safesession"
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

	benchmark(b, req, nil, false)
}

func BenchmarkIndex(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	benchmark(b, req, nil, false)
}

func BenchmarkIndex2(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	benchmark(b, req, nil, true)
}

func BenchmarkRegister(b *testing.B) {
	req := httptest.NewRequest("POST", "/register", nil)

	benchmark(b, req, func(req *http.Request) {
		req.PostForm = make(url.Values)
		v := &req.PostForm
		v.Set("name", randStr())
	}, false)
}

func BenchmarkSearch(b *testing.B) {
	req := httptest.NewRequest("POST", "/search", nil)

	origin1 := vote.Db
	origin2 := vote.NameDb
	defer func() { vote.Db = origin1; vote.NameDb = origin2 }()
	vote.Db = data.NewDb[*vote.Info](data.Vote, nil)
	vote.Db.AddKV("/vote/1", &vote.Info{End: time.Now()})
	vote.NameDb = data.NewDb(data.VoteName, func(n *vote.NameAndPath) string { return n.Name })
	vote.NameDb.AddKV("1", &vote.NameAndPath{Name: "1", Path: []string{"/vote/1"}})

	benchmark(b, req, func(req *http.Request) {
		req.PostForm = make(url.Values)
		v := &req.PostForm
		v.Set("search", "1")
	}, false)
}

func BenchmarkAllVote(b *testing.B) {
	req := httptest.NewRequest("GET", "/allvote", nil)

	origin1 := vote.Db
	origin2 := vote.NameDb
	defer func() { vote.Db = origin1; vote.NameDb = origin2 }()
	vote.Db = data.NewOsDb[*vote.Info]("", nil)
	vote.NameDb = data.NewOsDb("", func(n *vote.NameAndPath) string { return n.Name })
	for i := range 4 {
		vote.Db.AddKV("/vote/"+strconv.Itoa(i), &vote.Info{})
		vote.NameDb.AddKV(strconv.Itoa(i), &vote.NameAndPath{Name: strconv.Itoa(i), Path: []string{"/vote/" + strconv.Itoa(i)}})
	}
	benchmark(b, req, nil, false)
}

func BenchmarkGetVote(b *testing.B) {
	req := httptest.NewRequest("GET", "/vote/1", nil)

	origin := vote.Db
	defer func() { vote.Db = origin }()
	vote.Db = data.NewOsDb[*vote.Info]("", nil)
	vote.Db.AddKV("/vote/1", &vote.Info{Name: "n", End: time.Now(), Introduce: "i",
		Path: "/vote/1", Option: data.All[vote.Option]{vote.Option{Name: "0", GotNum: 1}, vote.Option{Name: "1", GotNum: 2}},
		Comment: data.All[string]{"1", "2"}})
	benchmark(b, req, nil, false)
}

func BenchmarkComment(b *testing.B) {
	cookie := &http.Cookie{
		Name:  "session",
		Value: cv,
	}
	req := httptest.NewRequest("POST", "/vote/k?comment=true", nil)
	req.PostForm = make(url.Values)
	v := &req.PostForm
	v.Set("commentValue", "1")
	req.AddCookie(cookie)

	benchmark(b, req, nil, false)
}

func benchmark(b *testing.B, req *http.Request, f func(*http.Request), test304 bool) {
	defer func(start time.Time) {
		total := time.Since(start).Seconds()
		b.ReportMetric(float64(b.N)/total, "reqs/s")
	}(time.Now())

	var h http.Header
	want := 200
	if test304 {
		w := httptest.NewRecorder()
		req := req.Clone(req.Context())
		if f != nil {
			f(req)
		}
		S.Handler().ServeHTTP(w, req)
		h = w.Header()
		want = 304
	}

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			w := httptest.NewRecorder()
			req := req.Clone(req.Context())
			if h != nil {
				req.Header = h.Clone()
				req.Header.Set("If-Modified-Since", h.Get("Last-Modified"))
			}
			if f != nil {
				f(req)
			}
			S.Handler().ServeHTTP(w, req)
			if w.Code != want {
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

func test_init() {
	safesession.Test = true
	data.Test = true
	config.Test = true
	for _, v := range []any{account.UserDb, account.SessionDb, vote.Db, vote.NameDb, account.LoginNumDb} {
		v.(interface{ Load() }).Load()
	}
	for _, v := range []any{account.UserDb, account.SessionDb, vote.Db, vote.NameDb, account.LoginNumDb} {
		have := false
		f := reflect.ValueOf(v).MethodByName("Data")
		yield := reflect.MakeFunc(f.Type().In(0), func(args []reflect.Value) (results []reflect.Value) {
			s := args[0].Interface().(string)
			if s != "" {
				have = true
				return []reflect.Value{reflect.ValueOf(false)}
			}
			return []reflect.Value{reflect.ValueOf(true)}
		})
		f.Call([]reflect.Value{yield})
		if have {
			fmt.Println("测试用的数据库应该是空的")
			os.Exit(2)
		}
	}
	k, _, err := account.NewUser("k")
	if err != nil {
		panic(err)
	}
	if account.UserDb.Find("k").Name != "k" {
		panic("test user generate fail")
	}
	old := slices.Clone(k.VotedPath)
	k.VotedPath = append(k.VotedPath, "/vote/k")
	if !account.UserDb.Updata("k", old, "VotedPath", k.VotedPath) {
		panic("update fail")
	}
	S = gin.New()
	Handle(S)
	cv = logink(nil)
	_, add := vote.Db.Add(&vote.Info{
		Path:      "/vote/k",
		Introduce: "",
		End:       time.Date(2100, time.April, 1, 1, 1, 1, 1, time.Local),
		Option:    []vote.Option{{Name: "0"}},
	})
	add()
	vote.NameDb.AddKV("k", &vote.NameAndPath{Name: "k", Path: []string{"/vote/k"}})
	gin.SetMode(gin.ReleaseMode)
}

func logink(t testing.TB) string {
	u := account.UserDb.Find("k")
	k, _ := otp.NewKeyFromURL(safe.Decrypt(u.TotpURL))
	code, _ := totp.GenerateCodeCustom(k.Secret(), time.Now(), totp.ValidateOpts{})

	req := httptest.NewRequest("POST", "/login", nil)
	req.PostForm = url.Values{
		"name": {"k"},
		"totp": {code},
	}

	w := httptest.NewRecorder()
	S.Handler().ServeHTTP(w, req)

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
	return base64.StdEncoding.EncodeToString(b)
}
