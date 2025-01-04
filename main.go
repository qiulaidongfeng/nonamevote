package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"image/png"
	"io"
	"nonamevote/internal/account"
	"nonamevote/internal/config"
	"nonamevote/internal/data"
	"nonamevote/internal/rss"
	"nonamevote/internal/vote"
	"os"
	"path/filepath"
	_ "time/tzdata"
	"unsafe"

	"gitee.com/qiulaidongfeng/cachefs"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

var html = filepath.Join("."+string(filepath.Separator), "html")

var register = filepath.Join(html, "register.html")
var index = filepath.Join(html, "index.html")
var login = filepath.Join(html, "login.html")
var createvote = filepath.Join(html, "createvote.html")

var imgIndex = bytes.LastIndex(cacheFile("register.html"), []byte("<img>"))
var imgIndex2 = bytes.LastIndex(cacheFile("showQRCode.html"), []byte("<img>"))

var (
	cert []byte
	key  []byte
	s    = gin.Default()
)

func main() {
	err := s.RunTLS(":443", "./cert.pem", "./key.pem")
	if err != nil {
		panic(err)
	}
}

func genTotpImg(user account.User) []byte {
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

func init() {
	if imgIndex == -1 {
		panic("应该有img在注册页")
	}
	if imgIndex2 == -1 {
		panic("应该有img在显示totp页")
	}
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
	s.Use(func(ctx *gin.Context) {
		ip := ctx.RemoteIP()
		count := data.IpCount.AddIpCount(ip)
		expiration := config.GetExpiration()
		maxcount := config.GetMaxCount()
		if count > maxcount {
			ctx.String(403, "%d秒内这个ip(%s)访问网站超过%d次，请等%d秒后再访问网站", expiration, ip, maxcount, expiration)
			ctx.Abort()
		}
		ctx.Next()
	})
	s.GET("/", func(ctx *gin.Context) {
		ctx.Data(200, "text/html", cacheFile("index.html"))
	})
	s.GET("/register", func(ctx *gin.Context) {
		ctx.Data(200, "text/html", cacheFile("register.html"))
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
		//在注册时直接就登录
		addSession(ctx, user)
		buf := genTotpImg(user)
		data := cacheFile("register.html")
		ctx.Writer.WriteHeader(200)
		ctx.Writer.Write(data[:imgIndex])
		ctx.Writer.WriteString("<p>注册成功</p><img src=data:image/png;base64,")
		ctx.Writer.WriteString(base64.StdEncoding.EncodeToString(buf))
		ctx.Writer.WriteString(">")
		ctx.Writer.Write(data[imgIndex+5:])
	})
	s.GET("/login", func(ctx *gin.Context) {
		//先考虑是否已经登录
		ok, err, _ := account.CheckLogined(ctx)
		if ok {
			ctx.String(200, "登录成功")
			return
		}
		if err != nil {
			ctx.String(401, "登录失败：%s", err.Error())
			return
		}
		ctx.Data(200, "text/html", cacheFile("login.html"))
	})
	s.POST("/login", func(ctx *gin.Context) {
		//先考虑是否已经登录
		ok, err, _ := account.CheckLogined(ctx)
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
			ctx.String(401, `登录失败：totp验证码不对
请排查以下原因
1. 输入时输错了验证码
2. 所有的设备时间不一致`)
			return
		}
		addSession(ctx, user)
		ctx.String(200, "登录成功")
	})
	s.GET("/createvote", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, _ := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.String(401, "登录失败：%s", err.Error())
				return
			}
			ctx.String(401, "已登录用户才能创建投票")
			return
		}
		ctx.Data(200, "text/html", cacheFile("createvote.html"))
	})
	s.POST("/createvote", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, _ := account.CheckLogined(ctx)
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
	s.GET("/allvote", vote.AllVote)
	s.GET("/exit", func(ctx *gin.Context) {
		_, err := ctx.Cookie("session")
		if err != nil {
			ctx.String(401, "未登录")
			return
		}
		ctx.SetCookie("session", "", -1, "", "", true, true)
		ctx.String(200, "退出登录成功")
	})
	s.GET("/showQRCode", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, se := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.String(401, "登录失败：%s", err.Error())
				return
			}
			ctx.String(401, "您未登录")
			return
		}
		user := account.GetUser(se.Name)
		if user.Name == "" {
			ctx.String(401, "没有这个用户")
			return
		}
		buf := genTotpImg(user)
		data := cacheFile("showQRCode.html")
		ctx.Writer.WriteHeader(200)
		ctx.Writer.Write(data[:imgIndex2])
		ctx.Writer.WriteString("<img src=data:image/png;base64,")
		ctx.Writer.WriteString(base64.StdEncoding.EncodeToString(buf))
		ctx.Writer.WriteString(">")
		ctx.Writer.Write(data[imgIndex2+5:])
	})
	s.GET("/rss.xml", func(ctx *gin.Context) {
		ctx.Writer.WriteString(rss.Generate())
	})
}

func addSession(ctx *gin.Context, user account.User) {
	se := account.NewSession(ctx, user.Name)
	account.SessionDb.Add(se)
	cookie := se.EnCode()
	wc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubkey, unsafe.Slice(unsafe.StringData(cookie), len(cookie)), nil)
	if err != nil {
		panic(err)
	}
	ctx.SetCookie("session", unsafe.String(unsafe.SliceData(wc), len(wc)), account.SessionMaxAge, "", "", true, true)
	user.Session[user.SessionIndex%3] = md5.Sum(unsafe.Slice(unsafe.StringData(se.Value), len(se.Value)))
	user.SessionIndex++
	account.ReplaceUser(user)
}

var hfs = cachefs.NewHttpCacheFs(html)

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
