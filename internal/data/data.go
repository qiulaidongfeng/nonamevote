package data

import (
	"encoding/json"
	"os"
)

type Table[T any] struct {
	Path string
	Data []T
}

func NewTable[T any](path string) Table[T] {
	return Table[T]{Path: path}
}

func (t *Table[T]) LoadToOS() {
	fd, err := os.OpenFile(t.Path, os.O_RDWR|os.O_APPEND, 0777)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	defer fd.Close()
	dn := json.NewDecoder(fd)
	err = dn.Decode(t)
	if err != nil {
		panic(err)
	}
}

func (t *Table[T]) SaveToOS() {
	fd, err := os.OpenFile(t.Path, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		panic(err)
	}
	defer fd.Close()
	j, err := json.MarshalIndent(t, "", "    ")
	if err != nil {
		panic(err)
	}
	_, err = fd.Write(j)
	if err != nil {
		panic(err)
	}
}

func (t *Table[T]) Add(v T) {
	t.Data = append(t.Data, v)
}
