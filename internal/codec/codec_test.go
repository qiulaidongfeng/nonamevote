package codec_test

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"gitee.com/qiulaidongfeng/nonamevote/internal/account"
	. "gitee.com/qiulaidongfeng/nonamevote/internal/codec"
)

func TestCodeSession(t *testing.T) {
	s := account.Session{
		Value:      "19063",
		Ip:         account.IPInfo{Country: "c"},
		Os:         "k",
		CreateTime: time.Now(),
	}
	result := account.Session{}
	c := Encode(s)
	t.Log(c)
	DeCode(&result, c)
	if !reflect.DeepEqual(s, result) && !s.CreateTime.Equal(result.CreateTime) {
		t.Fatalf("%+v != %+v", s.EnCode(), result.EnCode())
	}
}

func TestCodeSessionCookie(t *testing.T) {
	s := account.Session{
		Value:      "19063",
		Ip:         account.IPInfo{Country: "c b"},
		Os:         "k",
		CreateTime: time.Now(),
	}
	result := account.Session{}
	c := Encode(s)
	t.Log(c)
	c = url.QueryEscape(c)
	t.Log(c)
	c, err := url.QueryUnescape(c)
	if err != nil {
		panic(err)
	}
	DeCode(&result, c)
	if !reflect.DeepEqual(s, result) && !s.CreateTime.Equal(result.CreateTime) {
		t.Fatalf("%+v != %+v", s.EnCode(), result.EnCode())
	}
}
