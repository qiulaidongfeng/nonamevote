package nonamevote

import (
	"bytes"
	"crypto/rsa"
	"path/filepath"

	"gitee.com/qiulaidongfeng/cachefs"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"github.com/gin-gonic/gin"
)

var prefix = func() string {
	r := "."
	if config.Test {
		r = "../"
	}
	return r
}()

var html = filepath.Join(prefix+string(filepath.Separator), "html")

var register = filepath.Join(html, "register.html")
var index = filepath.Join(html, "index.html")
var login = filepath.Join(html, "login.html")
var createvote = filepath.Join(html, "createvote.html")

var imgIndex = bytes.LastIndex(cacheFile("register.html"), []byte("<img>"))
var imgIndex2 = bytes.LastIndex(cacheFile("showQRCode.html"), []byte("<img>"))

var (
	cert []byte
	key  []byte
	S    = gin.Default()
)

var (
	pubkey  *rsa.PublicKey
	privkey *rsa.PrivateKey
)

var hfs = cachefs.NewHttpCacheFs(html)
