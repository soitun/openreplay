package main

import (
	"log"
	"openreplay/backend/internal/config/http"
	"openreplay/backend/internal/http/router"
	"openreplay/backend/internal/http/server"
	"openreplay/backend/internal/http/services"
	"openreplay/backend/pkg/monitoring"
	"os"
	"os/signal"
	"syscall"

	"openreplay/backend/pkg/db/cache"
	"openreplay/backend/pkg/db/postgres"
	"openreplay/backend/pkg/pprof"
	"openreplay/backend/pkg/queue"
)

/*
HTTP
*/

func main() {
	metrics := monitoring.New("http")

	log.SetFlags(log.LstdFlags | log.LUTC | log.Llongfile)
	pprof.StartProfilingServer()

	// Load configuration
	cfg := http.New()

	// Connect to queue
	producer := queue.NewProducer()
	defer producer.Close(15000)

	// Connect to database
	dbConn := cache.NewPGCache(postgres.NewConn(cfg.Postgres), 1000*60*20)
	defer dbConn.Close()

	// Build all services
	services := services.New(cfg, producer, dbConn)

	// Init server's routes
	router, err := router.NewRouter(cfg, services, metrics)
	if err != nil {
		log.Fatalf("failed while creating engine: %s", err)
	}

	// Init server
	server, err := server.New(router.GetHandler(), cfg.HTTPHost, cfg.HTTPPort, cfg.HTTPTimeout)
	if err != nil {
		log.Fatalf("failed while creating server: %s", err)
	}

	// Run server
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v\n", err)
		}
	}()
	log.Printf("Server successfully started on port %v\n", cfg.HTTPPort)

	// Wait stop signal to shut down server gracefully
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	<-sigchan
	log.Printf("Shutting down the server\n")
	server.Stop()
}
