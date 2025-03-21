// Package accunt 提供用户信息定义和创建，登录会话
package account

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"gitee.com/qiulaidongfeng/nonamevote/internal/safe"
	"github.com/pquerna/otp/totp"
)

type User struct {
	Name         string `gorm:"primaryKey"`
	TotpURL      string
	SessionIndex uint8 `gorm:"column:sessionindex"`
	Session      allSession
	VotedPath    data.All[string] `gorm:"column:votedpath"`
}

type allSession [3][16]byte

func (a *allSession) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	err := json.Unmarshal(bytes, a)
	return err
}

// Value return json value, implement driver.Valuer interface
func (a allSession) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func NewUser(Name string) (*User, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "无记名投票",
		AccountName: Name,
		SecretSize:  64,
		Rand:        rand.Reader,
	})
	if err != nil {
		panic(err)
	}
	url := key.URL()
	user := User{Name: Name, TotpURL: safe.Encrypt(key.URL())}
	if !UserDb.AddKV(Name, &user) {
		return nil, "", fmt.Errorf("用户名 %s 已被注册", Name)
	}
	return &user, url, nil
}

var UserDb = data.NewDb(data.User, func(u *User) string { return u.Name })
var LoginNumDb = data.NewDb[*int64](data.LoginNum, nil)
