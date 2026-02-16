package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

func main() {
	port := flag.Int("port", 8080, "HTTP API port")
	idleTimeout := flag.Duration("idle-timeout", 0, "Shut down after inactivity (e.g., 60s, 0=never)")
	flag.Parse()

	registry := api.NewSimRegistry()
	router := api.NewRouter(registry)

	addr := fmt.Sprintf(":%d", *port)

	if *idleTimeout > 0 {
		serveWithIdleTimeout(addr, router, *idleTimeout)
	} else {
		fmt.Printf("API server running on %s\n", addr)
		fmt.Println("Try: curl -X POST http://localhost:8080/simulations -d '{\"seed\":42}'")
		if err := http.ListenAndServe(addr, router); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}

func serveWithIdleTimeout(addr string, handler http.Handler, timeout time.Duration) {
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())

	// Wrap handler to track request activity
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastActivity.Store(time.Now().UnixNano())
		handler.ServeHTTP(w, r)
	})

	srv := &http.Server{Addr: addr, Handler: wrapped}

	// Idle monitor
	go func() {
		for { // justified:WL — polling with timeout
			time.Sleep(time.Second)
			last := time.Unix(0, lastActivity.Load())
			if time.Since(last) > timeout {
				fmt.Println("Idle timeout reached, shutting down")
				srv.Shutdown(context.Background())
				return
			}
		}
	}()

	// Signal handler
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Printf("Signal %v received, shutting down\n", sig)
		srv.Shutdown(context.Background())
	}()

	fmt.Printf("API server running on %s (idle timeout: %s)\n", addr, timeout)
	fmt.Println("Try: curl -X POST http://localhost:8080/simulations -d '{\"seed\":42}'")

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
