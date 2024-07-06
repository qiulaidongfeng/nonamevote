package account

import (
	"crypto/rand"
	"fmt"
	"nonamevote/internal/data"

	"github.com/pquerna/otp/totp"
)

type User struct {
	Name         string
	TotpURL      string
	SessionIndex int8
	Session      [3][16]byte
}

func NewUser(Name string) (User, error) {
	_, ok := username[Name]
	if ok {
		return User{}, fmt.Errorf("用户名 %s 已被注册", Name)
	}
	username[Name] = struct{}{}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "无记名投票",
		AccountName: Name,
		SecretSize:  192,
		Rand:        rand.Reader,
	})
	if err != nil {
		panic(err)
	}
	return User{Name: Name, TotpURL: key.URL()}, nil
}

var UserDb = data.NewTable[User]("./user")

var username = make(map[string]struct{})

func init() {
	UserDb.LoadToOS()
	for i := range UserDb.Data {
		//记录用户不会允许重名，所以这里不要检查重名
		username[UserDb.Data[i].Name] = struct{}{}
	}
}

func GetUser(Name string) User {
	for i := range UserDb.Data {
		if UserDb.Data[i].Name == Name {
			return UserDb.Data[i]
		}
	}
	return User{}
}

func ReplaceUser(v User) {
	for i := range UserDb.Data {
		if UserDb.Data[i].Name == v.Name {
			UserDb.Data[i] = v
		}
	}
	UserDb.SaveToOS()
}
