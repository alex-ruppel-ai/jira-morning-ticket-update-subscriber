# MySQL Example: Tasks App

This is a working reference schema and Go wiring you can adapt for any MySQL-backed feature.

## App Config

```toml
# project.toml
enable_mysql = true
```

After enabling, env vars provided automatically:
- `MYSQL_INSTANCE_CONNECTION_NAME`
- `MYSQL_DB_USER` = `agentic-app-template-sa`
- `MYSQL_DB_NAME` = `agentic_app_template`

---

## Example Schema: Tasks

```sql
CREATE TABLE IF NOT EXISTS tasks (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    status      ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
    created_by  VARCHAR(100),   -- Slack user ID
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

---

## Example Seed Data

```sql
INSERT INTO tasks (title, description, status, created_by) VALUES
  ('Set up database', 'Enable MySQL in project.toml and deploy', 'done', 'U01ABC123'),
  ('Build task API', 'POST /api/tasks and GET /api/tasks endpoints', 'in_progress', 'U01ABC123'),
  ('Add frontend tab', 'Show task list in the Tasks tab', 'todo', 'U01ABC456');
```

---

## Example Go Wiring (`api.go`)

```go
import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "net"
    "net/http"
    "os"
    "time"

    "cloud.google.com/go/cloudsqlconn"
    "github.com/gin-gonic/gin"
    "github.com/go-sql-driver/mysql"
)

// Task represents a row in the tasks table.
type Task struct {
    ID          int       `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Status      string    `json:"status"`
    CreatedBy   string    `json:"created_by"`
    CreatedAt   time.Time `json:"created_at"`
}

// initMySQL returns a *sql.DB for either local (proxy) or Cloud SQL.
func initMySQL(ctx context.Context) (*sql.DB, error) {
    dbUser := os.Getenv("MYSQL_DB_USER")
    dbName := os.Getenv("MYSQL_DB_NAME")
    if dbUser == "" || dbName == "" {
        return nil, fmt.Errorf("missing MYSQL_DB_USER or MYSQL_DB_NAME")
    }

    instanceConnectionName := os.Getenv("MYSQL_INSTANCE_CONNECTION_NAME")
    if instanceConnectionName == "" {
        // Local: connect via proxy (apps-platform app connect-db --connect)
        dsn := fmt.Sprintf("%s@tcp(localhost:3306)/%s?parseTime=true", dbUser, dbName)
        db, err := sql.Open("mysql", dsn)
        if err != nil {
            return nil, err
        }
        if err := db.PingContext(ctx); err != nil {
            return nil, err
        }
        log.Printf("MySQL: connected locally to %s", dbName)
        return db, nil
    }

    dialer, err := cloudsqlconn.NewDialer(ctx,
        cloudsqlconn.WithIAMAuthN(),
        cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
    if err != nil {
        return nil, err
    }

    mysql.RegisterDialContext("cloudsql", func(ctx context.Context, addr string) (net.Conn, error) {
        return dialer.Dial(ctx, instanceConnectionName)
    })

    dsn := fmt.Sprintf("%s@cloudsql(%s)/%s?parseTime=true", dbUser, instanceConnectionName, dbName)
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, err
    }
    if err := db.PingContext(ctx); err != nil {
        return nil, err
    }
    log.Printf("MySQL: connected via Cloud SQL IAM auth")
    return db, nil
}

// migrateMySQL runs schema migrations on startup.
func migrateMySQL(db *sql.DB) error {
    _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS tasks (
            id          INT AUTO_INCREMENT PRIMARY KEY,
            title       VARCHAR(255) NOT NULL,
            description TEXT,
            status      ENUM('todo','in_progress','done') NOT NULL DEFAULT 'todo',
            created_by  VARCHAR(100),
            created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
        )
    `)
    return err
}

// registerTaskRoutes wires task API endpoints onto the Gin router.
func registerTaskRoutes(r *gin.Engine, db *sql.DB) {
    r.GET("/api/tasks", func(c *gin.Context) {
        rows, err := db.QueryContext(c.Request.Context(),
            "SELECT id, title, description, status, created_by, created_at FROM tasks ORDER BY created_at DESC")
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        defer rows.Close()

        var tasks []Task
        for rows.Next() {
            var t Task
            if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedBy, &t.CreatedAt); err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
                return
            }
            tasks = append(tasks, t)
        }
        c.JSON(http.StatusOK, tasks)
    })

    r.POST("/api/tasks", func(c *gin.Context) {
        var body struct {
            Title       string `json:"title" binding:"required"`
            Description string `json:"description"`
            CreatedBy   string `json:"created_by"`
        }
        if err := c.ShouldBindJSON(&body); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        result, err := db.ExecContext(c.Request.Context(),
            "INSERT INTO tasks (title, description, created_by) VALUES (?, ?, ?)",
            body.Title, body.Description, body.CreatedBy)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        id, _ := result.LastInsertId()
        c.JSON(http.StatusCreated, gin.H{"id": id})
    })
}
```

---

## Wiring into `main.go`

```go
func main() {
    ctx := context.Background()

    db, err := initMySQL(ctx)
    if err != nil {
        log.Fatalf("MySQL init failed: %v", err)
    }
    defer db.Close()

    if err := migrateMySQL(db); err != nil {
        log.Fatalf("MySQL migration failed: %v", err)
    }

    r := gin.Default()
    registerTaskRoutes(r, db)
    // ... other routes
    r.Run(":8081")
}
```

---

## Key Rules

- **Always** check `MYSQL_INSTANCE_CONNECTION_NAME` to branch local vs. cloud paths
- **Always** call `db.PingContext` after `sql.Open` — Open alone does not verify connectivity
- **Run migrations on startup** via `CREATE TABLE IF NOT EXISTS` — safe to re-run
- **Never** store passwords — IAM auth is used in production; the local proxy handles auth
- **Verify vendor**: confirm `github.com/go-sql-driver/mysql` and `cloud.google.com/go/cloudsqlconn` exist under `vendor/` before importing

---

## Testing

You **cannot** test MySQL locally — the Cloud SQL proxy is not available in this environment.

To verify your changes:
1. Deploy: use the `apps-platform` skill to run `apps-platform app deploy`
2. Check logs: use the `apps-platform` skill to run `apps-platform app logs`
3. Look for `"MySQL: connected via Cloud SQL IAM auth"` and any migration errors in the log output
