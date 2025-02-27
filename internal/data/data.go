package data

import "gitee.com/qiulaidongfeng/nonamevote/internal/config"

type Db[T any] interface {
	Load()
	Save()
	Add(v T) (int, func())
	AddKV(key string, v T)
	Data(yield func(string, T) bool)
	Find(k string) T
	Delete(k string)
	AddIpCount(ip string) (r int64)
	Changed()
	Updata(key string, old any, field string, v any) (ok bool)
	IncOption(key string, i int)
}

var _ Db[any] = (*OsDb[any])(nil)
var _ Db[any] = (*RedisDb[any])(nil)

const (
	Ip = iota
	User
	Session
	Vote
	VoteName
)

func NewDb[T any](typ int, key func(T) string) Db[T] {
	host, path := config.GetRedis()
	os := config.GetDbMode() == "os"
	file := ""
	db := 0
	switch typ {
	case Ip:
	case User:
		file = "./user"
		db = 1
	case Session:
		file = "./session"
		db = 2
	case Vote:
		file = "./vote"
		db = 3
	case VoteName:
		file = "./votename"
		db = 4
	}
	if os {
		r := NewOsDb[T](file, key)
		if typ == Ip {
			r.ipDb = true
		}
		return r
	}
	r := NewRedisDb[T](host, path, db, key)
	if typ == Ip {
		r.ipDb = true
	}
	return r
}

var Test bool
