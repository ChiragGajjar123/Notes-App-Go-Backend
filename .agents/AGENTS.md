# Project Rules

- **Git Pushes:** Never run `git push` or push changes to any remote repository without the user's **explicit approval**. Always show what will be committed and wait for confirmation first.
- **Deployments:** Never trigger Vercel deployments manually. They happen automatically when code is pushed to Git (with permission).

# Go Backend — Architecture & Rules

## What This Project Is

This is a **standard Go HTTP server** deployed on Vercel using the **"Go" framework preset**.

- Entrypoint: `cmd/api/main.go` (standard `http.ListenAndServe` on `PORT`)
- All route logic: `cmd/api/routes.go`
- Shared packages: `pkg/` (auth, database, models, ratelimit, response)

## Critical Rules

- **Do NOT use Vercel serverless functions** (`api/*.go` files with `func Handler`). This project is a standard Go HTTP server, not a serverless function project.
- **Do NOT add an `api/` directory** with individual `.go` handler files. All routes are registered in `cmd/api/main.go` and implemented in `cmd/api/routes.go`.
- **Do NOT add `vercel.json` with a `functions` block.** The "Go" preset auto-detects `cmd/api/main.go`.
- The server must listen on `os.Getenv("PORT")` — Vercel injects this at runtime.

## Project Structure

```
Notes App Go Backend/
├── cmd/api/
│   ├── main.go      ← server entry point — registers all routes, listens on PORT
│   └── routes.go    ← all HTTP route handler implementations
├── pkg/
│   ├── auth/        ← INTERNAL_API_KEY validation
│   ├── database/    ← MongoDB singleton connection
│   ├── models/      ← note.go, auth.go, user.go (MongoDB operations)
│   ├── ratelimit/   ← rate limiting via MongoDB
│   └── response/    ← JSON response helpers
├── go.mod           ← module: notes-go-backend, Go 1.26
├── go.sum
├── .env.example     ← template for required env vars
└── .gitignore
```

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `MONGODB_URI` | ✅ | MongoDB connection string |
| `MONGODB_DB` | optional | Database name (defaults to `notes-app`) |
| `INTERNAL_API_KEY` | ✅ | Shared secret — must match Next.js frontend |
| `PORT` | auto | Injected by Vercel at runtime |

## Development Commands

```bash
# Run locally
go run ./cmd/api

# Build
go build ./...

# Vet
go vet ./...
```

## API Routes

| Route | Methods | Auth Required |
|---|---|---|
| `/api/health` | GET | None (public) |
| `/api/signin` | POST | `X-Internal-Key` header |
| `/api/signup` | POST | `X-Internal-Key` header |
| `/api/notes` | GET, POST, DELETE | `X-Internal-Key` + `X-User-ID` headers |
| `/api/categories` | GET, POST, PUT, DELETE | `X-Internal-Key` + `X-User-ID` headers |
| `/api/settings` | GET, PUT | `X-Internal-Key` + `X-User-ID` headers |
| `/api/forgot-password` | POST | `X-Internal-Key` header |
| `/api/reset-password` | POST | `X-Internal-Key` header |

## Vercel Deployment

- **Framework Preset:** Go (finds `cmd/api/main.go` automatically)
- **Repo:** `https://github.com/ChiragGajjar123/Notes-App-Go-Backend.git`
- **Live URL:** `https://notes-app-go-backend.vercel.app`
