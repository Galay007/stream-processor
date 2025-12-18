package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {

	RedisAddr := os.Getenv("REDIS_ADDR")
	if RedisAddr == "" {
		RedisAddr = "localhost:6379"
	}

	ServicePort := ":" + os.Getenv("PORT")
	if ServicePort == "" {
		ServicePort = ":8080"
	}
	log.Print(RedisAddr)
	log.Print(ServicePort)

	Initialize(RedisAddr)

	r := SetupRoutes()

	log.Printf("Starting stream processor on port %s", ServicePort)
	log.Printf("Metri available on http://localhost%s/metrics", ServicePort)

	srv := &http.Server{
		Addr:         ServicePort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v", ServicePort, err)
	}
}
