// Package data 提供数据库操作
package data

import "gitee.com/qiulaidongfeng/nonamevote/internal/config"

// Db 表示一个数据库的封装
// 现有实现
//   - 可持久化的内存数据库
//   - redis
//   - mysql
//   - mongodb
type Db[T any] interface {
	// Load 读取数据
	Load()
	// Save 保存数据
	Save()
	// Add 添加一个数据，返回一个唯一的表示符和实际执行添加的函数
	// 确保创建投票时为每一个投票分配不同的路径
	Add(v T) (int, func())
	// AddKV 立即添加一个数据
	AddKV(key string, v T) (ok bool)
	// Data 遍历所有数据
	Data(yield func(string, T) bool)
	// Find 查找数据
	Find(k string) T
	// Delete 删除数据
	Delete(k string)
	// AddIpCount 增加ip的访问计数
	AddIpCount(ip string) (r int64)
	// Changed 表示数据有修改
	Changed()
	// Update 更新数据的指定字段
	Updata(key string, old any, field string, v any) (ok bool)
	// IncOption 增加投票指定选项的得票数
	IncOption(key string, i int, old any, v any) (ok bool)
	// Clear 清空数据库
	Clear()
	// AddLoginNum 增加用户的登录次数计数
	AddLoginNum(user string) (r int64)
	// IncField 让数据的指定字段自增
	IncField(key string, field string)
	// UpdataSession 更新session
	UpdataSession(key string, index uint8, v [16]byte, old, new any)
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

// NewDb 新建保存指定类型数据的数据库操作实现
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
	if config.GetDbMode() == "mongodb-redis" {
		return NewMongoDb[T](typ, key)
	}
	if mysql {
		user, password, addr := config.GetDsnInfo()
		return NewMysqlDb(user, password, addr, typ, key)
	}
	return NewRedisDb(host, path, typ, key)
}

var Test bool
