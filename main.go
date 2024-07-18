package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"image/png"
	"nonamevote/internal/account"
	"nonamevote/internal/vote"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

var html = filepath.Join("."+string(filepath.Separator), "html")

var register = filepath.Join(html, "register.html")
var index = filepath.Join(html, "index.html")
var login = filepath.Join(html, "login.html")
var createvote = filepath.Join(html, "createvote.html")

var (
	cert []byte
	key  []byte
	s    = gin.Default()
)

func main() {
	err := s.RunTLS(":560", "./cert.pem", "./key.pem")
	if err != nil {
		panic(err)
	}
}

func genTotpImg(user account.User) []byte {
	key, err := otp.NewKeyFromURL(user.TotpURL)
	if err != nil {
		panic(err)
	}
	img, err := key.Image(800, 800)
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

func init() {
	Init()
	// 创建投票网页
	for _, v := range vote.Db.Data {
		vote.AddVoteHtml(v)
	}
}

func Init() {
	initHttps()
	initRSA()

	vote.S = s
	account.Privkey = privkey

	s.UseH2C = true
	s.GET("/", func(ctx *gin.Context) {
		ctx.File(index)
	})
	s.GET("/register", func(ctx *gin.Context) {
		ctx.File(register)
	})
	s.POST("/register", func(ctx *gin.Context) {
		name := ctx.PostForm("name")
		if name == "" {
			ctx.String(401, "注册失败，因为没有提供用户名")
			return
		}
		user, err := account.NewUser(name)
		if err != nil {
			ctx.String(401, err.Error())
			return
		}
		buf := genTotpImg(user)
		ctx.Writer.Write(buf)
	})
	s.GET("/login", func(ctx *gin.Context) {
		//先考虑是否已经登录
		ok, err := account.CheckLogined(ctx)
		if ok {
			ctx.String(200, "登录成功")
			return
		}
		if err != nil {
			ctx.String(401, "登录失败：%s", err.Error())
			return
		}
		ctx.File(login)
	})
	s.POST("/login", func(ctx *gin.Context) {
		//先考虑是否已经登录
		ok, err := account.CheckLogined(ctx)
		if ok {
			ctx.String(200, "登录成功")
			return
		}

		name := ctx.PostForm("name")
		if name == "" {
			ctx.String(401, "登录失败，因为没有提供用户名")
			return
		}
		code := ctx.PostForm("totp")
		if len(code) != 6 {
			ctx.String(401, "登录失败，因为totp验证码必须是6位数")
			return
		}
		user := account.GetUser(name)
		if user.Name == "" {
			ctx.String(401, "登录失败，因为没有这个用户")
			return
		}
		key, err := otp.NewKeyFromURL(user.TotpURL)
		if err != nil {
			panic(err)
		}
		if !totp.Validate(code, key.Secret()) {
			ctx.String(401, "登录失败：totp验证码不对")
			return
		}
		se := account.NewSession(ctx, name)
		account.SessionDb.Add(se)
		account.SessionDb.SaveToOS()
		cookie := se.EnCode()
		wc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubkey, unsafe.Slice(unsafe.StringData(cookie), len(cookie)), nil)
		if err != nil {
			panic(err)
		}
		ctx.SetCookie("session", unsafe.String(unsafe.SliceData(wc), len(wc)), account.SessionMaxAge, "", "", true, false)
		user.Session[user.SessionIndex%3] = md5.Sum(unsafe.Slice(unsafe.StringData(se.Value), len(se.Value)))
		user.SessionIndex++
		account.ReplaceUser(user)
		ctx.String(200, "登录成功")
	})
	s.GET("/createvote", func(ctx *gin.Context) {
		ctx.File(createvote)
	})
	s.POST("/createvote", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.String(401, "登录失败：%s", err.Error())
				return
			}
			ctx.String(401, "已登录用户才能创建投票")
			return
		}
		_, err = vote.ParserCreateVote(ctx)
		if err != nil {
			ctx.String(401, "创建投票失败：%s", err.Error())
			return
		}
		ctx.String(200, "创建投票成功")
	})
}

func initHttps() {
	var err error
	cert, err = os.ReadFile("./cert.pem")
	if err != nil {
		if os.IsNotExist(err) {
			GenSSL()
			initHttps()
			return
		}
		panic(err)
	}
	key, err = os.ReadFile("./key.pem")
	if err != nil {
		panic(err)
	}
}
