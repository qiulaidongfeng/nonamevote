package data

import (
	"encoding/json"
	"os"
	"slices"
	"sync"
	"sync/atomic"
)

type Table[T any] struct {
	t       table[T]
	lock    sync.RWMutex
	changed atomic.Bool
	Changed func()
}

type table[T any] struct {
	Path string
	Data []T
}

func NewTable[T any](path string) *Table[T] {
	t := Table[T]{}
	t.t.Path = path
	t.Changed = func() {}
	return &t
}

func (t *Table[T]) LoadToOS() {
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
	err = dn.Decode(&t.t)
	if err != nil {
		panic(err)
	}
}

func (t *Table[T]) SaveToOS() (changed bool) {
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
	j, err := json.MarshalIndent(&t.t, "", "    ")
	if err != nil {
		panic(err)
	}
	_, err = fd.Write(j)
	if err != nil {
		panic(err)
	}
	return t.changed.Load()
}

func (t *Table[T]) Add(v T) int {
	t.changed.Store(true)
	t.Changed()
	t.lock.Lock()
	defer t.lock.Unlock()
	t.t.Data = append(t.t.Data, v)
	return len(t.t.Data)
}

func (t *Table[T]) Data(yield func(int, T) bool) {
	t.changed.Store(true)
	t.Changed()
	i := 0
	for {
		t.lock.Lock()
		if i >= len(t.t.Data) {
			t.lock.Unlock()
			break
		}
		v := t.t.Data[i]
		t.lock.Unlock()
		if !yield(i, v) {
			break
		}
		i++
	}
}

func (t *Table[T]) Replace(new T, ok func(T) bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	for i, v := range t.t.Data {
		if ok(v) {
			t.changed.Store(true)
			t.Changed()
			t.t.Data[i] = new
			break
		}
	}
}

func (t *Table[T]) Delete(delete func(T) bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	for i := 0; i < len(t.t.Data); i++ {
		v := t.t.Data[i]
		if delete(v) {
			t.changed.Store(true)
			t.Changed()
			t.t.Data = slices.Delete(t.t.Data, i, i+1)
		}
	}
}

func (t *Table[T]) DeleteIndex(i int) {
	t.changed.Store(true)
	t.Changed()
	t.lock.Lock()
	defer t.lock.Unlock()
	t.t.Data = slices.Delete(t.t.Data, i, i+1)
}

var Test = false
