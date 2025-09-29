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
	"time"
	"unsafe"

	"github.com/qiulaidongfeng/nonamevote/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisDb[T any] struct {
	rdb *redis.Client
	key func(T) string
	db  int
}

var IpCount = NewDb[int](Ip, nil)

func NewRedisDb[T any](host string, port string, DB int, key func(T) string) *RedisDb[T] {
	rdb := redis.NewClient(&redis.Options{
		Addr:         host + ":" + port,
		DialTimeout:  60 * time.Second,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		DB:           DB,
		Password:     config.GetRedisPassword(),
	})
	r := &RedisDb[T]{rdb: rdb, key: key, db: DB}
	return r
}

func (r *RedisDb[T]) Add(v T) (int, func()) {
	return r.Inc(), func() {
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
	case "Lock", "Ip", "Os", "OsVersion", "Device", "Broswer", "CreateTime":
		return
	case "Option":
		for i := range fv.Len() {
			h["Option_name"+strconv.Itoa(i)] = fv.Index(i).Field(0).Interface()
			h["Option_num"+strconv.Itoa(i)] = fv.Index(i).Field(1).Interface()
		}
		return
	case "End": //vote.Info.End
		b, _ := fv.Interface().(time.Time).MarshalText()
		h[ft.Name] = unsafe.String(unsafe.SliceData(b), len(b))
		return
	}
	h[ft.Name] = fv.Interface()
}

func (r *RedisDb[T]) AddKV(k string, v T) (ok bool) {
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

	for i := 0; i < 9; i++ {
		err := r.rdb.Watch(context.Background(), func(tx *redis.Tx) error {
			i, err := tx.Exists(context.Background(), k).Result()
			if err != nil {
				return err
			}
			if i == 1 { //如果已经存在
				return nil
			}
			_, err = tx.TxPipelined(context.Background(), func(p redis.Pipeliner) error {
				//TODO:SessionMaxAge移动到其他包避免循环导入
				if err := p.Expire(context.Background(), k, 12*60*60*time.Second).Err(); err != nil {
					return err
				}
				return p.HSet(context.Background(), k, h).Err()
			})
			if err == nil {
				ok = true
			}
			return err
		}, k)
		if err == nil {
			break
		}
		if err == redis.TxFailedErr {
			i--
		} else {
			slog.Error("", "err", err)
		}
		if i == 9 {
			panic(err)
		}
	}
	return
}

func (r *RedisDb[T]) Find(k string) T {
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
				vv, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					panic(err)
				}
				rv.FieldByName(k).SetUint(uint64(uint8(vv)))
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
	if v.Len() >= i+1 {
		return
	}
	i = max(v.Len(), i+1)
	n := reflect.MakeSlice(v.Type(), i, i)
	for i := range v.Len() {
		n.Index(i).Set(v.Index(i))
	}
	v.Set(n)
}

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
			if keys[i] == "i" && r.db == Vote || (keys[i] == "key" && r.db == Session) {
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

func (r *RedisDb[T]) AddIpCount(ip string) (ret int64) {
	if Test {
		defer func() {
			ret = 0
		}()
	}
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

func (r *RedisDb[T]) AddLoginNum(user string) int64 {
	i, err := r.rdb.Incr(context.Background(), user).Result()
	if i == 1 {
		err = r.rdb.Expire(context.Background(), user, time.Duration(30)*time.Second).Err()
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
	switch field {
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

func (r *RedisDb[T]) IncField(key string, field string) {
	for i := range 10 {
		err := r.rdb.HIncrBy(context.Background(), key, field, 1).Err()
		if err == nil {
			return
		}
		slog.Error("", "err", err)
		if i == 9 {
			panic(err)
		}
	}
}

func (r *RedisDb[T]) UpdataSession(key string, index uint8, _ [16]byte, old, new any) {
	r.Updata(key, old, "Session", new)
}

func (r *RedisDb[T]) IncOption(key string, i int, _, _ any) (ok bool) {
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

// 为实现接口而写，实际无效果
func (r *RedisDb[T]) Load() {}

// 为实现接口而写，实际无效果
func (r *RedisDb[T]) Save() {}

// 为实现接口而写，实际无效果
func (r *RedisDb[T]) Changed() {}

func (r *RedisDb[T]) LoadOrStoreStr(key, value string) (string, bool) {
	b, err := r.rdb.SetNX(context.Background(), key, value, 0).Result()
	if err != nil {
		panic(err)
	}
	if b {
		return value, b
	}
	s, err := r.rdb.Get(context.Background(), key).Result()
	if err != nil {
		panic(err)
	}
	return s, false
}
