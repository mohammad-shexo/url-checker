package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mohammad-shexo/url-checker/internal/handlers"
	"github.com/mohammad-shexo/url-checker/internal/services"
	"github.com/mohammad-shexo/url-checker/internal/utils"
)

const (
	defaultPort    = "8080"
	requestTimeout = 3 * time.Second
	cacheTTL       = 30 * time.Second
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	httpClient := utils.NewHTTPClient(requestTimeout)
	checker := services.NewChecker(httpClient, cacheTTL)
	checkHandler := handlers.NewCheckHandler(checker)

	mux := http.NewServeMux()
	mux.Handle("/check", checkHandler)
	mux.HandleFunc("/health", healthHandler)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("URL Status Checker listening on %s", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}
