package account

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"nonamevote/internal/codec"
	"nonamevote/internal/data"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
)

type Session struct {
	Value      string
	Ip         IPInfo
	CreateTime time.Time
	Os         string
	Name       string
}

func NewSession(ctx *gin.Context, Name string) Session {
	s := Session{}
	var b [256]byte
	var err error
	for {
		_, err = rand.Read(b[:])
		if err != nil {
			panic(err)
		}
		s.Value = base64.StdEncoding.EncodeToString(b[:])
		if !strings.Contains(s.Value, " ") {
			break
		}
	}
	s.CreateTime = time.Now()
	s.Name = Name
	s.Ip, err = getIPInfo(ctx.ClientIP())
	if err != nil {
		panic(err)
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
func (s *Session) Check(users Session, i int) (bool, error) {
	if users.CreateTime != s.CreateTime {
		slices.Delete(SessionDb.Data, i, i+1)
		return false, nil
	}
	//如果session过期
	if users.CreateTime.Sub(time.Now()) > sessionMaxAge {
		slices.Delete(SessionDb.Data, i, i+1)
		return false, nil
	}
	if users.Ip != s.Ip {
		if s.Ip.Country == "" {
			return false, nil
		}
		slices.Delete(SessionDb.Data, i, i+1)
		return false, errors.New("IP地址在两次登录时不在同一个国家，请重新登录")
	}
	if users.Os != s.Os {
		return false, nil
	}
	user := GetUser(users.Name)
	if user.Name == "" {
		slices.Delete(SessionDb.Data, i, i+1)
		return false, nil
	}
	m := md5.Sum(unsafe.Slice(unsafe.StringData(users.Value), len(users.Value)))
	if !slices.Contains(user.Session[:], m) {
		slices.Delete(SessionDb.Data, i, i+1)
		return false, nil
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
	for i := range SessionDb.Data {
		//TODO:优化删除过时session,避免找到一个就删除一个
		diff := SessionDb.Data[i].CreateTime.Sub(now)
		if diff > sessionMaxAge {
			slices.Delete(SessionDb.Data, i, i+1)
		}
	}
	SessionDb.SaveToOS()
}
