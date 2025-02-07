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
	SessionIndex uint8
	Session      [3][16]byte
	VotedPath    []string
}

func NewUser(Name string) (*User, error) {
	ok := UserDb.Find(Name) != nil
	if ok {
		return nil, fmt.Errorf("用户名 %s 已被注册", Name)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "无记名投票",
		AccountName: Name,
		SecretSize:  64,
		Rand:        rand.Reader,
	})
	if err != nil {
		panic(err)
	}
	user := User{Name: Name, TotpURL: key.URL()}
	//TODO:处理用户名突然被注册
	UserDb.AddKV(Name, &user)
	return &user, nil
}

var UserDb = data.NewMapTable[*User]("./user", nil)

func init() {
	UserDb.LoadToOS()
}
