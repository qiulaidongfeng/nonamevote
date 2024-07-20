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
	"net/http"
	"net/url"
	"nonamevote/internal/codec"
	"nonamevote/internal/data"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
)

var Test = false

type Session struct {
	Value      string
	Ip         IPInfo `json:"-"`
	CreateTime time.Time
	Os         string `json:"-"`
	Name       string `json:"-"`
}

func NewSession(ctx *gin.Context, Name string) Session {
	s := Session{}
	var b [256]byte
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
	s.Os = getOS(ctx)
	return s
}

func (s *Session) Load(v string) bool {
	return codec.DeCode(s, v)
}

func (s *Session) EnCode() string {
	return codec.Encode(s)
}

// Check 检查用户的session是否有效
func (s *Session) Check(ctx *gin.Context, cookie *http.Cookie, i int) (bool, error) {
	if s.CreateTime.Sub(time.Now()) >= sessionMaxAge {
		return false, errors.New("登录已失效，请重新登录")
	}
	//如果是测试或创建session时没有获得ip对应的国家，就不要检查ip对于的国家是否一致
	if !Test && s.Ip.Country != "" {
		userIp, err := getIPInfo(ctx.ClientIP())
		if err != nil {
			slog.Error("", "err", err)
		}
		if userIp != s.Ip && s.Ip.Country != "" && userIp.Country != "" {
			SessionDb.DeleteIndex(i)
			return false, errors.New("IP地址在两次登录时不在同一个国家，请重新登录")
		}
	}
	useros := getOS(ctx)
	if useros != s.Os {
		return false, nil
	}
	user := GetUser(s.Name)
	if user.Name == "" {
		SessionDb.DeleteIndex(i)
		return false, errors.New("没有这个用户 " + s.Name)
	}
	m := md5.Sum(unsafe.Slice(unsafe.StringData(s.Value), len(s.Value)))
	if !slices.Contains(user.Session[:], m) {
		SessionDb.DeleteIndex(i)
		return false, errors.New("登录已失效，请重新登录")
	}
	return true, nil
}

type IPInfo struct {
	Country string `json:"country"`
}

func getIPInfo(ip string) (IPInfo, error) {
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

func getOperatingSystem(userAgent string) string {
	if strings.Contains(userAgent, "Windows") {
		return "Windows"
	} else if strings.Contains(userAgent, "Mac OS X") || strings.Contains(userAgent, "Macintosh") {
		return "macOS"
	} else if strings.Contains(userAgent, "Linux") {
		return "Linux"
	}
	// 可以添加更多系统类型的检查，或返回"Unknown"
	return "Unknown"
}

func getOS(ctx *gin.Context) string {
	userAgent := ctx.Request.Header.Get("User-Agent")
	return getOperatingSystem(userAgent)
}

var SessionDb = data.NewTable[Session]("./session")

const SessionMaxAge = 1 * 60 * 60 //1小时

const sessionMaxAge = time.Hour * 1

func init() {
	SessionDb.LoadToOS()
	now := time.Now()
	SessionDb.Delete(func(s Session) bool {
		//TODO:优化删除过时session,避免找到一个就删除一个
		diff := now.Sub(s.CreateTime)
		return diff > sessionMaxAge
	})
	SessionDb.SaveToOS()
	go func() {
		for {
			//每60秒保存一次session数据库，每经过一次session最大有效时间，检查一次所有session，有过期的删除。
			select {
			case <-time.Tick(60 * time.Second):
				SessionDb.SaveToOS()
			case <-time.Tick(sessionMaxAge):
				for i, v := range SessionDb.Data {
					diff := now.Sub(v.CreateTime)
					if diff > sessionMaxAge {
						SessionDb.DeleteIndex(i)
					}
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
			for i, v := range SessionDb.Data {
				if v.Value == se.Value {
					ok, err := se.Check(ctx, s, i)
					return ok, err, se

				}
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
