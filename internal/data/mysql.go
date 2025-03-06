package data

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	dm "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var _ Db[int] = (*MysqlDb[int])(nil)

type MysqlDb[T any] struct {
	db *gorm.DB
	// key 获得主键
	key    func(T) string
	dbenum int
}

var db *gorm.DB
var once sync.Once

func NewMysqlDb[T any](user string, password string, addr string, Db int, key func(T) string) *MysqlDb[T] {
	once.Do(func() {
		dsn := fmt.Sprintf("%s:%s@tcp(%s)/vote?charset=utf8mb4&parseTime=True&loc=Local", user, password, addr)
		var err error
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			panic(err)
		}
		sqlDB.SetMaxIdleConns(100)
		sqlDB.SetMaxOpenConns(100)
	})

	model, _, _ := getModel[T]()
	err := db.AutoMigrate(model)
	if err != nil {
		panic(err)
	}
	return &MysqlDb[T]{db: db, dbenum: Db, key: key}
}

func getModel[T any]() (model any, t reflect.Type, v reflect.Value) {
	t = reflect.TypeFor[T]()
	nt := t
	if t.Kind() == reflect.Pointer { //if t like *Seesion
		nt = t.Elem()
	}
	v = reflect.New(nt)
	model = v.Interface()
	return
}

func (m *MysqlDb[T]) Add(v T) (int, func()) {
	return i.Inc(), func() {
		if err := m.db.Create(v).Error; err != nil {
			panic(err)
		}
	}
}

func (m *MysqlDb[T]) AddKV(k string, v T) (ok bool) {
	result := m.db.Create(v)
	ok = true
	if result.Error != nil {
		if e, ok := result.Error.(*dm.MySQLError); ok && e.Number == 1062 { //如果已经存在
			ok = false
		} else {
			panic(result.Error)
		}
	}
	return
}

func (m *MysqlDb[T]) Data(yield func(string, T) bool) {
	rows, err := m.db.Table(tablename(m.dbenum)).Rows()
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		model, oldt, v := getModel[T]()
		if err := m.db.ScanRows(rows, model); err != nil {
			panic(err)
		}
		tmp := toT[T](oldt, v, model)
		if !yield(m.key(tmp), tmp) {
			break
		}
	}
}

func toT[T any](oldt reflect.Type, v reflect.Value, model any) (tmp T) {
	if oldt.Kind() == reflect.Pointer { //if t like *Seesion
		v = v.Elem()
		tmp = model.(T)
	} else {
		tmp = *(model.(*T))
	}
	return
}

func (m *MysqlDb[T]) Find(k string) T {
	return m.find(m.db, k)
}

func (m *MysqlDb[T]) find(db *gorm.DB, k string) (ret T) {
	model, oldt, v := getModel[T]()
	v.Elem().FieldByName(primaryGo(m.dbenum)).Set(reflect.ValueOf(k))
	model = v.Interface()
	result := db.Find(model)
	if result.Error != nil {
		panic(result.Error)
	}
	if result.RowsAffected == 0 {
		return *new(T)
	}
	return toT[T](oldt, v, model)
}

func (m *MysqlDb[T]) Delete(k string) {
	if err := m.db.Raw("delete from ? where ?=?", tablename(m.dbenum), primary(m.dbenum), k).Error; err != nil {
		panic(err)
	}
}

func (m *MysqlDb[T]) Updata(key string, old any, field string, v any) (ok bool) {
	result := m.db.Table(tablename(m.dbenum)).Where(primary(m.dbenum)+"=?", key).Where(strings.ToLower(field)+"=?", old).Update(strings.ToLower(field), v)
	if result.Error != nil {
		panic(result.Error)
	}
	return result.RowsAffected == 1
}

func primary(db int) string {
	switch db {
	case User:
		return "name"
	case Session:
		return "value"
	case Vote:
		return "path"
	case VoteName:
		return "name"
	}
	panic("不可达分支")
}

func primaryGo(db int) string {
	switch db {
	case User:
		return "Name"
	case Session:
		return "Value"
	case Vote:
		return "Path"
	case VoteName:
		return "Name"
	}
	panic("不可达分支")
}

func tablename(db int) string {
	switch db {
	case Ip:
		return "ips"
	case User:
		return "users"
	case Session:
		return "sessions"
	case Vote:
		return "voteinfo"
	case VoteName:
		return "votenames"
	}
	panic("不可达分支")
}

func (m *MysqlDb[T]) IncOption(key string, i int, old, v any) bool {
	result := m.db.Table("voteinfo").Where("path=?", key).Where("`option`=?", old).Update("option", v)
	if err := result.Error; err != nil {
		panic(err)
	}
	return result.RowsAffected == 1
}

func (m *MysqlDb[T]) Clear() {
	for k := range m.Data {
		m.Delete(k)
	}
}

func (m *MysqlDb[T]) AddIpCount(ip string) (r int64) {
	// TODO:处理error
	m.db.Raw("update ips set num=num+1 where ip=?", ip)
	m.db.Raw("select num from ips where ip=?", ip).Scan(&r)
	return r
}

// 为实现接口而写，实际无效果
func (m *MysqlDb[T]) Load() {}

// 为实现接口而写，实际无效果
func (m *MysqlDb[T]) Save() {}

// 为实现接口而写，实际无效果
func (m *MysqlDb[T]) Changed() {}

var i = func() *RedisDb[int] {
	host, port := config.GetRedis()
	return NewRedisDb[int](host, port, 0, nil)
}()
