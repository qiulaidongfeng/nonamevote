package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	_ "time/tzdata"

	"gitee.com/qiulaidongfeng/nonamevote/nonamevote"
)

func main() {
	srv := &http.Server{
		Addr:    ":443",
		Handler: nonamevote.S.Handler(),
	}
	end := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		fmt.Println("正在关机")
		err := srv.Shutdown(context.Background())
		if err != nil {
			slog.Error("", "err", err)
		}
		nonamevote.Close()
		close(end)
		fmt.Println("关机完成")
	}()
	err := srv.ListenAndServeTLS("./cert.pem", "./key.pem")
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
	<-end
}
