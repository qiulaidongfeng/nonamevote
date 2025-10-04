package nonamevote

import (
	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/nonamevote/internal/config"
	"github.com/qiulaidongfeng/nonamevote/internal/data"
)

func init() {
	initHttps()
	checkKey()
	if imgIndex == -1 {
		panic("应该有img在注册页")
	}
	if imgIndex2 == -1 {
		panic("应该有img在显示totp页")
	}

	S.UseH2C = true
	S.Use(func(ctx *gin.Context) {
		ip := ctx.RemoteIP()
		count := data.IpCount.AddIpCount(ip)
		maxcount := config.GetMaxCount()
		if count > maxcount {
			ctx.String(429, config.GetIpLimitInfo())
			ctx.Abort()
		}
		ctx.Next()
	})
	Handle(S)
}
