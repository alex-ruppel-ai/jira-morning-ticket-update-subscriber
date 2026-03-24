# Local Development with Production Database

Test your app locally against the production database using `apps-platform connect-db`.

## For AI Assistants (Cursor)

When a user asks to "run my app locally" or "test against the database":

1. **Start the tunnel in a background terminal:**
   ```bash
   apps-platform connect-db
   ```
   Run with `is_background: true`. Wait for `✓ Cloud SQL proxy ready on localhost:5432`.

2. **Start the application in another terminal:**
   ```bash
   DB_USER="{app}-sa@{project}.iam" DB_NAME="postgres" K_SERVICE="{app}" go run main.go
   ```
   Replace `{app}` with the app name from `project.toml` and `{project}` with the GCP project ID.

3. **Tell the user** to visit `http://localhost:8080`

4. **Cleanup is automatic** - when Cursor closes, all terminals terminate.

## Database Usernames

| Database | Username Format | Example |
|----------|-----------------|---------|
| PostgreSQL | `{app}-sa@{project}.iam` | `my-app-sa@apps-platform-prod.iam` |
| MySQL | `{app}-sa` | `my-app-sa` |

## Code Pattern

Detect local vs production by checking `INSTANCE_CONNECTION_NAME`:

```go
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
    dialer, _ := cloudsqlconn.NewDialer(ctx,
        cloudsqlconn.WithIAMAuthN(),
        cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))

    config, _ := pgxpool.ParseConfig(fmt.Sprintf(
        "user=%s dbname=%s sslmode=disable", dbUser, dbName))

    config.ConnConfig.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
        return dialer.Dial(ctx, instanceConn)
    }

    return pgxpool.NewWithConfig(ctx, config)
}
```

## Schema Access

Schema name = app name with hyphens → underscores:
- `my-app` → `my_app`
- `cloud-run-todo` → `cloud_run_todo`

```go
schema := strings.ReplaceAll(os.Getenv("K_SERVICE"), "-", "_")
pool.Exec(ctx, "SET search_path TO "+schema)
```

## Tunnel Commands

```bash
apps-platform connect-db              # Auto-detect from project.toml
apps-platform connect-db --app my-app # Explicit app
apps-platform connect-db --connect    # Open psql/mysql session
apps-platform connect-db --type mysql # Override database type
```

## Security

No passwords involved. The Cloud SQL proxy uses IAM authentication with short-lived OAuth tokens that are automatically refreshed. Your credentials never touch the network.

## Troubleshooting

**Permission denied when impersonating service account:**
```bash
gcloud iam service-accounts add-iam-policy-binding \
    my-app-sa@PROJECT_ID.iam.gserviceaccount.com \
    --member="user:yourname@applied.co" \
    --role="roles/iam.serviceAccountTokenCreator" \
    --project=PROJECT_ID
```

**Connection refused:** Make sure tunnel is running. Check for `✓ Cloud SQL proxy ready`.

**Password authentication failed:** Use correct username format (no password needed).

**Schema not found:** Set search path: `SET search_path TO my_app;`

