package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// localHandler wraps an http.HandlerFunc for routing in local dev
type localHandler func(http.ResponseWriter, *http.Request)

func (h localHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

func main() {
	loadEnvFile(".env.local")
	loadEnvFile(".env")

	// Import each handler file's Handler func by building per-route wrappers.
	// In Vercel each api/*.go is compiled independently; locally we wire them manually.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/categories", categoriesHandler)
	mux.HandleFunc("/api/forgot-password", forgotPasswordHandler)
	mux.HandleFunc("/api/notes", notesHandler)
	mux.HandleFunc("/api/reset-password", resetPasswordHandler)
	mux.HandleFunc("/api/settings", settingsHandler)
	mux.HandleFunc("/api/signin", signinHandler)
	mux.HandleFunc("/api/signup", signupHandler)

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	addr := ":" + port
	fmt.Printf("Go API dev server listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
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
