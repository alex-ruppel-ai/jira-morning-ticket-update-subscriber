# Best Practices

## Server Port

Always use port 8080 (Cloud Run default) or read from `PORT` environment variable:

```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
r.Run(":" + port)
```

```python
port = int(os.environ.get('PORT', 8080))
app.run(host='0.0.0.0', port=port)
```

## Health Endpoints

Include a health check endpoint. Cloud Run uses this for load balancing:

```go
api.GET("/health", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "healthy"})
})
```

```python
@app.route('/api/health')
def health():
    return jsonify({'status': 'healthy'})
```

## Error Handling

Return proper HTTP status codes. Cloud Run uses these for health checks and load balancing:
- `200-299` - Success
- `400-499` - Client error (won't trigger retries)
- `500-599` - Server error (may trigger retries)

## Logging

Use structured logging. Logs automatically appear in Cloud Logging:

```go
log.Printf("Processing request: %s", requestID)

// For structured JSON logging (recommended for production)
import "log/slog"
slog.Info("processing request", "request_id", requestID, "user", userEmail)
```

```python
import logging
logging.info(f"Processing request: {request_id}")
```

## Frontend Build

For Go+React apps, ensure frontend builds before Go compilation:

```go
//go:generate sh -c "cd frontend && npm run build"
```

Run `go generate ./...` before building.

## Common Makefile

```makefile
.PHONY: deps run build deploy clean

# Install frontend dependencies
deps:
	cd frontend && npm install

# Run locally for development
run:
	ENV=dev go run main.go

# Build for production
build:
	go generate ./...
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

# Deploy to Apps Platform
deploy:
	apps-platform app deploy --no-build

# Clean build artifacts
clean:
	rm -f app
	rm -rf frontend/dist
```

## Project Structure

Keep a clean project structure:

```
my-app/
├── main.go              # Entry point
├── go.mod
├── go.sum
├── project.toml         # Apps Platform config
├── Makefile
├── README.md
├── handlers/            # HTTP handlers
├── models/              # Data models
├── services/            # Business logic
└── frontend/            # React app (if applicable)
    ├── src/
    ├── dist/
    └── package.json
```

## Configuration

Use environment variables for configuration, not config files:

```go
// Good
dbHost := os.Getenv("DB_HOST")

// Avoid
config, _ := loadConfigFile("config.yaml")
```

## Graceful Shutdown

Handle shutdown signals for clean termination:

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
defer stop()

server := &http.Server{Addr: ":8080", Handler: router}

go func() {
    <-ctx.Done()
    server.Shutdown(context.Background())
}()

server.ListenAndServe()
```

