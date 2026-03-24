# Architecture Patterns

## Go + React (Recommended)

The fastest deployment pattern. Frontend is embedded in the Go binary using `go:embed`.

**Structure:**
```
my-app/
├── main.go              # Go backend with embedded frontend
├── go.mod
├── go.sum
├── project.toml         # Apps Platform config
├── Makefile             # Build commands
└── frontend/
    ├── src/             # React source
    ├── dist/            # Built assets (embedded)
    ├── package.json
    └── vite.config.js
```

**main.go pattern:**
```go
package main

//go:generate sh -c "cd frontend && npm run build"

import (
    "embed"
    "io/fs"
    "net/http"
    "os"

    "github.com/gin-gonic/gin"
)

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
    r := gin.Default()

    // API routes
    api := r.Group("/api")
    {
        api.GET("/health", func(c *gin.Context) {
            c.JSON(http.StatusOK, gin.H{"status": "healthy"})
        })
        // Add your API endpoints here
    }

    // Serve embedded frontend
    distFS, _ := fs.Sub(frontendFS, "frontend/dist")
    r.NoRoute(func(c *gin.Context) {
        c.FileFromFS(c.Request.URL.Path, http.FS(distFS))
    })

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    r.Run(":" + port)
}
```

**Deploy:**
```bash
apps-platform app deploy --no-build  # Fastest: ~30 seconds
```

## Python Flask

**Structure:**
```
my-app/
├── app.py               # Flask application
├── requirements.txt     # Python dependencies
├── Procfile            # gunicorn app:app
├── project.toml
├── static/             # CSS, images
└── templates/          # Jinja2 templates
```

**app.py pattern:**
```python
import os
from flask import Flask, render_template, jsonify

app = Flask(__name__)

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/api/health')
def health():
    return jsonify({'status': 'healthy'})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 8080))
    app.run(host='0.0.0.0', port=port)
```

**Procfile:**
```
web: gunicorn app:app
```

## Static Site (nginx)

For documentation, landing pages, or pure frontend apps.

**Structure:**
```
my-app/
├── Dockerfile
├── nginx.conf
├── project.toml
└── static/
    ├── index.html
    └── style.css
```

**Dockerfile:**
```dockerfile
FROM nginx:alpine
COPY nginx.conf /etc/nginx/nginx.conf
COPY static /usr/share/nginx/html
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
```

**nginx.conf:**
```nginx
events { worker_connections 1024; }
http {
    include /etc/nginx/mime.types;
    server {
        listen 8080;
        root /usr/share/nginx/html;
        location / {
            try_files $uri $uri/ /index.html;
        }
    }
}
```

