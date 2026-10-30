package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"quizbox/engine/internal/api"
	"quizbox/engine/internal/store"
)

var version = "dev"

func main() {
	host := flag.String("host", "127.0.0.1", "监听地址")
	port := flag.Int("port", 54310, "监听端口，0 表示动态分配")
	dataDir := flag.String("data-dir", "data", "数据目录")
	desktop := flag.Bool("desktop", false, "桌面模式：允许回环地址静默登录本机账号")
	webDir := flag.String("web-dir", "", "前端构建目录；设置后由引擎直接托管网页版")
	showVersion := flag.Bool("version", false, "打印版本并退出")
	verifyDB := flag.String("verify-db", "", "校验指定数据库完整性后退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	if *verifyDB != "" {
		if err := verifyDatabase(*verifyDB); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	absDir, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatalf("resolve data dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(absDir, "logs"), 0o755); err != nil {
		log.Fatalf("create log dir: %v", err)
	}
	logFile, err := os.OpenFile(
		filepath.Join(absDir, "logs", "engine-"+time.Now().Format("20060102")+".log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644,
	)
	if err != nil {
		log.Printf("open log file failed, fallback to stdout: %v", err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}

	st, err := store.Open(absDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if *desktop {
		if _, err := st.EnsureLocalUser(); err != nil {
			log.Printf("ensure local user: %v", err)
		}
	}
	if *webDir != "" {
		info, err := os.Stat(*webDir)
		if err != nil || !info.IsDir() {
			log.Fatalf("web-dir invalid: %s", *webDir)
		}
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(*host, strconv.Itoa(*port)))
	if err != nil && *port != 0 {
		log.Printf("listen %d: %v, fallback to dynamic port", *port, err)
		ln, err = net.Listen("tcp", net.JoinHostPort(*host, "0"))
	}
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	actualPort := ln.Addr().(*net.TCPAddr).Port

	stop := make(chan struct{})
	var stopOnce sync.Once
	requestStop := func() { stopOnce.Do(func() { close(stop) }) }

	server := &http.Server{
		Handler:           api.New(st, version, absDir, os.Getenv("QUIZBOX_SHUTDOWN_TOKEN"), actualPort, *desktop, *webDir, requestStop),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		requestStop()
	}()

	log.Printf("quizbox engine %s listening on %s:%d, data=%s", version, *host, actualPort, absDir)
	fmt.Printf("ENGINE_PORT=%d\n", actualPort)

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
	log.Print("engine stopped")
}

func verifyDatabase(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}
	var userVersion int
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	fmt.Printf("ok: user_version=%d\n", userVersion)
	return nil
}
