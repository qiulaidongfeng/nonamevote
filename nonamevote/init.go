package nonamevote

func init() {
	initHttps()
	if imgIndex == -1 {
		panic("应该有img在注册页")
	}
	if imgIndex2 == -1 {
		panic("应该有img在显示totp页")
	}

	S.UseH2C = true
	Handle(S)
}
