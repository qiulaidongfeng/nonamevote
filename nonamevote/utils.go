package nonamevote

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"image/png"
	"io"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
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

func genTotpImg(user *account.User) []byte {
	key, err := otp.NewKeyFromURL(user.TotpURL)
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
	wc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubkey, unsafe.Slice(unsafe.StringData(cookie), len(cookie)), nil)
	if err != nil {
		panic(err)
	}
	ctx.SetCookie("session", unsafe.String(unsafe.SliceData(wc), len(wc)), account.SessionMaxAge, "", "", true, true)

	old1 := user.Session
	old2 := user.SessionIndex

	user.Session[user.SessionIndex%3] = md5.Sum(unsafe.Slice(unsafe.StringData(se.Value), len(se.Value)))
	user.SessionIndex++

	//Note:这里不需要重试，如果有用户在极短时间重复登录，不是正常行为，是恶意攻击者有的行为
	account.UserDb.Updata(user.Name, old1, "Session", user.Session)
	//TODO:优化使用HIncrBy
	account.UserDb.Updata(user.Name, old2, "SessionIndex", user.SessionIndex)
}
