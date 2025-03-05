package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/run"
	"github.com/redis/go-redis/v9"
)

type RedisDb[T any] struct {
	rdb     *redis.Client
	r       redisDb
	key     func(T) string
	changed func()
	db      int
}

type redisDb struct {
	i int64
}

var IpCount = NewDb[int](Ip, nil)

func NewRedisDb[T any](host string, port string, DB int, key func(T) string) *RedisDb[T] {
	rdb := redis.NewClient(&redis.Options{
		Addr:         host + ":" + port,
		DialTimeout:  60 * time.Second,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		DB:           DB,
	})
	r := &RedisDb[T]{rdb: rdb, key: key, db: DB}
	r.Load()
	r.changed = run.Ticker(func() {
		r.Save()
	})
	return r
}

func (r *RedisDb[T]) Load() {
	if r.db == Vote {
		var v int64
		for i := range 10 {
			c := r.rdb.Get(context.Background(), "i")
			val, err := c.Result()
			if err == nil {
				i, err := strconv.Atoi(val)
				if err != nil {
					panic(err)
				}
				v = int64(i)
				break
			}
			if err == redis.Nil {
				return
			}
			slog.Error(val, "err", err)
			if i == 9 {
				panic(err)
			}
		}
		r.r.i = v
	}
}
func (r *RedisDb[T]) Save() {
	if r.db == Vote {
		for i := range 10 {
			c := r.rdb.Set(context.Background(), "i", atomic.LoadInt64(&r.r.i), 0)
			val, err := c.Result()
			if err == nil {
				break
			}
			slog.Error(val, "err", err)
			if i == 9 {
				panic(err)
			}
		}
	}
}

func (r *RedisDb[T]) Add(v T) (int, func()) {
	r.Changed()
	return int(atomic.AddInt64(&r.r.i, 1)), func() {
		rv := reflect.ValueOf(v)
		rt := reflect.TypeFor[T]()
		if rv.Kind() == reflect.Pointer {
			rv = rv.Elem()
			rt = rt.Elem()
		}
		num := rv.NumField()
		h := make(map[string]any, num)
		for i := range num {
			fv := rv.Field(i)
			add(h, fv, rt.Field(i))
		}

		for i := range 10 {
			c := r.rdb.HSet(context.Background(), r.key(v), h)
			_, err := c.Result()
			if err == nil {
				break
			}
			slog.Error("", "err", err)
			if i == 9 {
				panic(err)
			}
		}
	}
}

func add(h map[string]any, fv reflect.Value, ft reflect.StructField) {
	switch ft.Name {
	case "VotedPath", "Session", "Path", "Comment": //account.User.VotedPath,account.User.Session,vote.NameAndPath.Path,vote.Info.Comment
		b, err := json.Marshal(fv.Interface())
		if err != nil {
			panic(err)
		}
		h[ft.Name] = unsafe.String(unsafe.SliceData(b), len(b))
		return
	case "Lock", "Ip":
		return
	case "Option":
		for i := range fv.Len() {
			h["Option_name"+strconv.Itoa(i)] = fv.Index(i).Field(0).Interface()
			h["Option_num"+strconv.Itoa(i)] = fv.Index(i).Field(1).Interface()
		}
		return
	case "End", "CreateTime": //vote.Info.End,account.Session.CreateTime
		b, _ := fv.Interface().(time.Time).MarshalText()
		h[ft.Name] = unsafe.String(unsafe.SliceData(b), len(b))
		return
	}
	h[ft.Name] = fv.Interface()
}

func (r *RedisDb[T]) AddKV(k string, v T) {
	r.Changed()
	rv := reflect.ValueOf(v)
	rt := reflect.TypeFor[T]()
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
		rt = rt.Elem()
	}
	num := rv.NumField()
	h := make(map[string]any, num)
	for i := range num {
		fv := rv.Field(i)
		add(h, fv, rt.Field(i))
	}

	for i := range 10 {
		c := r.rdb.HSet(context.Background(), k, h)
		_, err := c.Result()
		if err == nil {
			break
		}
		slog.Error("", "err", err)
		if i == 9 {
			panic(err)
		}
	}
}

