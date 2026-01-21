package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/binaryphile/sofdevsim-2026/internal/api"
)

func main() {
	port := flag.Int("port", 8080, "HTTP API port")
	flag.Parse()

	registry := api.NewSimRegistry()
	router := api.NewRouter(registry)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("API server running on %s\n", addr)
	fmt.Println("Try: curl -X POST http://localhost:8080/simulations -d '{\"seed\":42}'")

	if err := http.ListenAndServe(addr, router); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
