package nonamevote

import (
	"bytes"
	"path/filepath"

	"gitee.com/qiulaidongfeng/cachefs"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/utils"
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

var hfs = cachefs.NewHttpCacheFs(html)

const rebots = `
User-agent: *
Disallow: /exit
Disallow: /showQRCode
`

var login_ok = utils.GenTipText("登录成功，您可以返回首页创建并进行投票", "/", "返回首页")
var login_ed = utils.GenTipText("已经登录", "/", "返回首页")
var register_fail = utils.GenTipText("注册失败，因为没有提供用户名", "/register", "返回注册页")
var login_fail_noname = utils.GenTipText("登录失败，因为没有提供用户名", "/login", "返回登录页")
var login_fail_totpnum = utils.GenTipText("登录失败，因为totp验证码必须是6位数", "/login", "返回登录页")
var login_fail_nouser = utils.GenTipText("登录失败，因为没有这个用户", "/login", "返回登录页")
var login_fail_totperr = utils.GenTipText(`登录失败：totp验证码不对
请排查以下原因
1. 输入时输错了验证码
2. 所有的设备时间不一致`, "/login", "返回登录页")
var createvote_fail = utils.GenTipText("创建投票失败：已登录用户才能创建投票", "/login", "前往登录页")
var createvote_ok = utils.GenTipText("创建投票成功", "/", "返回首页")
var exit_fail = utils.GenTipText("退出登录失败：您未登录或登录已失效", "/", "返回首页")
var exit_ok = utils.GenTipText("退出登录成功", "/", "返回首页")
var show_fail_nologin = utils.GenTipText("您未登录", "/login", "前往登录页")
var search_fail = utils.GenTipText("搜索失败：搜索的投票不存在", "/search", "返回")
