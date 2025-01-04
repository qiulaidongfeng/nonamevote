package rss

import (
	"strconv"
	"strings"

	"nonamevote/internal/config"
	"nonamevote/internal/vote"

	"github.com/gorilla/feeds"
)

func Generate() string {
	link := config.GetLink()
	feed := &feeds.Feed{
		Title:       "无记名投票",
		Description: "一个无记名投票的网站",
		Link:        &feeds.Link{Href: link},
	}
	for _, v := range vote.Db.Data {
		var buf strings.Builder
		for _, o := range v.Option {
			buf.WriteString("选项")
			buf.WriteString(o.Name)
			buf.WriteString("得票")
			buf.WriteString(strconv.Itoa(o.GotNum))
			buf.WriteString("\n")
		}
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       v.Name,
			Description: v.Introduce,
			Content:     buf.String(),
			Link:        &feeds.Link{Href: link + v.Path},
		})
	}
	s, err := feed.ToRss()
	if err != nil {
		panic(err)
	}
	return s
}
