package nonamevote

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"gitee.com/qiulaidongfeng/nonamevote/internal/rss"
	"gitee.com/qiulaidongfeng/nonamevote/internal/vote"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func Handle(s *gin.Engine) {
	vote.S = S
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
	vote.Init()
	s.GET("/", func(ctx *gin.Context) {
		ctx.Data(200, "text/html", cacheFile("index.html"))
	})
	s.GET("/register", func(ctx *gin.Context) {
		ctx.Data(200, "text/html", cacheFile("register.html"))
	})
	s.POST("/register", func(ctx *gin.Context) {
		name := ctx.PostForm("name")
		if name == "" {
			ctx.String(400, "注册失败，因为没有提供用户名")
			return
		}
		user, err := account.NewUser(name)
		if err != nil {
			ctx.String(409, err.Error())
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
		user := account.UserDb.Find(name)
		if user == nil {
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
			ctx.String(400, "创建投票失败：%s", err.Error())
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
		user := account.UserDb.Find(se.Name)
		if user == nil {
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
	s.GET("/search", func(ctx *gin.Context) {
		ctx.Data(200, "text/html", cacheFile("search.html"))
	})
	s.POST("/search", func(ctx *gin.Context) {
		s := ctx.PostForm("search")
		v := vote.NameDb.Find(s)
		if v == nil {
			ctx.String(404, "查询的投票不存在")
			return
		}
		ret := `
			<!DOCTYPE html>
				<head>
					<meta charset="UTF-8">
				</head>
				<body>
				</body>
				<script>
					function f() {
						window.location.href = "%s";
					}
					f();
    			</script>
			</html>
			`
		//TODO:支持查询有同名的投票
		v.Lock.Lock()
		//Note:添加进数据库的，v.Path[0]肯定有值
		path := v.Path[0]
		v.Lock.Unlock()
		ret = fmt.Sprintf(ret, strings.Join([]string{"https://", ctx.Request.Host, path}, ""))
		ctx.Data(200, "text/html", unsafe.Slice(unsafe.StringData(ret), len(ret)))
	})
	s.GET("/robots.txt", func(ctx *gin.Context) {
		ctx.String(200, rebots)
	})
}

const rebots = `
User-agent: *
Disallow: /exit
Disallow: /showQRCode
`
