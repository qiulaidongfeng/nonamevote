package account

import (
	"crypto/rand"
	"nonamevote/internal/data"

	"github.com/pquerna/otp/totp"
)

type User struct {
	Name    string
	TotpURL string
}

func NewUser(Name string) User {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "无记名投票",
		AccountName: Name,
		SecretSize:  256,
		Rand:        rand.Reader,
	})
	if err != nil {
		panic(err)
	}
	return User{Name: Name, TotpURL: key.URL()}
}

var Db = data.NewTable[User]("./user")

func init() {
	Db.LoadToOS()
}
