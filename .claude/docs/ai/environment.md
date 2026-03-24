# Environment Variables

## Automatically Set by Apps Platform

These are available in your deployed app without any configuration:

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | Port to listen on | `8080` |
| `K_SERVICE` | Your service name | `my-app` |
| `K_REVISION` | Current revision | `my-app-00042` |
| `K_CONFIGURATION` | Configuration name | `my-app` |
| `PROJECT_ID` | GCP project ID | `apps-platform-prod` |

## Database (when `enable_postgres = true`)

| Variable | Description | Example |
|----------|-------------|---------|
| `INSTANCE_CONNECTION_NAME` | Cloud SQL connection | `project:region:instance` |
| `DB_USER` | IAM database user | `my-app-sa@project.iam` |
| `DB_NAME` | Database name | `postgres` |

## MySQL (when `enable_mysql = true`)

| Variable | Description | Example |
|----------|-------------|---------|
| `MYSQL_INSTANCE_CONNECTION_NAME` | Cloud SQL connection | `project:region:instance` |
| `MYSQL_DB_USER` | IAM database user | `my-app-sa` |
| `MYSQL_DB_NAME` | Database name | `my_app` |

## Custom Environment Variables

Set in `project.toml`:

```toml
[cloudrun.env_vars]
LOG_LEVEL = "info"
MY_CONFIG = "value"
FEATURE_FLAG = "enabled"
```

## Local Development

When running locally (no `INSTANCE_CONNECTION_NAME`), set these manually:

```bash
export DB_USER="my-app-sa@apps-platform-prod.iam"
export DB_NAME="postgres"
export K_SERVICE="my-app"
export PORT="8080"
```

## Detecting Environment

```go
// Check if running in Cloud Run
if os.Getenv("K_SERVICE") != "" {
    // Running in Cloud Run
} else {
    // Running locally
}

// Check if running locally with tunnel
if os.Getenv("INSTANCE_CONNECTION_NAME") == "" {
    // Local mode - connect to localhost
} else {
    // Production mode - use Cloud SQL connector
}
```

