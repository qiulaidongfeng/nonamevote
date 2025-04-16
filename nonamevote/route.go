package nonamevote

import (
	"encoding/base64"
	"strings"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"gitee.com/qiulaidongfeng/nonamevote/internal/rss"
	"gitee.com/qiulaidongfeng/nonamevote/internal/safe"
	"gitee.com/qiulaidongfeng/nonamevote/internal/utils"
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
		handleFile(ctx, "index.html")
	})
	s.GET("/register", func(ctx *gin.Context) {
		handleFile(ctx, "register.html")
	})
	s.POST("/register", func(ctx *gin.Context) {
		name := ctx.PostForm("name")
		if name == "" {
			ctx.Data(400, "text/html", register_fail)
			return
		}
		user, url, err := account.NewUser(name)
		if err != nil {
			ctx.Data(409, "text/html", utils.GenTipText("注册失败："+err.Error(), "/register", "返回注册页"))
			return
		}
		//在注册时直接就登录
		addSession(ctx, user)
		buf := genTotpImg(url)
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
			ctx.Data(200, "text/html", login_ed)
			return
		}
		if err != nil {
			ctx.Data(401, "text/html", utils.GenTipText("登录失败："+err.Error(), "/login", "返回登录页"))
			return
		}
		handleFile(ctx, "login.html")
	})
	s.POST("/login", func(ctx *gin.Context) {
		//先考虑是否已经登录
		ok, err, _ := account.CheckLogined(ctx)
		if ok {
			ctx.Data(200, "text/html", login_ed)
			return
		}

		name := ctx.PostForm("name")
		if name == "" {
			ctx.Data(401, "text/html", login_fail_noname)
			return
		}
		code := ctx.PostForm("totp")
		if len(code) != 6 {
			ctx.Data(401, "text/html", login_fail_totpnum)
			return
		}
		user := account.UserDb.Find(name)
		if user == nil {
			ctx.Data(401, "text/html", login_fail_nouser)
			return
		}
		if account.LoginNumDb.AddLoginNum(user.Name) > 3 && !config.Test {
			ctx.Data(401, "text/html", login_fail_too_often)
			return
		}
		key, err := otp.NewKeyFromURL(safe.Decrypt(user.TotpURL))
		if err != nil {
			panic(err)
		}
		if !totp.Validate(code, key.Secret()) {
			ctx.Data(401, "text/html", login_fail_totperr)
			return
		}
		addSession(ctx, user)
		ctx.Data(200, "text/html", login_ok)
	})
	s.GET("/createvote", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, _ := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.Data(401, "text/html", utils.GenTipText("登录失败："+err.Error(), "/login", "前往登录页"))
				return
			}
			ctx.Data(401, "text/html", createvote_fail)
			return
		}
		handleFile(ctx, "createvote.html")
	})
	s.POST("/createvote", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, _ := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.Data(401, "text/html", utils.GenTipText("登录失败："+err.Error(), "/login", "前往登录页"))
				return
			}
			ctx.Data(401, "text/html", createvote_fail)
			return
		}
		v, err := vote.ParserCreateVote(ctx)
		if err != nil {
			ctx.Data(400, "text/html", utils.GenTipText("创建投票失败："+err.Error(), "/createvote", "返回创建投票页"))
			return
		}
		ret := redirect(v.Path)
		ctx.Data(200, "text/html", unsafe.Slice(unsafe.StringData(ret), len(ret)))
	})
	s.GET("/allvote", vote.AllVote)
	s.GET("/exit", func(ctx *gin.Context) {
		_, err := ctx.Cookie("session")
		if err != nil {
			ctx.Data(401, "text/html", exit_fail)
			return
		}
		ctx.SetCookie("session", "", -1, "", "", true, true)
		ctx.Data(200, "text/html", exit_ok)
	})
	s.GET("/showQRCode", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, se := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.Data(401, "text/html", utils.GenTipText("登录失败："+err.Error(), "/login", "前往登录页"))
				return
			}
			ctx.Data(401, "text/html", show_fail_nologin)
			return
		}
		user := account.UserDb.Find(se.Name)
		if user == nil {
			//Note:这里极不可能是正常用户的行为，所以返回简短的提示文字就行
			ctx.String(401, "没有这个用户")
			return
		}
		buf := genTotpImg(safe.Decrypt(user.TotpURL))
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
		handleFile(ctx, "search.html")
	})
	s.POST("/search", func(ctx *gin.Context) {
		s := ctx.PostForm("search")
		v := vote.NameDb.Find(s)
		if v == nil {
			ctx.Data(404, "text/html", search_fail)
			return
		}
		//TODO:支持查询有同名的投票
		v.Lock.Lock()
		//Note:添加进数据库的，v.Path[0]肯定有值
		path := v.Path[0]
		v.Lock.Unlock()
		ret := redirect(strings.Join([]string{"https://", ctx.Request.Host, path}, ""))
		ctx.Data(200, "text/html", unsafe.Slice(unsafe.StringData(ret), len(ret)))
	})
	s.GET("/robots.txt", func(ctx *gin.Context) {
		ctx.String(200, rebots)
	})
	s.GET("/deleteAccount", func(ctx *gin.Context) {
		//先检查是否已登录
		ok, err, se := account.CheckLogined(ctx)
		if !ok {
			if err != nil {
				ctx.Data(401, "text/html", utils.GenTipText("登录失败："+err.Error(), "/login", "前往登录页"))
				return
			}
			ctx.Data(401, "text/html", show_fail_nologin)
			return
		}
		account.UserDb.Delete(se.Name)
		ctx.Data(200, "text/html", delete_ok)
	})
}

func redirect(path string) string {
	var buf strings.Builder
	buf.WriteString(`<!DOCTYPE html>
	<head>
		<meta charset="UTF-8">
	</head>
	<body>
	</body>
	<script>
		function f() {
			window.location.href ="`)
	buf.WriteString(path)
	buf.WriteString(`";
		}
		f();
	</script>
</html>`)
	return buf.String()
}
