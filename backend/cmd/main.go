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
	libraryPath := flag.String("library", "", "Path to library folder (required)")
	port := flag.Int("port", 7754, "Server port (HTTP)")
	httpsPort := flag.Int("https-port", 7755, "HTTPS port (0 to disable)")
	convexURL := flag.String("convex-url", "", "URL of the Convex Optimizer Python service (e.g., http://localhost:7756)")
	watch := flag.Bool("watch", false, "Enable auto-reload when CSV lookup tables change")
	autoloadBooks := flag.Bool("autoload-books", false, "Enable automatic loading of event books at startup (uses more memory)")
	flag.Parse()

	// Check environment variable for convex URL if not provided via flag
	if *convexURL == "" {
		if envURL := os.Getenv("CONVEX_OPTIMIZER_URL"); envURL != "" {
			*convexURL = envURL
		}
	}

	if *libraryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -library flag is required")
		fmt.Fprintln(os.Stderr, "Usage: lutexplorer -library <path/to/library> [-port 7754] [-https-port 7755]")
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	httpsAddr := fmt.Sprintf(":%d", *httpsPort)

	// Load index from library folder
	loader := lut.NewLoaderFromLibrary(*libraryPath)
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
	if *autoloadBooks {
		bgLoader.Start()
		log.Println("Background loader started (low priority mode)")
	} else {
		log.Println("Events lazy loading enabled (memory efficient)")
		log.Println("  - Events loaded on-demand when viewing individual spins")
		log.Println("  - Use -autoload-books to preload all events (high memory)")
	}

	// Create CSV watcher for auto-reload on file changes (optional)
	var csvWatcher *watcher.FileWatcher
	if *watch {
		csvFiles := loader.GetCSVFiles()
		var watcherErr error
		csvWatcher, watcherErr = watcher.NewFileWatcher(loader.BaseDir(), csvFiles, func(mode string) error {
			log.Printf("CSV file changed, reloading LUT for mode: %s", mode)
			if reloadErr := loader.ReloadModeTable(mode); reloadErr != nil {
				return reloadErr
			}
			// Broadcast to WebSocket clients
			hub.Broadcast(ws.Message{
				Type: ws.MsgLUTReloaded,
				Payload: map[string]string{
					"mode":    mode,
					"message": "Lookup table reloaded",
				},
			})
			return nil
		})
		if watcherErr != nil {
			log.Printf("Warning: Failed to create CSV watcher: %v", watcherErr)
		} else {
			if startErr := csvWatcher.Start(); startErr != nil {
				log.Printf("Warning: Failed to start CSV watcher: %v", startErr)
			} else {
				log.Println("CSV watcher started (auto-reload on lookup table changes)")
			}
		}
	} else {
		log.Println("CSV watcher disabled (use --watch to enable)")
	}

	// Create and configure server
	server := api.NewServer(loader, addr, hub, *convexURL)
	server.SetBackgroundLoader(bgLoader)
	server.SetCSVWatcher(csvWatcher)

	// Log convex optimizer status
	if *convexURL != "" {
		log.Printf("Convex Optimizer proxy enabled: %s", *convexURL)
	} else {
		log.Println("Convex Optimizer proxy disabled (no -convex-url or CONVEX_OPTIMIZER_URL)")
	}

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		if csvWatcher != nil {
			csvWatcher.Stop()
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
