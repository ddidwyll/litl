package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"litl/lib/config"
	"litl/lib/server"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	port := flag.Int("port", 8765, "Listen port")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}

	srv := server.New(cfg, *port)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	defer stop()

	go func() {
		log.Printf("Server starting on 127.0.0.1:%d", *port)
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed to start: %s", err)
		}
	}()

	<-ctx.Done()
	log.Println("Server shutting down...")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := srv.Stop(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v:", err)
	}

	log.Println("Server stopped")
}
