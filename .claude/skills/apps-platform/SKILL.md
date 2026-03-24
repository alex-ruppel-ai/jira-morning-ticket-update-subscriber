---
name: apps-platform
description: Use the Apps Platform V2 CLI to deploy, manage, debug, and configure applications on Applied Intuition's internal Cloud Run infrastructure
---

# Apps Platform CLI

Apps Platform V2 (APV2) is Applied Intuition's internal infrastructure for deploying web applications to Google Cloud Run. The CLI is `apps-platform`.

## Available Commands

```
apps-platform app tail      # view or stream logs (alias: logs)
apps-platform app config    # manage environment profiles (staging/production)
apps-platform app secret    # manage secrets (set, get, list, delete)
apps-platform app apikey    # manage API keys (create, list, delete)
apps-platform app connect-db  # open a secure tunnel to the app's database
apps-platform app schedule  # manage Cloud Scheduler jobs
apps-platform app cloud-task  # manage Cloud Tasks workers
apps-platform update        # update the CLI
```

Run any command with `--help` to see flags and examples.
