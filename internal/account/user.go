package account

import (
	"crypto/rand"
	"fmt"
	"nonamevote/internal/data"
	"sync"

	"github.com/pquerna/otp/totp"
)

type User struct {
	Name         string
	TotpURL      string
	SessionIndex uint8
	Session      [3][16]byte
}

func NewUser(Name string) (User, error) {
	usernameLock.RLock()
	_, ok := username[Name]
	usernameLock.RUnlock()
	if ok {
		return User{}, fmt.Errorf("用户名 %s 已被注册", Name)
	}
	usernameLock.Lock()
	username[Name] = struct{}{}
	usernameLock.Unlock()
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "无记名投票",
		AccountName: Name,
		SecretSize:  192,
		Rand:        rand.Reader,
	})
	if err != nil {
		panic(err)
	}
	user := User{Name: Name, TotpURL: key.URL()}
	UserDb.Add(user)
	UserDb.SaveToOS()
	return user, nil
}

var UserDb = data.NewTable[User]("./user")

var username = make(map[string]struct{})

var usernameLock sync.RWMutex

func init() {
	UserDb.LoadToOS()
	for _, v := range UserDb.Data {
		//记录用户不会允许重名，所以这里不要检查重名
		username[v.Name] = struct{}{}
	}
}

func GetUser(Name string) User {
	for _, v := range UserDb.Data {
		if v.Name == Name {
			return v
		}
	}
	return User{}
}

func ReplaceUser(v User) {
	UserDb.Replace(v, func(u User) bool {
		return u.Name == v.Name
	})
	UserDb.SaveToOS()
}
