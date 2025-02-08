package data

import (
	"context"
	"log/slog"
	"nonamevote/internal/config"
	"time"

	"github.com/redis/go-redis/v9"
)

type KVDB struct {
	rdb *redis.Client
}

var IpCount = newKVDB(config.GetRedis())

func newKVDB(host string, port string) KVDB {
	rdb := redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: "", // 没有密码，默认值
		DB:       0,  // 默认DB 0
	})
	return KVDB{rdb: rdb}
}

func (k *KVDB) AddIpCount(ip string) (r int64) {
	if Test {
		defer func() {
			r = 0
		}()
	}
	i, err := k.rdb.Incr(context.Background(), ip).Result()
	if i == 1 {
		err = k.rdb.Expire(context.Background(), ip, time.Duration(config.GetExpiration())*time.Second).Err()
		if err != nil {
			slog.Error("", "err", err)
			return i
		}
	}
	if err != nil {
		slog.Error("", "err", err)
		return i
	}
	return i
}
