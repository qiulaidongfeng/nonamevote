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
	IncOption(key string, i int, old any, v any) (ok bool)
	Clear()
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
	mysql := config.GetDbMode() == "mysql-redis"
	file := ""
	switch typ {
	case Ip:
	case User:
		file = "./user"
	case Session:
		file = "./session"
	case Vote:
		file = "./vote"
	case VoteName:
		file = "./votename"
	}
	if os {
		r := NewOsDb(file, key)
		r.ipDb = typ == Ip
		return r
	}
	if typ == Ip {
		return NewRedisDb(host, path, typ, key)
	}
	if mysql {
		user, password, addr := config.GetDsnInfo()
		return NewMysqlDb(user, password, addr, typ, key)
	}
	return NewRedisDb(host, path, typ, key)
}

var Test bool
