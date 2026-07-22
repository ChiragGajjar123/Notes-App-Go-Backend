package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"notes-go-backend/pkg/auth"
	"notes-go-backend/pkg/database"

	"github.com/valyala/fasthttp"
)

func requestHandler(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	switch path {
	case "/api/categories":
		categoriesHandler(ctx)
	case "/api/forgot-password":
		forgotPasswordHandler(ctx)
	case "/api/notes":
		notesHandler(ctx)
	case "/api/reset-password":
		resetPasswordHandler(ctx)
	case "/api/settings":
		settingsHandler(ctx)
	case "/api/signin":
		signinHandler(ctx)
	case "/api/signup":
		signupHandler(ctx)
	case "/api/health":
		ctx.SetStatusCode(fasthttp.StatusOK)
		_, _ = ctx.Write([]byte("ok"))
	default:
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		_, _ = ctx.Write([]byte("not found"))
	}
}

func main() {
	loadEnvFile(".env.local")
	loadEnvFile(".env")

	// ── Eager initialization ────────────────────────────────────────────
	// Initialize MongoDB connection at startup so first request doesn't pay the cost
	if _, err := database.ConnectDB(); err != nil {
		log.Printf("Warning: failed to connect to MongoDB at startup: %v", err)
	}

	// Cache the INTERNAL_API_KEY so auth checks avoid os.Getenv syscalls
	auth.InitAuth()

	// ── Server configuration ────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	server := &fasthttp.Server{
		Handler:                       requestHandler,
		Concurrency:                   512 * 1024,      // 512K concurrent connections (default 256K)
		ReadBufferSize:                8 * 1024,         // 8KB read buffer per connection
		WriteBufferSize:               8 * 1024,         // 8KB write buffer per connection
		MaxRequestBodySize:            1 * 1024 * 1024,  // 1MB max request body
		ReadTimeout:                   10 * time.Second, // prevent slow-read attacks
		WriteTimeout:                  10 * time.Second, // prevent write stalls
		IdleTimeout:                   120 * time.Second, // keep idle connections alive longer
		DisableKeepalive:              false,            // keep connections alive for reuse
		DisableHeaderNamesNormalizing: true,             // skip header normalization for max speed
		ReduceMemoryUsage:             false,            // trade memory for speed
		TCPKeepalive:                  true,             // TCP-level keepalive
		TCPKeepalivePeriod:            15 * time.Second, // fast TCP keepalive probe
	}

	addr := ":" + port

	// ── Graceful shutdown ───────────────────────────────────────────────
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("Go API dev server listening on http://localhost%s\n", addr)
		if err := server.ListenAndServe(addr); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-done
	log.Println("Shutting down gracefully...")

	// Give in-flight requests time to complete
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.ShutdownWithContext(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	// Disconnect MongoDB
	if client := database.GetClient(); client != nil {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("MongoDB disconnect error: %v", err)
		}
	}

	log.Println("Server stopped.")
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		_ = os.Setenv(key, value)
	}
}
