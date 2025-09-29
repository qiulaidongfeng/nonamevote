package vote

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/nonamevote/internal/account"
	"github.com/qiulaidongfeng/nonamevote/internal/config"
	"github.com/qiulaidongfeng/nonamevote/internal/data"
	"github.com/qiulaidongfeng/nonamevote/internal/utils"
)

type Info struct {
	Name      string
	End       time.Time
	Introduce string
	Path      string `gorm:"primaryKey"`
	Option    data.All[Option]
	Comment   data.All[string]
	Lock      sync.Mutex `json:"-" gorm:"-:all"`
}

func (*Info) TableName() string {
	return "voteinfo"
}

type Option struct {
	Name   string
	GotNum int
}

// ParserCreateVote 从post请求表单中获取创建投票的信息
func ParserCreateVote(ctx *gin.Context) (*Info, error) {
	var ret = &Info{}
	name := ctx.PostForm("name")
	if name == "" {
		return ret, errors.New("投票名不能为空")
	}
	ret.Name = name
	introduce := ctx.PostForm("introduce")
	if introduce == "" {
		return ret, errors.New("投票介绍不能为空")
	}
	ret.Introduce = introduce

	endtime := ctx.PostForm("time")
	if endtime == "" {
		return ret, errors.New("投票截止时间不能为空")
	}
	Time := strings.Split(endtime, ":")
	if len(Time) != 2 {
		return ret, errors.New("投票截止时间错误")
	}
	date := ctx.PostForm("date")
	if date == "" {
		return ret, errors.New("投票截止日期不能为空")
	}
	Date := strings.Split(date, "-")
	if len(Date) != 3 {
		return ret, errors.New("投票截止日期错误")
	}

	year, month, day := Date[0], Date[1], Date[2]
	hour, min := Time[0], Time[1]

	Year, err := strconv.Atoi(year)
	if err != nil {
		slog.Error("", "err", err)
		return ret, errors.New("投票截止日期错误")
	}
	Month, err := strconv.Atoi(month)
	if err != nil {
		slog.Error("", "err", err)
		return ret, errors.New("投票截止日期错误")
	}
	Day, err := strconv.Atoi(day)
	if err != nil {
		slog.Error("", "err", err)
		return ret, errors.New("投票截止日期错误")
	}
	Hour, err := strconv.Atoi(hour)
	if err != nil {
		slog.Error("", "err", err)
		return ret, errors.New("投票截止时间错误")
	}
	Min, err := strconv.Atoi(min)
	if err != nil {
		slog.Error("", "err", err)
		return ret, errors.New("投票截止时间错误")
	}

	ret.End = time.Date(Year, time.Month(Month), Day, Hour, Min, 0, 0, loc)

	option := ctx.PostForm("option")
	options := strings.Split(option, " ")
	if len(option) == 0 {
		return ret, errors.New("投票选项不能为空")
	}
	ret.Option = make([]Option, 0, len(options))
	for i := range options {
		ret.Option = append(ret.Option, Option{Name: options[i]})
	}

	num, add := Db.Add(ret)
	path := "/vote/" + strconv.Itoa(num)
	ret.Path = path
	add()

	n := NameDb.Find(ret.Name)
	if n == nil {
		//TODO:修复这里的竞态条件
		//如果有两个同名投票，同时执行到这里，只有一个会被记录
		n = new(NameAndPath)
		n.Name = ret.Name
		n.Path = append(n.Path, path)
		NameDb.AddKV(ret.Name, n)
	} else {
		for {
			n.Lock.Lock()
			n.Path = append(n.Path, path)
			if NameDb.Updata(ret.Name, n.Path[:len(n.Path)-1], "Path", n.Path) {
				n.Lock.Unlock()
				break
			}
			n.Lock.Unlock()
			n = NameDb.Find(ret.Name)
		}
	}
	return ret, nil
}

// 获取北京时间
var loc = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}()

type NameAndPath struct {
	Name string `gorm:"primaryKey"`
	Path data.All[string]
	Lock sync.Mutex `json:"-" gorm:"-:all"`
}

func (*NameAndPath) TableName() string {
	return "votenames"
}

var Db = data.NewDb(data.Vote, func(i *Info) string { return i.Path })
var NameDb = data.NewDb(data.VoteName, func(n *NameAndPath) string { return n.Name })

