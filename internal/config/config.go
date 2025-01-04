package config

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var v = func() *viper.Viper {
	v := viper.New()
	v.SetConfigFile("config.ini")
	v.AddConfigPath("./")

	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})
	v.WatchConfig()
	err := v.ReadInConfig()
	if err != nil {
		panic(err)
	}
	return v
}()

func GetRedis() (host string, port string) {
	host = v.GetString("redis.host")
	port = v.GetString("redis.port")
	return
}

func GetExpiration() int {
	return v.GetInt("redis.expiration")
}

func GetMaxCount() int64 {
	return v.GetInt64("redis.maxcount")
}

func GetLink() string {
	return v.GetString("link.path")
}