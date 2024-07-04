package main

import (
	"bytes"
	"image/png"
	"nonamevote/internal/account"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
)

var html = filepath.Join("."+string(filepath.Separator), "html")

var register = filepath.Join(html, "register.html")
var index = filepath.Join(html, "index.html")
var pleasesavetotp = filepath.Join(html, "pleasesavetotp.html")

func main() {
	s := gin.Default()
	s.GET("/", func(ctx *gin.Context) {
		ctx.File(index)
	})
	s.GET("/register", func(ctx *gin.Context) {
		ctx.File(register)
	})
	s.POST("/register", func(ctx *gin.Context) {
		name := ctx.PostForm("name")
		if name == "" {
			ctx.String(200, "注册失败，因为没有提供用户名")
			return
		}
		user := account.NewUser(name)
		account.Db.Add(user)
		account.Db.SaveToOS()
		key, err := otp.NewKeyFromURL(user.TotpURL)
		if err != nil {
			panic(err)
		}
		img, err := key.Image(600, 600)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		err = png.Encode(&buf, img)
		if err != nil {
			panic(err)
		}
		ctx.Writer.Write(buf.Bytes())
	})
	s.Run(":560")
}
