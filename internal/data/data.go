package data

import (
	"encoding/json"
	"os"
	"slices"
	"sync"
)

type Table[T any] struct {
	t    table[T]
	lock sync.Mutex
}

type table[T any] struct {
	Path string
	Data []T
}

func NewTable[T any](path string) *Table[T] {
	t := Table[T]{}
	t.t.Path = path
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

func (t *Table[T]) SaveToOS() {
	if Test {
		return
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	fd, err := os.OpenFile(t.t.Path, os.O_RDWR|os.O_CREATE, 0777)
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
}

func (t *Table[T]) Add(v T) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.t.Data = append(t.t.Data, v)
}

func (t *Table[T]) Data(yield func(int, T) bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	for i, v := range t.t.Data {
		if !yield(i, v) {
			break
		}
	}
}

func (t *Table[T]) Replace(new T, ok func(T) bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	for i, v := range t.t.Data {
		if ok(v) {
			t.t.Data[i] = new
			break
		}
	}
}

func (t *Table[T]) Delete(delete func(T) bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	for i, v := range t.t.Data {
		if delete(v) {
			_ = slices.Delete(t.t.Data, i, i+1)
		}
	}
}

func (t *Table[T]) DeleteIndex(i int) {
	t.lock.Lock()
	defer t.lock.Unlock()
	_ = slices.Delete(t.t.Data, i, i+1)
}

func (t *Table[T]) Len() int {
	return len(t.t.Data)
}

var Test = false
