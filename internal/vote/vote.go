package vote

import (
	"errors"
	"html/template"
	"log/slog"
	"nonamevote/internal/account"
	"nonamevote/internal/data"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
)

type Info struct {
	Name      string
	End       time.Time
	Introduce string
	Path      string
	Option    []Option
	lock      sync.Mutex
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

	len, add := Db.Add(ret)
	path := "/vote/" + strconv.Itoa(len)
	ret.Path = path
	add()
	AddVoteHtml(ret)

	Db.SaveToOS()

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

var Db = data.NewMapTable[*Info]("./vote", func(i *Info) string { return i.Path })

var addvotelock sync.Mutex

func init() {
	Db.LoadToOS()
	go func() {
		for {
			// 每10秒保存一次数据库
			select {
			case <-time.Tick(10 * time.Second):
				Db.SaveToOS()
			}
		}
	}()
}

func AddVoteHtml(v *Info) {
	addvotelock.Lock()
	defer addvotelock.Unlock()
	S.GET(v.Path, func(ctx *gin.Context) {
		//先检查是否登录
		logined, err, _ := account.CheckLogined(ctx)
		if err != nil {
			ctx.String(401, err.Error())
			return
		}
		//根据是否登录决定能看到的网页，不登录不能投票
		type gen struct {
			*Info
			Logined bool
		}
		ret := Db.Find(v.Path)
		var b strings.Builder
		err = votetmpl.Execute(&b, gen{Info: ret, Logined: logined})
		if err != nil {
			slog.Error("", "err", err)
			ctx.String(500, "internal server error")
			return
		}
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(200, b.String())
	})
	S.POST(v.Path, func(ctx *gin.Context) {
		//先检查是否登录
		ok, err, se := account.CheckLogined(ctx)
		if err != nil {
			ctx.String(401, err.Error())
			return
		}
		if !ok {
			ctx.String(401, "需要登录才能投票")
			return
		}
		user := account.GetUser(se.Name)
		if slices.Contains(user.VotedPath, v.Path) {
			ctx.String(401, "投票失败：因为已经投过票了")
			return
		}
		dv := Db.Find(v.Path)
		if !v.End.After(time.Now()) {
			ctx.String(401, "投票失败：因为投票截止时间已经到了")
			return
		}
		option := ctx.PostForm("k")
		opt, err := strconv.Atoi(option)
		dv.lock.Lock()
		defer dv.lock.Unlock()
		if err != nil || opt >= len(dv.Option) {
			ctx.String(401, "投票失败")
			return
		}
		dv.Option[opt].GotNum++
		user.VotedPath = append(user.VotedPath, v.Path)
		account.ReplaceUser(user)
		ctx.String(200, "投票成功")
	})
}

var S *gin.Engine

var tmpl = filepath.Join("."+string(filepath.Separator), "template")

var votetmpl = func() *template.Template {
	t := template.New("vote")
	m := template.FuncMap{
		"getOption": func(name string) []Option {
			for _, v := range Db.Data {
				v.lock.Lock()
				if v.Name == name {
					v.lock.Unlock()
					return v.Option
				}
				v.lock.Unlock()
			}
			panic("未知的投票")
		}}
	t.Funcs(m)
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
		//TODO:等go模板支持range-over-func后，改为返回迭代器
		"getAllVote": func() chan *Info {
			c := make(chan *Info)
			go func() {
				for _, v := range Db.Data {
					c <- v
				}
				close(c)
			}()
			return c
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
