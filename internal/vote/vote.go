package vote

import (
	"errors"
	"html/template"
	"log/slog"
	"nonamevote/internal/account"
	"nonamevote/internal/data"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Info struct {
	Name        string
	End         time.Time
	Introduce   string
	Path        string
	Html        string
	NologinHtml string
	Option      []Option
}

type Option struct {
	Name   string
	GotNum int
}

// ParserCreateVote 从post请求表单中获取创建投票的信息
func ParserCreateVote(ctx *gin.Context) (Info, error) {
	var ret Info
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

	len := Db.Add(&ret)

	type gen struct {
		Info
		Logined bool
	}

	{ // 生成登录用户的投票网页
		var b strings.Builder
		err = votetmpl.Execute(&b, gen{Info: ret, Logined: true})
		if err != nil {
			slog.Error("", "err", err)
			return ret, errors.New("生成投票网页失败")
		}
		ret.Html = b.String()
	}
	{ // 生成没登录用户的投票网页
		var b strings.Builder
		err = votetmpl.Execute(&b, gen{Info: ret, Logined: false})
		if err != nil {
			slog.Error("", "err", err)
			return ret, errors.New("生成投票网页失败")
		}
		ret.NologinHtml = b.String()
	}
	path := "vote/" + strconv.Itoa(len)
	ret.Path = path
	AddVoteHtml(&ret)

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

var Db = data.NewTable[*Info]("./vote")

func init() {
	Db.LoadToOS()
}

func AddVoteHtml(v *Info) {
	S.GET(v.Path, func(ctx *gin.Context) {
		//先检查是否登录
		ok, err := account.CheckLogined(ctx)
		if err != nil {
			ctx.String(401, err.Error())
			return
		}
		//根据是否登录决定能看到的网页，不登录不能投票
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		if ok {
			ctx.String(200, v.Html)
		} else {
			ctx.String(200, v.NologinHtml)
		}
	})
}

var S *gin.Engine

var tmpl = filepath.Join("."+string(filepath.Separator), "template")

var votetmpl = func() *template.Template {
	t := template.New("vote")
	m := template.FuncMap{
		"getOption": func(name string) []Option {
			for _, v := range Db.Data {
				if v.Name == name {
					return v.Option
				}
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
