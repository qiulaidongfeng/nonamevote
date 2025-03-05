package data

import (
	"encoding/json"

	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/run"

	"os"
	"sync"
	"sync/atomic"
	"time"
)

type OsDb[T any] struct {
	t       osdb[T]
	key     func(T) string
	lock    sync.Mutex
	changed func()
	ipDb    bool
}

type osdb[T any] struct {
	Path string
	M    sync.Map
	i    int64
}

func NewOsDb[T any](path string, key func(T) string) *OsDb[T] {
	t := OsDb[T]{key: key}
	t.t.Path = path
	t.changed = run.Ticker(func() {
		t.Save()
	})
	t.Load()
	return &t
}

func (t *OsDb[T]) Load() {
	if Test || t.ipDb {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	fd, err := os.OpenFile(t.t.Path, os.O_RDWR|os.O_APPEND, 0777)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	defer fd.Close()
	dn := json.NewDecoder(fd)
	m := make(map[string]T)
	d := struct {
		M map[string]T
		I int64
	}{m, 0}
	err = dn.Decode(&d)
	if err != nil {
		panic(err)
	}
	for k, v := range m {
		t.t.M.Store(k, v)
	}
	atomic.StoreInt64(&t.t.i, d.I)
}

func (t *OsDb[T]) Save() {
	if Test || t.ipDb {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	fd, err := os.OpenFile(t.t.Path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		panic(err)
	}
	defer fd.Close()
	m := make(map[string]T)
	t.t.M.Range(func(key, value any) bool {
		k := key.(string)
		v := value.(T)
		m[k] = v
		return true
	})
	d := struct {
		M map[string]T
		I int64
	}{m, atomic.LoadInt64(&t.t.i)}
	j, err := json.MarshalIndent(&d, "", "    ")
	if err != nil {
		panic(err)
	}
	_, err = fd.Write(j)
	if err != nil {
		panic(err)
	}
	return
}

func (t *OsDb[T]) Add(v T) (int, func()) {
	return int(atomic.AddInt64(&t.t.i, 1)), func() { t.t.M.Store(t.key(v), v); t.Changed() }
}

func (t *OsDb[T]) AddKV(key string, v T) {
	t.t.M.Store(key, v)
	t.Changed()
}

func (t *OsDb[T]) Data(yield func(string, T) bool) {
	t.t.M.Range(func(key, value any) bool {
		k := key.(string)
		v := value.(T)
		return yield(k, v)
	})
}

func (t *OsDb[T]) Find(k string) (ret T) {
	v, ok := t.t.M.Load(k)
	if !ok {
		return
	}
	return v.(T)
}

func (t *OsDb[T]) Delete(k string) {
	t.Changed()
	t.t.M.Delete(k)
}

func (t *OsDb[T]) Changed() {
	t.changed()
}

func (t *OsDb[T]) AddIpCount(ip string) (r int64) {
	if Test {
		defer func() {
			r = 0
		}()
	}
	for {
		v, ok := t.t.M.Load(ip)
		if !ok {
			p := int64(1)
			old, load := t.t.M.Swap(ip, &p)
			if !load {
				time.AfterFunc(time.Duration(config.GetExpiration())*time.Second, func() {
					t.t.M.Delete(ip)
				})
				return
			}
			v = old
		}
		p := v.(*int64)
		return atomic.AddInt64(p, 1)
	}
}

// 为实现接口而写，实际无效果
func (t *OsDb[T]) Updata(key string, old any, field string, v any) (ok bool) { return true }

// 为实现接口而写，实际无效果
func (t *OsDb[T]) IncOption(key string, i int, old any, v any) (ok bool) { return true }

// 为实现接口而写，实际无效果
func (t *OsDb[T]) Clear() {}
