package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	handler "notes-go-backend/api"
)

func main() {
	loadEnvFile(".env.local")
	loadEnvFile(".env")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/categories", handler.Categories)
	mux.HandleFunc("/api/forgot-password", handler.ForgotPassword)
	mux.HandleFunc("/api/notes", handler.Notes)
	mux.HandleFunc("/api/reset-password", handler.ResetPassword)
	mux.HandleFunc("/api/settings", handler.Settings)
	mux.HandleFunc("/api/signin", handler.Signin)
	mux.HandleFunc("/api/signup", handler.Signup)

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
