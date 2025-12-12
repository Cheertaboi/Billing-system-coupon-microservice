package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Cheertaboi/Billing-system-coupon-microservice/internal/api"
	"github.com/Cheertaboi/Billing-system-coupon-microservice/internal/api/middleware"
	"github.com/Cheertaboi/Billing-system-coupon-microservice/pkg/db"
)

func main() {
	// load DB config from env
	cfg, _ := db.LoadPostgresConfig()

	conn, err := db.NewPostgresConnection(cfg)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer conn.Close()

	// create handler with repos & services
	handler := api.NewRouter(conn)

	// add middleware if needed (example: logger)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Mount("/", handler)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// graceful shutdown
	idleConnsClosed := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		// we received an interrupt signal, shut down.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Println("starting coupon-service on :8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %s\n", err)
	}

	<-idleConnsClosed
	log.Println("server stopped")
}
