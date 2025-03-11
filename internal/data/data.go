package data

import "gitee.com/qiulaidongfeng/nonamevote/internal/config"

type Db[T any] interface {
	Load()
	Save()
	Add(v T) (int, func())
	AddKV(key string, v T) (ok bool)
	Data(yield func(string, T) bool)
	Find(k string) T
	Delete(k string)
	AddIpCount(ip string) (r int64)
	Changed()
	Updata(key string, old any, field string, v any) (ok bool)
	IncOption(key string, i int, old any, v any) (ok bool)
	Clear()
	AddLoginNum(user string) (r int64)
}

var _ Db[any] = (*OsDb[any])(nil)
var _ Db[any] = (*RedisDb[any])(nil)

const (
	Ip = iota
	User
	Session
	Vote
	VoteName
	LoginNum
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
	case LoginNum:
		file = "./loginnum"
	}
	//如果是os模式，全部使用OsDb
	//如果是redis模式，全部使用RedisDb
	//如果是mysql-redis模式，除Ip和LoginNum数据库和Session数据库用RedisDb,其他用MysqlDb
	if os {
		r := NewOsDb(file, key)
		r.ipDb = typ == Ip
		return r
	}
	if typ == Ip || typ == LoginNum || typ == Session {
		return NewRedisDb(host, path, typ, key)
	}
	if mysql {
		user, password, addr := config.GetDsnInfo()
		return NewMysqlDb(user, password, addr, typ, key)
	}
	return NewRedisDb(host, path, typ, key)
}

var Test bool
