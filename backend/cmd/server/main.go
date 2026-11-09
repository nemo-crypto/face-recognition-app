package main

import (
	"context"
	"errors"
	"face-demo/backend/internal/app"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	addr := flag.String("addr", "127.0.0.1:8080", "监听地址")
	data := flag.String("data", "data", "数据目录")
	web := flag.String("web", "web/dist", "前端构建目录")
	flag.Parse()
	token := strings.TrimSpace(os.Getenv("FACE_API_TOKEN"))
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return errors.New("addr 必须包含主机与端口")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) && len(token) < 24 {
		return errors.New("非回环监听必须设置至少 24 字符的 FACE_API_TOKEN")
	}
	release, err := app.Lock(*data)
	if err != nil {
		return err
	}
	defer release()
	store, err := app.OpenStore(*data)
	if err != nil {
		return err
	}
	var provider app.Provider = app.DemoProvider{}
	if endpoint := os.Getenv("FACE_PROVIDER_URL"); endpoint != "" {
		threshold := 0.8
		if value := os.Getenv("FACE_THRESHOLD"); value != "" {
			threshold, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return errors.New("FACE_THRESHOLD 无效")
			}
		}
		provider, err = app.NewRemoteProvider(endpoint, os.Getenv("FACE_PROVIDER_TOKEN"), threshold)
		if err != nil {
			return err
		}
	}
	server := &http.Server{Addr: *addr, Handler: app.NewServer(store, provider, token, *web).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Printf("Face Studio: http://%s，模式=%s", *addr, provider.Mode())
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
