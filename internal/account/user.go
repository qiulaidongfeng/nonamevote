package account

import (
	"crypto/rand"
	"fmt"
	"nonamevote/internal/data"

	"github.com/pquerna/otp/totp"
)

type User struct {
	Name    string
	TotpURL string
}

func NewUser(Name string) (User, error) {
	_, ok := username[Name]
	if ok {
		return User{}, fmt.Errorf("用户名 %s 已被注册", Name)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "无记名投票",
		AccountName: Name,
		SecretSize:  256,
		Rand:        rand.Reader,
	})
	if err != nil {
		panic(err)
	}
	return User{Name: Name, TotpURL: key.URL()}, nil
}

var Db = data.NewTable[User]("./user")

var username map[string]struct{}

func init() {
	Db.LoadToOS()
	for i := range Db.Data {
		//记录用户不会允许重名，所以这里不要检查重名
		username[Db.Data[i].Name] = struct{}{}
	}
}
