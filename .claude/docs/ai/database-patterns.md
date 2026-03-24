# Database Connection Patterns

## PostgreSQL

When `enable_postgres = true` in project.toml, these environment variables are set:
- `INSTANCE_CONNECTION_NAME` - Cloud SQL connection string
- `DB_USER` - IAM-authenticated user (service account)
- `DB_NAME` - Database name (postgres)

### Go Connection

```go
import (
    "context"
    "fmt"
    "net"
    "os"
    "strings"

    "cloud.google.com/go/cloudsqlconn"
    "github.com/jackc/pgx/v5/pgxpool"
)

func connectDB(ctx context.Context) (*pgxpool.Pool, error) {
    dbUser := os.Getenv("DB_USER")
    dbName := os.Getenv("DB_NAME")
    if dbName == "" {
        dbName = "postgres"
    }

    instanceConn := os.Getenv("INSTANCE_CONNECTION_NAME")
    if instanceConn == "" {
        // Local: connect to localhost (tunnel handles auth)
        dsn := fmt.Sprintf("host=localhost port=5432 user=%s dbname=%s sslmode=disable",
            dbUser, dbName)
        return pgxpool.New(ctx, dsn)
    }

    // Production: use Cloud SQL connector with IAM auth
    dialer, err := cloudsqlconn.NewDialer(ctx,
        cloudsqlconn.WithIAMAuthN(),
        cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
    if err != nil {
        return nil, err
    }

    config, err := pgxpool.ParseConfig(fmt.Sprintf(
        "user=%s dbname=%s sslmode=disable", dbUser, dbName))
    if err != nil {
        return nil, err
    }

    config.ConnConfig.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
        return dialer.Dial(ctx, instanceConn)
    }

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, err
    }

    // Set schema (service name with hyphens → underscores)
    schema := strings.ReplaceAll(os.Getenv("K_SERVICE"), "-", "_")
    pool.Exec(ctx, "SET search_path TO "+schema)

    return pool, nil
}
```

### Python Connection

```python
import os
from google.cloud.sql.connector import Connector
import sqlalchemy

def get_db_engine():
    db_user = os.environ.get('DB_USER')
    db_name = os.environ.get('DB_NAME', 'postgres')
    instance_conn = os.environ.get('INSTANCE_CONNECTION_NAME')

    if not instance_conn:
        # Local: connect to localhost (tunnel handles auth)
        return sqlalchemy.create_engine(
            f"postgresql+psycopg2://{db_user}@localhost:5432/{db_name}"
        )

    # Production: use Cloud SQL connector with IAM auth
    connector = Connector()

    def getconn():
        return connector.connect(
            instance_conn,
            "pg8000",
            user=db_user,
            db=db_name,
            enable_iam_auth=True,
            ip_type="private"
        )

    return sqlalchemy.create_engine("postgresql+pg8000://", creator=getconn)
```

## MySQL

When `enable_mysql = true`, these environment variables are set:
- `MYSQL_INSTANCE_CONNECTION_NAME`
- `MYSQL_DB_USER` - Format: `{app}-sa` (no @project suffix)
- `MYSQL_DB_NAME`

### Go Connection

```go
import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "net"
    "os"

    "cloud.google.com/go/cloudsqlconn"
    "github.com/go-sql-driver/mysql"
)

func initMySQL(ctx context.Context) (*sql.DB, error) {
    dbUser := os.Getenv("MYSQL_DB_USER")
    dbName := os.Getenv("MYSQL_DB_NAME")

    if dbUser == "" || dbName == "" {
        return nil, fmt.Errorf("missing required MySQL environment variables (MYSQL_DB_USER, MYSQL_DB_NAME)")
    }

    // Local mode: MYSQL_INSTANCE_CONNECTION_NAME is not set, connect via proxy
    instanceConnectionName := os.Getenv("MYSQL_INSTANCE_CONNECTION_NAME")
    if instanceConnectionName == "" {
        dsn := fmt.Sprintf("%s@tcp(localhost:3306)/%s?parseTime=true", dbUser, dbName)
        db, err := sql.Open("mysql", dsn)
        if err != nil {
            return nil, fmt.Errorf("failed to open local MySQL: %w", err)
        }
        if err := db.PingContext(ctx); err != nil {
            return nil, fmt.Errorf("failed to ping local MySQL: %w", err)
        }
        log.Printf("Connected to local MySQL: localhost:3306/%s", dbName)
        return db, nil
    }

    // Cloud SQL connector with IAM auth + private IP
    dialer, err := cloudsqlconn.NewDialer(ctx,
        cloudsqlconn.WithIAMAuthN(),
        cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
    if err != nil {
        return nil, fmt.Errorf("failed to create Cloud SQL dialer: %w", err)
    }

    mysql.RegisterDialContext("cloudsql", func(ctx context.Context, addr string) (net.Conn, error) {
        return dialer.Dial(ctx, instanceConnectionName)
    })

    // No password with IAM auth
    dsn := fmt.Sprintf("%s@cloudsql(%s)/%s?parseTime=true", dbUser, instanceConnectionName, dbName)
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    log.Printf("Connected to Cloud SQL MySQL via private IP with IAM auth")
    return db, nil
}
```

### Testing MySQL Changes

Local MySQL testing is not available. To verify changes, deploy via `apps-platform app deploy` and read logs with `apps-platform app logs`. Look for the `"Connected to Cloud SQL MySQL"` log line and any migration errors.

## Schema Access

Each app has its own PostgreSQL schema (MySQL uses separate databases):

| App Name | PostgreSQL Schema | MySQL Database |
|----------|------------------|----------------|
| `my-app` | `my_app` | `my_app` |
| `cloud-run-todo` | `cloud_run_todo` | `cloud_run_todo` |

**Always set the search path for PostgreSQL:**
```go
schema := strings.ReplaceAll(os.Getenv("K_SERVICE"), "-", "_")
pool.Exec(ctx, "SET search_path TO "+schema)
```

## Username Formats

| Database | Username Format | Example |
|----------|-----------------|---------|
| PostgreSQL | `{app}-sa@{project}.iam` | `my-app-sa@apps-platform-prod.iam` |
| MySQL | `{app}-sa` | `my-app-sa` |

