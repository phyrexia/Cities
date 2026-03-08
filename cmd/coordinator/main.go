package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cities/game/internal/coordinator"
)

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Println("╔══════════════════════════════════════╗")
	log.Println("║   CITIES — Social Simulator v0.1     ║")
	log.Println("║   Coordinator Server                 ║")
	log.Println("╚══════════════════════════════════════╝")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[Server] Shutting down...")
		cancel()
		os.Exit(0)
	}()

	server := coordinator.NewServer()
	if err := server.Start(ctx, addr); err != nil {
		log.Fatalf("[Server] Fatal: %v", err)
	}
}
