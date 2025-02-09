package config

import (
	"fmt"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var v *viper.Viper

func newv() *viper.Viper {
	v := viper.New()
	v.SetConfigFile("config.ini")
	v.AddConfigPath("./")

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
}{}

func loadConfig() {
	config.host.Store(ptr(v.GetString("redis.host")))
	config.port.Store(ptr(v.GetString("redis.port")))
	config.expiration.Store(int64(v.GetInt("ip_limit.expiration")))
	config.maxcount.Store(v.GetInt64("ip_limit.maxcount"))
	config.link.Store(ptr(v.GetString("link.path")))
	config.mode.Store(ptr(v.GetString("db.mode")))
}

func ptr(v string) *string {
	return &v
}

func init() {
	v = newv()
	loadConfig()
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
