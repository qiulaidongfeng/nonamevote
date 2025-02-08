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
		Addr:         host + ":" + port,
		DialTimeout:  60 * time.Second,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
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
		if err != nil && !Test {
			slog.Error("", "err", err)
			return i
		}
	}
	if err != nil && !Test {
		slog.Error("", "err", err)
		return i
	}
	return i
}