func (r *RedisDb[T]) Find(k string) T {
	if r.db == Ip {
		c := r.rdb.Get(context.Background(), k)
		r, _ := c.Result()
		var ret T
		a := any(&ret).(*int)
		*a, _ = strconv.Atoi(r)
		return ret
	}

	var ret map[string]string
	var err error
	for i := range 10 {
		ret, err = r.rdb.HGetAll(context.Background(), k).Result()
		if err == nil || err == redis.Nil {
			break
		}
		slog.Error("", "k", k, "err", err)
		if i == 9 {
			panic(err)
		}
	}
	var result T
	rv := reflect.ValueOf(result)
	set := false
	if rv.Type().Kind() == reflect.Pointer {
		n := reflect.New(rv.Type().Elem())
		result = n.Interface().(T)
		rv = n.Elem()
		l := rv.FieldByName("Lock")
		if l.Kind() == reflect.Interface {
			l.Set(reflect.ValueOf(mu))
		}
	} else {
		n := reflect.New(rv.Type())
		set = true
		rv = n.Elem()
	}
	if len(ret) != 0 {
		for k, v := range ret {
			switch k {
			case "VotedPath", "Session", "Path", "Comment":
				t := rv.FieldByName(k).Type()
				p := reflect.New(t)
				err := json.Unmarshal(unsafe.Slice(unsafe.StringData(v), len(v)), p.Interface())
				if err != nil {
					panic(err)
				}
				rv.FieldByName(k).Set(p.Elem())
				continue
			case "SessionIndex":
				vv, err := strconv.ParseUint(v, 10, 8)
				if err != nil {
					panic(err)
				}
				rv.FieldByName(k).SetUint(uint64(vv))
				continue
			case "End", "CreateTime":
				var t time.Time
				if err := t.UnmarshalText(unsafe.Slice(unsafe.StringData(v), len(v))); err != nil {
					fmt.Println(k, v[1:len(v)-1])
					panic(err)
				}
				rv.FieldByName(k).Set(reflect.ValueOf(t))
				continue
			}
			if strings.Contains(k, "Option_name") {
				i, err := strconv.Atoi(k[len("Option_name"):])
				if err != nil {
					panic(err)
				}
				grow(rv.FieldByName("Option"), i)

				rv.FieldByName("Option").Index(i).Field(0).SetString(v)
				continue
			}
			if strings.Contains(k, "Option_num") {
				i, err := strconv.Atoi(k[len("Option_num"):])
				if err != nil {
					panic(err)
				}
				vv, err := strconv.Atoi(v)
				if err != nil {
					panic(err)
				}

				grow(rv.FieldByName("Option"), i)

				rv.FieldByName("Option").Index(i).Field(1).SetInt(int64(vv))
				continue
			}
			rv.FieldByName(k).SetString(v)
		}
	} else {
		var z T
		result = z
	}
	if set {
		result = rv.Interface().(T)
	}

	return result
}

func grow(v reflect.Value, i int) {
	i = max(v.Len(), i+1)
	n := reflect.MakeSlice(v.Type(), i, i)
	for i := range v.Len() {
		n.Index(i).Set(v.Index(i))
	}
	v.Set(n)
}

type fackLock struct{}

func (_ fackLock) Lock()   {}
func (_ fackLock) Unlock() {}

var mu fackLock

func (r *RedisDb[T]) Data(yield func(string, T) bool) {
	var cursor uint64
	i := 0
	for {
		var keys []string
		var err error
		keys, cursor, err = r.rdb.Scan(context.Background(), cursor, "*", 100).Result()
		if err != nil {
			i++
			if i == 9 {
				panic(err)
			}
			slog.Error("", "err", err)
			continue
		}
		for i := range keys {
			if keys[i] == "i" && r.db == Vote {
				continue
			}
			v := r.Find(keys[i])
			if !yield(keys[i], v) {
				return
			}
		}
		if cursor == 0 {
			return
		}
	}
}

func (r *RedisDb[T]) Delete(k string) {
	r.Changed()
	for i := range 10 {
		c := r.rdb.Del(context.Background(), k)
		code, err := c.Result()
		if err != nil {
			slog.Error("", "code", code, "err", err)
			if i == 9 {
				panic(err)
			}
		}
		break
	}
}

func (r *RedisDb[T]) Changed() {
	r.changed()
}

func (r *RedisDb[T]) AddIpCount(ip string) (ret int64) {
	if Test {
		defer func() {
			ret = 0
		}()
	}
	r.Changed()
	i, err := r.rdb.Incr(context.Background(), ip).Result()
	if i == 1 {
		err = r.rdb.Expire(context.Background(), ip, time.Duration(config.GetExpiration())*time.Second).Err()
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

func (r *RedisDb[T]) Updata(key string, old any, field string, v any) (ok bool) {
	r.Changed()
	switch field {
	case "SessionIndex":
		old = strconv.Itoa(int(old.(uint8)))
		v = strconv.Itoa(int(v.(uint8)))
	case "Session", "VotedPath", "Comment", "Path":
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		v = unsafe.String(unsafe.SliceData(b), len(b))

		b, err = json.Marshal(old)
		if err != nil {
			panic(err)
		}
		old = unsafe.String(unsafe.SliceData(b), len(b))
	default:
		panic(field)
	}

	err := r.rdb.Watch(context.Background(), func(tx *redis.Tx) error {
		// 获取当前值
		oldVal, err := tx.HGet(context.Background(), key, field).Result()
		if err != nil && err != redis.Nil {
			return err
		}

		// 执行事务
		_, err = tx.TxPipelined(context.Background(), func(pipe redis.Pipeliner) error {
			if old == oldVal {
				c := pipe.HSet(context.Background(), key, field, v)
				_, err = c.Result()
				return err
			}
			return errors.New("")
		})
		return err
	}, key)
	return err == nil
}

func (r *RedisDb[T]) IncOption(key string, i int, _, _ any) (ok bool) {
	r.Changed()
	field := "Option_num" + strconv.Itoa(i)
	for i := range 10 {
		c := r.rdb.HIncrBy(context.Background(), key, field, 1)
		_, err := c.Result()
		if err == nil {
			return true
		}
		slog.Error("", "err", err)
		if i == 9 {
			panic(err)
		}
	}
	return true
}

// Inc 用于生成自增值
func (r *RedisDb[T]) Inc() int {
	i, err := r.rdb.Incr(context.Background(), "i").Result()
	if err != nil {
		slog.Error("", "err", err)
	}
	return int(i)
}

func (r *RedisDb[T]) Clear() {
	r.rdb.FlushDB(context.Background())
}
