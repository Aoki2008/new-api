package main

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed templates/*.html static/*
var embeddedFiles embed.FS

func main() {
	cfg := LoadConfig()

	db, sqlDB, err := openDB(cfg)
	if err != nil {
		log.Fatalf("open db failed: %v", err)
	}
	defer sqlDB.Close()

	listTmpl, detailTmpl, staticHandler, err := loadAssets()
	if err != nil {
		log.Fatalf("load templates failed: %v", err)
	}

	s := &Server{
		cfg:        cfg,
		db:         db,
		listTmpl:   listTmpl,
		detailTmpl: detailTmpl,
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.routes(staticHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("audit-server listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func loadAssets() (listTmpl, detailTmpl *template.Template, staticHandler http.Handler, err error) {
	listTmpl, err = template.ParseFS(embeddedFiles, "templates/layout.html", "templates/list.html")
	if err != nil {
		return nil, nil, nil, err
	}
	detailTmpl, err = template.ParseFS(embeddedFiles, "templates/layout.html", "templates/detail.html")
	if err != nil {
		return nil, nil, nil, err
	}
	staticFS, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		return nil, nil, nil, err
	}
	staticHandler = http.FileServer(http.FS(staticFS))
	return listTmpl, detailTmpl, staticHandler, nil
}
