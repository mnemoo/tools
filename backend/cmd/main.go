// LUT Explorer Backend - A utility for analyzing game lookup tables.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lutexplorer/internal/api"
	"lutexplorer/internal/bgloader"
	"lutexplorer/internal/lut"
	"lutexplorer/internal/watcher"
	"lutexplorer/internal/ws"
)

const (
	certFile = "lutexplorer.crt"
	keyFile  = "lutexplorer.key"
)

// loadOrGenerateCert loads cached certificate or generates a new one
func loadOrGenerateCert() (tls.Certificate, error) {
	// Try to load existing certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err == nil {
		log.Println("Loaded cached TLS certificate")
		return cert, nil
	}

	// Generate new certificate
	log.Println("Generating new self-signed TLS certificate...")
	return generateAndSaveCert()
}

// generateAndSaveCert generates a self-signed TLS certificate and saves it to disk
func generateAndSaveCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"LUT Explorer Dev"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Save certificate to file
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		log.Printf("Warning: could not save certificate: %v", err)
	}

	// Save private key to file
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		log.Printf("Warning: could not save private key: %v", err)
	}

	log.Printf("TLS certificate saved to %s and %s", certFile, keyFile)

	return tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}, nil
}

func main() {
	indexPath := flag.String("index", "", "Path to index.json file (required)")
	port := flag.Int("port", 7754, "Server port (HTTP)")
	httpsPort := flag.Int("https-port", 7755, "HTTPS port (0 to disable)")
	flag.Parse()

	if *indexPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -index flag is required")
		fmt.Fprintln(os.Stderr, "Usage: lutexplorer -index <path/to/index.json> [-port 7755] [-https-port 7756]")
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	httpsAddr := fmt.Sprintf(":%d", *httpsPort)

	// Load index
	loader := lut.NewLoader(*indexPath)
	if err := loader.Load(); err != nil {
		log.Fatalf("Failed to load index: %v", err)
	}

	index := loader.GetIndex()
	log.Printf("Loaded index: %d modes", len(index.Modes))

	// Print mode summaries
	for _, summary := range loader.GetModeSummaries() {
		log.Printf("  Mode %q: %d outcomes, Cost=%.2f, RTP=%.4f%%, HitRate=%.2f%%, MaxPayout=%.0fx",
			summary.Mode, summary.Outcomes, summary.Cost, summary.RTP*100, summary.HitRate*100, summary.MaxPayout)
	}

	// Create WebSocket hub
	hub := ws.NewHub()
	go hub.Run()
	log.Println("WebSocket hub started")

	// Create background loader
	bgLoader := bgloader.NewBackgroundLoader(loader, hub)
	bgLoader.Start()
	log.Println("Background loader started (low priority mode)")

	// Create book watcher for auto-reload on file changes
	bookFiles := bgLoader.GetBookFiles()
	bookWatcher, err := watcher.NewBookWatcher(loader.BaseDir(), bookFiles, func(mode string) error {
		log.Printf("Book file changed, reloading mode: %s", mode)
		return bgLoader.ReloadMode(mode)
	})
	if err != nil {
		log.Printf("Warning: Failed to create book watcher: %v", err)
	} else {
		if err := bookWatcher.Start(); err != nil {
			log.Printf("Warning: Failed to start book watcher: %v", err)
		} else {
			log.Println("Book watcher started (auto-reload on file changes)")
		}
	}

	// Create and configure server
	server := api.NewServer(loader, addr, hub)
	server.SetBackgroundLoader(bgLoader)

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		if bookWatcher != nil {
			bookWatcher.Stop()
		}
		bgLoader.Stop()
		os.Exit(0)
	}()

	// Get the HTTP handler
	handler := server.GetHandler()

	// Start HTTPS server if enabled
	if *httpsPort > 0 {
		cert, err := loadOrGenerateCert()
		if err != nil {
			log.Fatalf("Failed to load/generate TLS certificate: %v", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		httpsServer := &http.Server{
			Addr:      httpsAddr,
			Handler:   handler,
			TLSConfig: tlsConfig,
		}

		go func() {
			log.Printf("HTTPS server listening on https://localhost%s", httpsAddr)
			log.Printf("  LGS endpoints: https://localhost%s/wallet/authenticate, /wallet/play", httpsAddr)
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTPS server error: %v", err)
			}
		}()
	}

	// Start HTTP server
	log.Printf("HTTP server listening on http://localhost%s", addr)
	log.Printf("WebSocket available at ws://localhost%s/ws", addr)
	log.Printf("REST API:")
	log.Printf("  GET  /api/loader/status  - Get loading status")
	log.Printf("  POST /api/loader/boost   - Enable turbo mode (full CPU)")
	log.Printf("  DELETE /api/loader/boost - Disable turbo mode")
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
