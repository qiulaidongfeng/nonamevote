package nonamevote

import (
	"bytes"
	"crypto/md5"
	"image/png"
	"io"
	"net/http"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
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

	old1 := user.Session
	old2 := user.SessionIndex

	user.Session[user.SessionIndex%3] = md5.Sum(unsafe.Slice(unsafe.StringData(se.Value), len(se.Value)))
	user.SessionIndex++

	//Note:这里不需要重试，如果有用户在极短时间重复登录，不是正常行为，是恶意攻击者有的行为
	account.UserDb.Updata(user.Name, old1, "Session", user.Session)
	//TODO:优化使用HIncrBy
	account.UserDb.Updata(user.Name, old2, "SessionIndex", user.SessionIndex)
}

func Close() {
	account.SessionDb.Save()
	account.UserDb.Save()
	account.LoginNumDb.Save()
	vote.Db.Save()
	vote.NameDb.Save()
}