func Init() {
	S.GET("/vote/:num", func(ctx *gin.Context) {
		// 先检查是否登录
		logined, err, s := account.CheckLogined(ctx)
		if err != nil {
			ctx.String(401, err.Error())
			return
		}
		var csrf_token [32]byte
		io.ReadFull(rand.Reader, csrf_token[:])
		tmp := base64.StdEncoding.EncodeToString(csrf_token[:])
		s.CSRF_TOKEN = tmp
		utils.ChangeSession(ctx.Writer, account.UserDb.Find(s.Name), &s)
		// 根据是否登录决定能看到的网页，不登录不能投票
		type gen struct {
			*Info
			Logined    bool
			CSRF_TOKEN string
		}
		v := Db.Find(ctx.Request.URL.Path)
		if v == nil {
			ctx.Data(404, "text/html", notexist)
			return
		}
		var b strings.Builder
		v.Lock.Lock()
		err = votetmpl.Execute(&b, gen{Info: v, Logined: logined, CSRF_TOKEN: tmp})
		v.Lock.Unlock()
		if err != nil {
			//Note:这里大概是测试时执行的，然后修复，不会让用户看到
			slog.Error("", "err", err)
			ctx.String(500, "internal server error")
			return
		}
		ret := b.String()
		ctx.Data(200, "text/html", unsafe.Slice(unsafe.StringData(ret), len(ret)))
	})

	S.POST("/vote/:num", func(ctx *gin.Context) {
		//先检查是否登录
		ok, err, se := account.CheckLogined(ctx)
		if err != nil {
			ctx.Data(401, "text/html", utils.GenTipText(err.Error(), "/login", "前往登录页"))
			return
		}
		if !ok {
			ctx.Data(401, "text/html", needlogin)
			return
		}
		if se.CSRF_TOKEN != ctx.PostForm("csrf_token") {
			ctx.Data(401, "text/html", utils.GenTipText("安全验证失败", ctx.Request.URL.Path, "重试"))
			return
		}

		path := ctx.Request.URL.Path

		//处理新增评论
		if ok := ctx.Query("comment"); ok != "" {
			comment := ctx.PostForm("commentValue")
			if comment == "" {
				ctx.Data(401, "text/html", utils.GenTipText("评论失败：评论不能为空", path, "返回"))
				return
			}
			v := Db.Find(path)
			if v == nil {
				//Note:这里可能是恶意攻击，返回几个字就行
				ctx.String(404, "不存在的投票")
				return
			}
			Db.Changed()
			for {
				v.Lock.Lock()
				var old any
				if !config.NoCasUpdate {
					old = slices.Clone(v.Comment)
				}
				v.Comment = append(v.Comment, comment)
				if Db.Updata(v.Path, old, "Comment", v.Comment) {
					v.Lock.Unlock()
					break
				}
				v.Lock.Unlock()
				v = Db.Find(path)
			}
			ret := `
			<!DOCTYPE html>
			<html lang="zh-CN">
				<head>
					<meta charset="UTF-8">
					<style>
						#message {
							font-size: 20px;
							margin-top: 20px;
						}
					</style>
				</head>
				<body>
					<div id="message">评论成功，5秒后返回</div>
				</body>
				<script>
					function ret() {
						// 设置一个5秒后的定时器来跳转
						setTimeout(function() {
							// 跳转到指定路径
							window.location.href = "%s";
						}, 5000); // 5秒后执行
					}
					ret();
    			</script>
			</html>
			`
			ret = fmt.Sprintf(ret, strings.Join([]string{"https://", ctx.Request.Host, path}, ""))
			ctx.Data(200, "text/html", unsafe.Slice(unsafe.StringData(ret), len(ret)))
			return
		}

		user := account.UserDb.Find(se.Name)
		if user == nil {
			//Note:会在极短的时间，从已登录变成已注销，只可能是恶意攻击
			return
		}
		if slices.Contains(user.VotedPath, path) {
			ctx.Data(401, "text/html", utils.GenTipText("投票失败：因为已经投过票了", path, "返回"))
			return
		}
		v := Db.Find(path)
		if v == nil {
			//Note:这里可能是恶意攻击，返回几个字就行
			ctx.String(404, "不存在的投票")
			return
		}
		if !v.End.After(time.Now()) {
			ctx.Data(401, "text/html", utils.GenTipText("投票失败：因为投票截止时间已经到了", path, "返回"))
			return
		}
		option := ctx.PostForm("k")
		opt, err := strconv.Atoi(option)
		v.Lock.Lock()
		defer v.Lock.Unlock()
		if err != nil || opt >= len(v.Option) {
			//Note:这里可能是恶意攻击，返回几个字就行
			ctx.String(401, "投票失败")
			return
		}
		//Note: 这里两个for之间如果进程突然崩溃，会导致记录已投票，但没有增加得票数。
		for {
			old := user.VotedPath
			user.VotedPath = append(user.VotedPath, path)
			if account.UserDb.Updata(user.Name, old, "VotedPath", user.VotedPath) {
				break
			}
			user = account.UserDb.Find(se.Name)
			if slices.Contains(user.VotedPath, path) {
				ctx.Data(401, "text/html", utils.GenTipText("投票失败：因为已经投过票了", path, "返回"))
				return
			}
		}

		Db.Changed()
		for {
			var old any
			if !config.NoCasUpdate {
				old = slices.Clone(v.Option)
			}
			v.Option[opt].GotNum++
			if Db.IncOption(path, opt, old, v.Option) {
				break
			}
			v = Db.Find(path)
		}
		ctx.Data(200, "text/html", utils.GenTipText("投票成功", path, "返回"))
	})
}

var S *gin.Engine

var prefix = func() string {
	r := "."
	if config.Test {
		r = "../"
	}
	return r
}()

var tmpl = filepath.Join(prefix+string(filepath.Separator), "template")

var votetmpl = func() *template.Template {
	t := template.New("vote")
	file, err := os.ReadFile(filepath.Join(tmpl, "vote.temp"))
	if err != nil {
		panic(err)
	}
	t, err = t.Parse(string(file))
	if err != nil {
		panic(err)
	}
	return t
}()

var allvotetmpl = func() *template.Template {
	t := template.New("allvote")
	m := template.FuncMap{
		"getAllVote": func() func(func(*Info) bool) {
			return func(yield func(*Info) bool) {
				for _, v := range Db.Data {
					if !yield(v) {
						break
					}
				}
			}
		}}
	t.Funcs(m)
	file, err := os.ReadFile(filepath.Join(tmpl, "allvote.temp"))
	if err != nil {
		panic(err)
	}
	t, err = t.Parse(string(file))
	if err != nil {
		panic(err)
	}
	return t
}()

func AllVote(ctx *gin.Context) {
	var b strings.Builder
	err := allvotetmpl.Execute(&b, nil)
	if err != nil {
		slog.Error("", "err", err)
		ctx.String(500, "internal server error")
		return
	}
	s := b.String()
	ctx.Data(200, "text/html", unsafe.Slice(unsafe.StringData(s), len(s)))
}
