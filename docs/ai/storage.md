# Storage: Secrets and Filestore

## Secret Manager

When `enable_secrets = true`, your app can read secrets prefixed with your app name.

### Creating Secrets

Use Google Cloud Console:
`https://console.cloud.google.com/security/secret-manager?project=YOUR_PROJECT_ID`

**Secret naming:** `{app-name}-{secret-name}` (e.g., `my-app-api-key`)

### Go Access

```go
import (
    "context"
    "fmt"
    "os"

    secretmanager "cloud.google.com/go/secretmanager/apiv1"
    "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

func getSecret(ctx context.Context, secretName string) (string, error) {
    client, err := secretmanager.NewClient(ctx)
    if err != nil {
        return "", err
    }
    defer client.Close()

    // Prefix with service name
    serviceName := os.Getenv("K_SERVICE")
    projectID := os.Getenv("PROJECT_ID")
    fullSecretName := fmt.Sprintf("%s-%s", serviceName, secretName)

    secretPath := fmt.Sprintf("projects/%s/secrets/%s/versions/latest",
        projectID, fullSecretName)

    result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
        Name: secretPath,
    })
    if err != nil {
        return "", err
    }

    return string(result.Payload.Data), nil
}

// Usage:
apiKey, err := getSecret(ctx, "api-key")  // Reads {app-name}-api-key
```

### Python Access

```python
import os
from google.cloud import secretmanager

def get_secret(secret_name: str) -> str:
    client = secretmanager.SecretManagerServiceClient()

    service_name = os.environ['K_SERVICE']
    project_id = os.environ['PROJECT_ID']
    full_secret_name = f"{service_name}-{secret_name}"

    name = f"projects/{project_id}/secrets/{full_secret_name}/versions/latest"
    response = client.access_secret_version(request={"name": name})

    return response.payload.data.decode('UTF-8')

# Usage:
api_key = get_secret("api-key")  # Reads {app-name}-api-key
```

### Permissions

- Read access to secrets prefixed with your service name
- Cannot access secrets without the prefix
- Cannot create or modify secrets (read-only)

## Filestore (NFS)

When `enable_filestore = true`, persistent storage is mounted at `/mnt/data`.

### Usage

```go
import (
    "os"
    "path/filepath"
)

mountPath := "/mnt/data"

// Check if filestore is mounted
if _, err := os.Stat(mountPath); os.IsNotExist(err) {
    // Not mounted (enable_filestore not set)
    panic("filestore not mounted")
}

// Write file
testFile := filepath.Join(mountPath, "data.txt")
err := os.WriteFile(testFile, []byte("content"), 0644)

// Read file
content, err := os.ReadFile(testFile)

// List files
files, err := os.ReadDir(mountPath)
for _, f := range files {
    info, _ := f.Info()
    fmt.Printf("%s (%d bytes)\n", f.Name(), info.Size())
}
```

### Python

```python
# Write file
with open('/mnt/data/user-uploads/file.txt', 'w') as f:
    f.write('Hello, persistent storage!')

# Read file
with open('/mnt/data/user-uploads/file.txt', 'r') as f:
    content = f.read()
```

### Common Use Cases

- User file uploads
- Shared cache across instances
- Generated reports and exports
- Application logs and data files

### Key Points

- **Mount Point:** `/mnt/data`
- **Persistence:** Data survives deployments and container restarts
- **Isolation:** Each app has its own directory (can't access other apps' files)

