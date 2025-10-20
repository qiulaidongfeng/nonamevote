package account

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/maxmind/mmdbinspect/pkg/mmdbinspect"
	"github.com/oschwald/maxminddb-golang"
	"github.com/qiulaidongfeng/nonamevote/internal/config"
	"github.com/qiulaidongfeng/nonamevote/internal/data"
	"github.com/qiulaidongfeng/nonamevote/internal/safe"
	"github.com/qiulaidongfeng/safesession"
)

var SessionControl = safesession.NewControl(safe.Aeskey, sessionMaxAge, 0, func(clientIp string) safesession.IPInfo {
	i, err := getIPInfo(clientIp)
	if err != nil {
		panic(err)
	}
	return i
}, safesession.DB{
	Store: func(ID string, CreateTime time.Time) bool {
		return SessionDb.AddKV(ID, safesession.Session{ID: ID, CreateTime: CreateTime})
	},
	Delete: func(ID string) {
		SessionDb.Delete(ID)
	},
	Exist: func(ID string) bool {
		return SessionDb.Find(ID).ID == ID
	},
	Valid: func(UserName string, SessionID string) error {
		user := UserDb.Find(UserName)
		if user == nil {
			SessionDb.Delete(SessionID)
			return errors.New("没有这个用户 " + UserName)
		}
		m := md5.Sum(unsafe.Slice(unsafe.StringData(SessionID), len(SessionID)))
		if !slices.Contains(user.Session[:], m) {
			SessionDb.Delete(SessionID)
			return safesession.LoginExpired
		}
		return nil
	},
})

type Session = safesession.Session

func NewSession(ctx *gin.Context, Name string) Session {
	return SessionControl.NewSession(ctx.ClientIP(), ctx.Request.UserAgent(), Name)
}

var findIpMode = 1

var mmdb_db *maxminddb.Reader

func init() {
	mode := os.Getenv("nonamevote_findIP")
	if mode == "remote" {
		findIpMode = 2
	} else {
		mmdb_db = func() *maxminddb.Reader {
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
	}
}

func getIPInfo(ip string) (safesession.IPInfo, error) {
	if findIpMode == 1 {
		var m map[string]any
		mmdb_db.Lookup(net.ParseIP(ip), &m)
		if len(m) == 0 {
			return safesession.IPInfo{Country: ""}, nil
		}
		country := m["country_name"].(string)
		slog.Info("", "country", country)
		return safesession.IPInfo{Country: country}, nil
	}
	// 使用一个公共的IP地理位置API服务
	apiURL := "http://ip-api.com/json/" + ip

	var location safesession.IPInfo
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

var SessionDb = data.NewDb(data.Session, func(s Session) string { return s.ID })

const SessionMaxAge = 12 * 60 * 60 //12小时

const sessionMaxAge = time.Hour * 12

func init() {
	if config.GetDbMode() == "os" {
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
}

// CheckLogined 检查是否已经登录
func CheckLogined(ctx *gin.Context) (bool, error, Session) {
	cs, err := ctx.Request.Cookie("session")
	if err == http.ErrNoCookie {
		return false, nil, Session{}
	}
	if err != nil {
		panic(err)
	}
	return SessionControl.CheckLogined(ctx.ClientIP(), ctx.Request.UserAgent(), cs)
}
