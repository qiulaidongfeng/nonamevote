// Package config 封装获取配置
package config

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/go-viper/encoding/ini"
	"github.com/spf13/viper"
)

var v *viper.Viper

var Test bool = os.Getenv("TEST") != ""

func newv() *viper.Viper {
	codecRegistry := viper.NewCodecRegistry()
	codecRegistry.RegisterCodec("ini", ini.Codec{})
	v := viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	prefix := ""
	if Test {
		prefix = "../"
	}
	v.SetConfigFile(prefix + "config.ini")
	v.OnConfigChange(func(e fsnotify.Event) {
		loadConfig()
		fmt.Println("Config file changed:", e.Name)
	})
	v.WatchConfig()
	err := v.ReadInConfig()
	if err != nil {
		panic(err)
	}
	return v
}

var config = struct {
	host, port           atomic.Pointer[string]
	expiration, maxcount atomic.Int64
	link                 atomic.Pointer[string]
	mode                 atomic.Pointer[string]
	mysqluser            atomic.Pointer[string]
	mysqlpassword        atomic.Pointer[string]
	mysqladdr            atomic.Pointer[string]
	redisPassword        atomic.Pointer[string]
	mongodbUser          atomic.Pointer[string]
	mongodbPassword      atomic.Pointer[string]
	iplimit_info         atomic.Pointer[string]
}{}

func loadConfig() {
	config.host.Store(ptr(v.GetString("redis.host")))
	config.port.Store(ptr(v.GetString("redis.port")))
	config.expiration.Store(int64(v.GetInt("ip_limit.expiration")))
	config.maxcount.Store(v.GetInt64("ip_limit.maxcount"))
	config.link.Store(ptr(v.GetString("link.path")))
	config.mode.Store(ptr(v.GetString("db.mode")))
	config.mysqluser.Store(ptr(v.GetString("mysql.user")))
	config.mysqlpassword.Store(ptr(v.GetString("mysql.password")))
	config.mysqladdr.Store(ptr(v.GetString("mysql.addr")))
	config.redisPassword.Store(ptr(v.GetString("redis.password")))
	config.mongodbUser.Store(ptr(v.GetString("mongodb.user")))
	config.mongodbPassword.Store(ptr(v.GetString("mongodb.password")))
	config.iplimit_info.Store(ptr(fmt.Sprintf("%d秒内您的ip访问超过%d次，请等%d秒再访问", config.expiration.Load(), config.maxcount.Load(), config.expiration.Load())))
}

func ptr(v string) *string {
	return &v
}

// 如果数据库对数组的更新操作不需要采用cas的方法
var NoCasUpdate bool

func init() {
	v = newv()
	loadConfig()
	if GetDbMode() == "os" || GetDbMode() == "mongodb-redis" {
		NoCasUpdate = true
	}
}

func GetRedis() (host string, port string) {
	host = *config.host.Load()
	port = *config.port.Load()
	return
}

func GetExpiration() int {
	return int(config.expiration.Load())
}

func GetMaxCount() int64 {
	return config.maxcount.Load()
}

func GetLink() string {
	return *config.link.Load()
}

func GetDbMode() string {
	return *config.mode.Load()
}

func GetDsnInfo() (user, password, addr string) {
	return *config.mysqluser.Load(), *config.mysqlpassword.Load(), *config.mysqladdr.Load()
}

func GetRedisPassword() string {
	return *config.redisPassword.Load()
}

func GetMongodbUser() string {
	return *config.mongodbUser.Load()
}

func GetMongodbPassword() string {
	return *config.mongodbPassword.Load()
}

func GetIpLimitInfo() string {
	return *config.iplimit_info.Load()
}
