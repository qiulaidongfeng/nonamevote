package nonamevote

import (
	"bytes"
	"crypto/md5"
	"errors"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"gitee.com/qiulaidongfeng/nonamevote/internal/safe"
	"gitee.com/qiulaidongfeng/nonamevote/internal/vote"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
)

func cacheFile(file string) []byte {
	f, err := hfs.Open(file)
	if err != nil {
		panic(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return b
}

func handleFile(ctx *gin.Context, name string) {
	f, err := hfs.Open(name)
	if err != nil {
		panic(err)
	}
	i, err := f.Stat()
	if err != nil {
		panic(err)
	}
	http.ServeContent(ctx.Writer, ctx.Request, name, i.ModTime(), f)
}

func genTotpImg(url string) []byte {
	key, err := otp.NewKeyFromURL(url)
	if err != nil {
		panic(err)
	}
	img, err := key.Image(400, 400)
	if err != nil {
		panic(err)
	}
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func addSession(ctx *gin.Context, user *account.User) {
	se := account.NewSession(ctx, user.Name)
	//TODO:处理session value重复
	account.SessionDb.AddKV(se.Value, se)
	cookie := se.EnCode()
	wc := safe.Encrypt(cookie)
	ctx.SetCookie("session", wc, account.SessionMaxAge, "", "", true, true)

	old := user.Session
	user.Session[user.SessionIndex%3] = md5.Sum(unsafe.Slice(unsafe.StringData(se.Value), len(se.Value)))
	//Note:这里不需要重试，如果有用户在极短时间重复登录，不是正常行为，是恶意攻击者有的行为
	account.UserDb.UpdataSession(user.Name, user.SessionIndex%3, user.Session[user.SessionIndex%3], old, user.Session)
	user.SessionIndex++
	account.UserDb.IncField(user.Name, "SessionIndex")
}

func Close() {
	account.SessionDb.Save()
	account.UserDb.Save()
	account.LoginNumDb.Save()
	vote.Db.Save()
	vote.NameDb.Save()
}

func checkKey() {
	if config.GetDbMode() == "os" {
		v, err := os.ReadFile("./check")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				fd, err := os.OpenFile("./check", os.O_RDWR|os.O_CREATE, 0600)
				if err != nil {
					panic(err)
				}
				_, err = fd.WriteString(safe.Encrypt("test"))
				if err != nil {
					panic(err)
				}
				return
			}
			panic(err)
		}
		if safe.Decrypt(string(v)) != "test" {
			panic("两次启动使用了不同的主密钥")
		}
		return
	}
	v, set := data.IpCount.(interface {
		LoadOrStoreStr(key, value string) (string, bool)
	}).LoadOrStoreStr("key", safe.Encrypt("test"))
	if set {
		return
	}
	if safe.Decrypt(v) != "test" {
		panic("两次启动使用了不同的主密钥")
	}
}

func AddIpCount(ip string) int64 {
	return data.IpCount.AddIpCount(ip)
}

func GetMaxCount() int64 {
	return config.GetMaxCount()
}

func GetExpiration() int {
	return config.GetExpiration()
}
