//go:build !race

package nonamevote

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func BenchmarkLogin(b *testing.B) {
	req := httptest.NewRequest("POST", "/login", nil)

	benchmark(b, req, func(req *http.Request) {
		u := account.UserDb.Find("k")
		k, _ := otp.NewKeyFromURL(u.TotpURL)
		code, _ := totp.GenerateCodeCustom(k.Secret(), time.Now(), totp.ValidateOpts{})

		req.PostForm = url.Values{
			"name": {"k"},
			"totp": {code},
		}
	})
}
