//go:build !race

package nonamevote

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/qiulaidongfeng/nonamevote/internal/account"
	"github.com/qiulaidongfeng/nonamevote/internal/config"
	"github.com/qiulaidongfeng/nonamevote/internal/safe"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func BenchmarkLogin(b *testing.B) {
	req := httptest.NewRequest("POST", "/login", nil)

	benchmark(b, req, func(req *http.Request) {
		u := account.UserDb.Find("k")
		k, _ := otp.NewKeyFromURL(safe.Decrypt(u.TotpURL))
		code, _ := totp.GenerateCodeCustom(k.Secret(), time.Now(), totp.ValidateOpts{})

		req.PostForm = url.Values{
			"name": {"k"},
			"totp": {code},
		}
	}, false)
}

func TestLogin(t *testing.T) {
	config.Test = false
	defer func() { config.Test = true }()
	req := httptest.NewRequest("POST", "/login", nil)

	_, u, _ := account.NewUser("testa")
	k, _ := otp.NewKeyFromURL(u)
	code, _ := totp.GenerateCodeCustom(k.Secret(), time.Now(), totp.ValidateOpts{})

	req.PostForm = url.Values{
		"name": {"testa"},
		"totp": {code},
	}
	for range 3 {
		w := httptest.NewRecorder()
		S.Handler().ServeHTTP(w, req)
	}
	w := httptest.NewRecorder()
	S.Handler().ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("got %d, want 401", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("30秒内只能尝试3次登录")) {
		t.Fatalf("got %s", string(w.Body.Bytes()))
	}
}
