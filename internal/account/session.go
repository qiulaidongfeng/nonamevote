package account

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"
	"unsafe"

	"gitee.com/qiulaidongfeng/nonamevote/internal/codec"
	"gitee.com/qiulaidongfeng/nonamevote/internal/config"
	"gitee.com/qiulaidongfeng/nonamevote/internal/data"
	"github.com/gin-gonic/gin"
	"github.com/maxmind/mmdbinspect/pkg/mmdbinspect"
	"github.com/mileusna/useragent"
	"github.com/oschwald/maxminddb-golang"
)

var Test = false

type Session struct {
	Value         string `gorm:"primaryKey;type:char(64)"`
	Ip            IPInfo `json:"-" gorm:"-:all"`
	CreateTime    time.Time
	Os, OsVersion string `json:"-" gorm:"-:all"`
	Name          string `json:"-" gorm:"-:all"`
	Device        string `json:"-" gorm:"-:all"`
	Broswer       string `json:"-" gorm:"-:all"`
}

func NewSession(ctx *gin.Context, Name string) Session {
	s := Session{}
	var b [32]byte
	var err error
	_, err = rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	s.Value = base64.StdEncoding.EncodeToString(b[:])
	s.CreateTime = time.Now()
	s.Name = Name
	if !Test { //不要在测试时获取IP属地
		s.Ip, err = getIPInfo(ctx.ClientIP())
		if err != nil {
			panic(err)
		}
	}
	u := useragent.Parse(ctx.Request.UserAgent())
	s.Device = u.Device
	s.Os = u.OS
	s.OsVersion = u.OSVersion
	s.Broswer = u.Name
	return s
}

func (s *Session) Load(v string) bool {
	return codec.DeCode(s, v)
}

func (s *Session) EnCode() string {
	return codec.Encode(s)
}

// Check 检查用户的session是否有效
func (s *Session) Check(ctx *gin.Context, cookie *http.Cookie) (bool, error) {
	if s.CreateTime.Sub(time.Now()) >= sessionMaxAge {
		return false, errors.New("登录已失效，请重新登录")
	}
	//如果是测试或创建session时没有获得ip对应的地区，就不要检查ip对于的地区是否一致
	if !Test && s.Ip.Country != "" {
		userIp, err := getIPInfo(ctx.ClientIP())
		if err != nil {
			slog.Error("", "err", err)
		}
		if userIp != s.Ip && s.Ip.Country != "" && userIp.Country != "" {
			SessionDb.Delete(s.Value)
			return false, errors.New("IP地址在两次登录时不在同一个地区，请重新登录")
		}
	}
	u := useragent.Parse(ctx.Request.UserAgent())
	if u.OS != s.Os || u.Device != s.Device || u.OSVersion != s.OsVersion || u.Name != s.Broswer {
		return false, errors.New("登录疑似存在风险，请重新登录")
	}
	user := UserDb.Find(s.Name)
	if user == nil {
		SessionDb.Delete(s.Value)
		return false, errors.New("没有这个用户 " + s.Name)
	}
	m := md5.Sum(unsafe.Slice(unsafe.StringData(s.Value), len(s.Value)))
	if !slices.Contains(user.Session[:], m) {
		SessionDb.Delete(s.Value)
		return false, errors.New("登录已失效，请重新登录")
	}
	return true, nil
}

var findIpMode = 1

var mmdb_db = func() *maxminddb.Reader {
	prefix := ""
	if config.Test {
		prefix = "../"
	}
	db, err := mmdbinspect.OpenDB(prefix + "country_asn.mmdb")
	if err != nil {
		panic(err)
	}
	return db
}()

func init() {
	mode := os.Getenv("nonamevote_findIP")
	if mode == "remote" {
		findIpMode = 2
	}
}

type IPInfo struct {
	Country string `json:"country"`
}

func getIPInfo(ip string) (IPInfo, error) {
	if findIpMode == 1 {
		var m map[string]any
		mmdb_db.Lookup(net.ParseIP(ip), &m)
		if len(m) == 0 {
			return IPInfo{Country: ""}, nil
		}
		country := m["country_name"].(string)
		slog.Info("", "country", country)
		return IPInfo{Country: country}, nil
	}
	// 使用一个公共的IP地理位置API服务
	apiURL := "http://ip-api.com/json/" + ip

	var location IPInfo
	resp, err := http.Get(apiURL)
	if err != nil {
		return location, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return location, err
	}

	if err := json.Unmarshal(body, &location); err != nil {
		return location, err
	}

	return location, nil
}

var SessionDb = data.NewDb(data.Session, func(s Session) string { return s.Value })

const SessionMaxAge = 12 * 60 * 60 //12小时

const sessionMaxAge = time.Hour * 12

func init() {
	now := time.Now()
	for k, s := range SessionDb.Data {
		diff := now.Sub(s.CreateTime)
		if diff > sessionMaxAge {
			SessionDb.Delete(k)
		}
	}

	go func() {
		for {
			//每经过一次session最大有效时间，检查一次所有session，有过期的删除。
			<-time.Tick(sessionMaxAge)
			for k, v := range SessionDb.Data {
				diff := now.Sub(v.CreateTime)
				if diff > sessionMaxAge {
					SessionDb.Delete(k)
				}
			}
		}
	}()
}

// CheckLogined 检查是否已经登录
func CheckLogined(ctx *gin.Context) (bool, error, Session) {
	s, err := ctx.Request.Cookie("session")
	if err == nil {
		ok, se := DecodeSession(s.Value)
		if ok {
			v := SessionDb.Find(se.Value)
			if v.Value == se.Value {
				ok, err := se.Check(ctx, s)
				return ok, err, se
			}
		}
	} else if err != http.ErrNoCookie {
		panic(err)
	}
	return false, nil, Session{}
}

func DecodeSession(v string) (bool, Session) {
	v, err := url.QueryUnescape(v)
	if err != nil {
		slog.Error("", "err", err)
		return false, Session{}
	}
	b, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, Privkey, unsafe.Slice(unsafe.StringData(v), len(v)), nil)
	if err != nil {
		slog.Error("", "err", err)
		return false, Session{}
	}
	var se Session
	ok := se.Load(unsafe.String(&b[0], len(b)))
	return ok, se
}

var Privkey *rsa.PrivateKey
