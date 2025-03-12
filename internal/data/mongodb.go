package data

import (
	"context"
	"log"
	"reflect"
	"strconv"
	"sync"
	"unsafe"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type MongoDb[T any] struct {
	db         *mongo.Client
	dbenum     int
	collection *mongo.Collection
	key        func(T) string
}

var _ Db[any] = (*MongoDb[any])(nil)

var mongo_once = sync.OnceValue[*mongo.Client](func() *mongo.Client {
	//TODO:让addr可配置
	opt := options.Client().ApplyURI("mongodb://127.0.0.1:27017").SetMaxConnecting(100)
	db, err := mongo.Connect(opt)
	if err != nil {
		panic(err)
	}
	err = db.Ping(context.Background(), readpref.Primary())
	if err != nil {
		panic(err)
	}
	return db
})

func NewMongoDb[T any](dbenum int, key func(T) string) *MongoDb[T] {
	db := mongo_once()
	c := db.Database("vote").Collection(tablename(dbenum))
	return &MongoDb[T]{
		db:         db,
		dbenum:     dbenum,
		collection: c,
		key:        key,
	}
}

func (m *MongoDb[T]) Add(v T) (int, func()) {
	return i.Inc(), func() {
		d := m.toD(v)
		_, err := m.collection.InsertOne(context.Background(), d)
		if err != nil {
			panic(err)
		}
	}
}

func (m *MongoDb[T]) AddKV(key string, v T) (ok bool) {
	d := m.toD(v)
	_, err := m.collection.InsertOne(context.Background(), d)
	if mongo.IsDuplicateKeyError(err) {
		return false
	}
	if err != nil {
		panic(err)
	}
	return true
}

func (m *MongoDb[T]) Data(yield func(string, T) bool) {
	filter := bson.M{}
	cursor, err := m.collection.Find(context.Background(), filter)
	if err != nil {
		panic(err)
	}
	for cursor.Next(context.Background()) {
		var result bson.M
		if err = cursor.Decode(&result); err != nil {
			panic(err)
		}
		if len(result) == 0 {
			continue
		}
		v := m.toV(result)
		tmp := v.Interface().(T)
		if !yield(m.key(tmp), tmp) {
			break
		}
	}
	if err = cursor.Err(); err != nil {
		log.Fatal(err)
	}
}

func (m *MongoDb[T]) Find(k string) T {
	filter := bson.M{"_id": k}
	var result bson.M
	err := m.collection.FindOne(context.Background(), filter).Decode(&result)
	if err != nil && err.Error() != "mongo: no documents in result" {
		panic(err)
	}
	if len(result) == 0 {
		return *new(T)
	}
	return m.toV(result).Interface().(T)
}

func (m *MongoDb[T]) Delete(k string) {
	filter := bson.M{"_id": k}
	_, err := m.collection.DeleteOne(context.Background(), filter)
	if err != nil {
		panic(err)
	}
}

func (m *MongoDb[T]) Updata(key string, old any, field string, v any) (ok bool) {
	//TODO:利用要更新的在mongodb都是存为array优化
	filter := bson.M{"_id": key, field: old}
	update := bson.M{"$set": bson.M{
		field: v,
	}}
	result, err := m.collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		panic(err)
	}
	return result.ModifiedCount == 1
}

func (m *MongoDb[T]) IncOption(key string, i int, old any, v any) (ok bool) {
	filter := bson.M{"_id": key}
	update := bson.M{
		"$inc": bson.M{
			//TODO:优化字符串拼接
			"Option." + strconv.Itoa(i) + ".gotnum": 1,
		}}
	result, err := m.collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		panic(err)
	}
	return result.ModifiedCount == 1
}

func (m *MongoDb[T]) toV(b bson.M) reflect.Value {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	r := reflect.New(t)
	v := r.Elem()
	for k, value := range b {
		if value == nil {
			continue
		}
		switch k {
		case "_id":
			k = m_primary(m.dbenum)
		case "TotpURL":
			b := value.(bson.Binary)
			v.FieldByName(k).SetString(unsafe.String(unsafe.SliceData(b.Data), len(b.Data)))
			continue
		case "Session":
			f := v.FieldByName(k)
			a := value.(bson.A)
			for i := range a {
				tmp := a[i].(bson.Binary)
				f.Index(i).Set(reflect.ValueOf(([16]byte)(tmp.Data[:16])))
			}
			continue
		case "Lock":
			continue
		case "SessionIndex":
			value = uint8(value.(int32))
		case "VotedPath":
			f := v.FieldByName(k)
			a := value.(bson.A)
			s := make([]string, len(a))
			for i := range a {
				tmp := a[i].(string)
				s[i] = tmp
			}
			f.Set(reflect.ValueOf(s))
			continue
		case "Option":
			f := v.FieldByName(k)
			a := value.(bson.A)
			//TODO:避免反射创建object
			s := reflect.MakeSlice(f.Type(), len(a), len(a))
			for i := range a {
				elem := reflect.New(s.Type().Elem()).Elem()
				for _, m := range a[i].(bson.D) {
					if m.Key == "name" {
						m.Key = "Name"
					}
					if m.Key == "gotnum" {
						m.Key = "GotNum"
						m.Value = int(m.Value.(int32))
					}
					elem.FieldByName(m.Key).Set(reflect.ValueOf(m.Value))
				}
				s.Index(i).Set(elem)
			}
			f.Set(s)
			continue
		case "End":
			value = value.(bson.DateTime).Time()
		case "Path", "Comment":
			f := v.FieldByName(k)
			a := value.(bson.A)
			s := make([]string, len(a))
			for i := range a {
				s[i] = a[i].(string)
			}
			f.Set(reflect.ValueOf(s))
			continue
		}
		v.FieldByName(k).Set(reflect.ValueOf(value))
	}
	return r
}

func (m *MongoDb[T]) toD(v any) bson.M {
	r := reflect.ValueOf(v)
	if r.Kind() == r.Kind() {
		r = r.Elem()
	}
	t := r.Type()
	pri := m_primary(m.dbenum)
	ret := bson.M{}
	for i := range r.NumField() {
		f := r.Field(i)
		info := t.Field(i)
		switch info.Name {
		case pri:
			info.Name = "_id"
		case "Lock":
			continue
		case "TotpURL":
			s := f.String()
			tmp := bson.Binary{Data: unsafe.Slice(unsafe.StringData(s), len(s))}
			ret[info.Name] = tmp
			continue
		}
		ret[info.Name] = f.Interface()
	}
	return ret
}

func m_primary(db int) string {
	switch db {
	case User, VoteName:
		return "Name"
	case Vote:
		return "Path"
	}
	panic("不可达分支")
}

//TODO:复用bson.M
//TODO:研究复用Data中创建的value
//TODO:在出错时重试

func (m *MongoDb[T]) Clear() {
	filter := bson.M{}
	_, err := m.collection.DeleteMany(context.Background(), filter)
	if err != nil {
		panic(err)
	}
}

// 为实现接口而写，实际无效果
func (m *MongoDb[T]) Load() {}

// 为实现接口而写，实际无效果
func (m *MongoDb[T]) Save() {}

// 为实现接口而写，实际无效果
func (m *MongoDb[T]) Changed() {}

// 为实现接口而写，实际无效果
func (m *MongoDb[T]) AddIpCount(ip string) int64 { return 0 }

// 为实现接口而写，实际无效果
func (m *MongoDb[T]) AddLoginNum(user string) (r int64) { return 0 }
