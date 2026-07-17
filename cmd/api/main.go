package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	addr := ":" + port
	fmt.Printf("Go API dev server listening on http://localhost%s\n", addr)
	log.Fatal(fasthttp.ListenAndServe(addr, requestHandler))
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
