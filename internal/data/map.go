package data

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
)

type MapTable[T any] struct {
	t       maptable[T]
	key     func(T) string
	lock    sync.Mutex
	changed atomic.Bool
	Changed func()
}

type maptable[T any] struct {
	Path string
	M    sync.Map
	i    int64
}

func NewMapTable[T any](path string, key func(T) string) *MapTable[T] {
	t := MapTable[T]{key: key}
	t.t.Path = path
	t.Changed = func() {}
	return &t
}

func (t *MapTable[T]) LoadToOS() {
	if Test {
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

func (t *MapTable[T]) SaveToOS() (changed bool) {
	if Test {
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
	return t.changed.Load()
}

func (t *MapTable[T]) Add(v T) (int, func()) {
	return int(atomic.AddInt64(&t.t.i, 1)), func() { t.t.M.Store(t.key(v), v); t.changed.Store(true); t.Changed() }
}

func (t *MapTable[T]) AddKV(key string, v T) {
	t.t.M.Store(key, v)
	t.changed.Store(true)
	t.Changed()
}

func (t *MapTable[T]) Data(yield func(string, T) bool) {
	t.t.M.Range(func(key, value any) bool {
		k := key.(string)
		v := value.(T)
		return yield(k, v)
	})
}

func (t *MapTable[T]) Find(k string) T {
	v, _ := t.t.M.Load(k)
	r, _ := v.(T)
	return r
}

func (t *MapTable[T]) Delete(k string) {
	t.Changed()
	t.changed.Store(true)
	t.t.M.Delete(k)
}

var Test bool
