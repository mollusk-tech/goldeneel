package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"messagingapi/internal/config"
	"messagingapi/internal/db"
	"messagingapi/internal/httpserver"
)

func main() {
	cfg := config.Load()

	dbConn, err := db.Connect(cfg.DBDSN)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer dbConn.Close()

	if err := db.RunMigrationsAndSeed(dbConn); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	r := httpserver.NewRouter(dbConn, cfg)

	// Update last active on each request happens via middleware in router

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if cfg.EnableTLS {
		certFile := cfg.TLSCertPath
		keyFile := cfg.TLSKeyPath
		if _, err := os.Stat(certFile); err == nil {
			// Serve HTTPS on HTTPS_PORT
			httpsAddr := fmt.Sprintf(":%d", cfg.HTTPSPort)
			go func() {
				log.Printf("HTTPS server listening on %s", httpsAddr)
				if err := httpserver.ListenAndServeTLS(r, httpsAddr, certFile, keyFile); err != nil && err != http.ErrServerClosed {
					log.Fatalf("https server error: %v", err)
				}
			}()
		} else {
			log.Printf("ENABLE_TLS=true but cert/key not found at %s / %s; falling back to HTTP", certFile, keyFile)
		}
	}

	log.Printf("HTTP server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}