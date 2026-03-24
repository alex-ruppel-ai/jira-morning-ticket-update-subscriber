# Troubleshooting

## Deployment Issues

### Deployment Fails

1. **Check `project.toml`** exists and has valid `name` and `owner`
2. **Authenticate**: Run `apps-platform auth login`
3. **Service name**: Must be lowercase with hyphens only (no underscores)

```bash
# Valid
name = "my-app"
name = "cloud-run-todo"

# Invalid
name = "My_App"
name = "myApp"
```

### Build Fails

1. **Go apps**: Ensure `go.mod` and `go.sum` are committed
2. **Frontend**: Run `npm install` and `npm run build` locally first
3. **Check logs**: `apps-platform app logs --service my-app`

## Database Issues

### Connection Refused

1. **Verify `enable_postgres = true`** in `project.toml`
2. **Check environment variables** are being read correctly
3. **Redeploy** after enabling database: `apps-platform app deploy`

### Permission Denied

1. **Schema access**: Ensure you're using your app's schema
2. **Set search path**:
   ```go
   schema := strings.ReplaceAll(os.Getenv("K_SERVICE"), "-", "_")
   pool.Exec(ctx, "SET search_path TO "+schema)
   ```

### Schema/Table Not Found

1. Schema name = app name with hyphens → underscores
2. `my-app` uses schema `my_app`
3. Run: `SET search_path TO my_app;`

## Local Development Issues

### Tunnel Won't Start

1. **Authenticate**: `apps-platform auth login`
2. **Check permissions**: You may need Token Creator role
   ```bash
   gcloud iam service-accounts add-iam-policy-binding \
       my-app-sa@PROJECT_ID.iam.gserviceaccount.com \
       --member="user:yourname@applied.co" \
       --role="roles/iam.serviceAccountTokenCreator" \
       --project=PROJECT_ID
   ```

### Password Authentication Failed

The tunnel uses IAM auth (no password). Use correct username:
- PostgreSQL: `{app}-sa@{project}.iam`
- MySQL: `{app}-sa`

### Connection Refused on localhost:5432

1. Make sure tunnel is running in another terminal
2. Wait for `✓ Cloud SQL proxy ready on localhost:5432`
3. Check tunnel output for errors

## Frontend Issues

### Frontend Not Loading

1. **Go+React**: Ensure `go generate` runs before deploy
2. **Check embed path**: `//go:embed frontend/dist`
3. **Build frontend**: `cd frontend && npm run build`

### Assets 404

1. Check `NoRoute` handler serves from correct path
2. Verify `fs.Sub(frontendFS, "frontend/dist")` matches embed path

## Secret Manager Issues

### Access Denied

1. Verify `enable_secrets = true` in `project.toml`
2. Secret must be prefixed with app name: `my-app-api-key`
3. Redeploy after enabling secrets

### Secret Not Found

1. Check secret exists in GCP Console
2. Verify naming: `{app-name}-{secret-name}`
3. Check `PROJECT_ID` environment variable is set

## Getting Help

- **Slack**: [#eng-apps-platform-v2](https://grid-appliedint.enterprise.slack.com/archives/C0A2586929Z)
- **Docs**: https://apps.applied.dev
- **Examples**: https://github.com/Applied-Shared/apps-platform-example-apps

