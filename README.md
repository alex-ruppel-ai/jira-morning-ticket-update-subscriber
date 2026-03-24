# Slack Go

A Go Slack bot template for building Slack bots on Apps Platform.

## Overview

This project contains:

1. **`slacklib/`** - A reusable library for building Slack bots with a clean, high-level API
2. **`example/`** - A minimal example showing how to use slacklib
3. **Main app** - A full demo app with React frontend showing all Slack capabilities

## Apps Platform Integration

This template integrates with Apps Platform in the following ways:

### Configuration (`project.toml`)

```toml
name = "slack-go"
enable_secrets = true   # Enables Google Cloud Secret Manager
enable_slack = true     # Enables Slack webhook routing via slack-proxy
```

### Slack Proxy

Apps Platform provides a `slack-proxy` service that routes Slack webhooks to your app:

- **Events**: `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/events`
- **Commands**: `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/commands`
- **Interactions**: `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/interactions`

The proxy handles Slack URL verification and routes events to the corresponding `/slack/*` endpoints in your app.

### Cloud Logging

Uses `cloudlogger` from `apps-platform/lib` to integrate with Google Cloud Logging:

```go
logger := cloudlogger.New()
zap.ReplaceGlobals(logger)
```

### Deployment

Deploy using the Apps Platform CLI:

```bash
apps-platform app deploy --no-build
```

## Secrets

This template uses one secret:

| Secret Name | Environment Variable | Description |
|-------------|---------------------|-------------|
| `<app-name>-slack-bot-token` | `SLACK_BOT_TOKEN` | Slack Bot OAuth token (starts with `xoxb-`) |

### Secret Loading Priority

The `slacklib` library loads the bot token in this order:

1. **Explicit config**: `slacklib.Config{BotToken: "xoxb-..."}`
2. **Secret Manager**: `slacklib.Config{SecretName: "my-secret"}` - loads from Google Cloud Secret Manager
3. **Environment variable**: Falls back to `SLACK_BOT_TOKEN` env var

### Local Development

Set the environment variable directly:

```bash
export SLACK_BOT_TOKEN=xoxb-your-token-here
make run
```

### Production Deployment

Create a secret in Google Cloud Secret Manager with the naming convention:

```
<app-name>-slack-bot-token
```

For example, if your app is named `slack-go`, create a secret called `slack-go-slack-bot-token`.

The Apps Platform deployment will automatically make this secret available to your app when `enable_secrets = true` is set in `project.toml`.

## Quick Start with slacklib

```go
package main

import (
    "slack-go-hackathon/slacklib"
    "github.com/gin-gonic/gin"
)

func main() {
    // Create bot (token from SLACK_BOT_TOKEN env or Secret Manager)
    bot, _ := slacklib.New(slacklib.Config{})

    // Handle @mentions
    bot.OnMention(func(ctx *slacklib.MentionContext) {
        ctx.Reply("Hello! You mentioned me.")
    })

    // Handle DMs
    bot.OnDM(func(ctx *slacklib.DMContext) {
        ctx.Reply("Got your message: " + ctx.Text)
    })

    // Slash command with modal form
    bot.Command("/feedback", func(ctx *slacklib.CommandContext) {
        modal := slacklib.NewModal("feedback_form", "Submit Feedback").
            AddSelect("cat", "category", "Category", "Select", []slacklib.SelectOption{
                {Text: "Bug", Value: "bug"},
                {Text: "Feature", Value: "feature"},
            }).
            AddTextArea("desc", "description", "Description", "Details...")
        ctx.OpenModal(modal)
    })

    // Handle form submission
    bot.ViewSubmission("feedback_form", func(ctx *slacklib.ViewContext) {
        category := ctx.GetValue("cat", "category")
        desc := ctx.GetValue("desc", "description")
        ctx.Reply("Thanks! Got: " + category + " - " + desc)
    })

    // Handle button clicks
    bot.Action("my_button", func(ctx *slacklib.ActionContext) {
        ctx.OpenModal(someModal)
    })

    // Set up routes
    r := gin.Default()
    bot.RegisterRoutes(r.Group("/slack"))
    r.Run(":8081")
}
```

## Capabilities

| Feature | slacklib API | Slack Events |
|---------|--------------|--------------|
| App mentions | `bot.OnMention()` | `app_mention` event |
| DM receive | `bot.OnDM()` | `message` event (im) |
| DM send | `bot.SendDM()` | - |
| Channel messages | `bot.SendMessage()` | - |
| Slash commands | `bot.Command()` | Slash command |
| Modal forms | `bot.OpenModal()` | - |
| Form submissions | `bot.ViewSubmission()` | `view_submission` |
| Button clicks | `bot.Action()` | `block_actions` |
| List members | `bot.GetChannelMembers()` | - |

## Setup

### 1. Install dependencies

```bash
make deps
```

### 2. Configure Slack App

Create a Slack app at [api.slack.com/apps](https://api.slack.com/apps) with:

**OAuth Scopes:**
- `app_mentions:read` - Receive @mentions
- `chat:write` - Send messages
- `im:history` - Read DM history
- `im:read` - View DM metadata
- `im:write` - Start DMs
- `channels:read` - View channel info
- `users:read` - View user info
- `commands` - Slash commands

**Event Subscriptions URL:**
```
https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/events
```

**Interactivity URL:**
```
https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/interactions
```

**Slash Commands:**
- `/feedback` → `https://slack-proxy.<env>.apps.applied.dev/webhook/<app-name>/commands`

### 3. Set Bot Token

For local development, create a `.env` file:

```bash
cp .env.example .env
# Edit .env and add your tokens
```

Required variables for `.env`:
```bash
SLACK_BOT_TOKEN=xoxb-your-token
```

Alternatively, export environment variables:
```bash
export SLACK_BOT_TOKEN=xoxb-your-token
```

For deployed apps, create secrets in Secret Manager:
- `<app-name>-slack-bot-token` - Slack bot token

## Running

### Development

```bash
make run
```

This starts:
- Go backend on http://localhost:8081
- React frontend on http://localhost:3000

### Production

```bash
make build
make deploy
```

## API Endpoints

The app exposes both Slack webhook endpoints and REST API:

### Slack Webhooks (used by slack-proxy)

- `POST /slack/events` - Slack Events API
- `POST /slack/commands` - Slash commands
- `POST /slack/interactions` - Buttons, modals

### REST API (for frontend and external apps)

- `POST /api/send-message` - Send channel message
- `POST /api/send-dm` - Send DM to user
- `POST /api/send-message-with-button` - Message with interactive button
- `GET /api/members?channel=C...` - List channel members
- `GET /api/user?user_id=U...` - Get user info
- `GET /api/events` - Get recent events (for dashboard)
- `GET /api/feedback` - Get feedback submissions

## Library Reference

### Creating a Bot

```go
// With explicit token
bot, _ := slacklib.New(slacklib.Config{
    BotToken: "xoxb-...",
})

// From environment (SLACK_BOT_TOKEN)
bot, _ := slacklib.New(slacklib.Config{})

// From Secret Manager
bot, _ := slacklib.New(slacklib.Config{
    SecretName: "my-slack-bot-token",
})
```

### Handler Contexts

Each handler receives a context with convenience methods:

```go
// MentionContext
ctx.Reply(text)           // Reply in thread
ctx.ReplyInChannel(text)  // Reply in channel
ctx.UserID                // Who mentioned
ctx.ChannelID             // Where
ctx.Text                  // Full message text

// DMContext
ctx.Reply(text)           // Reply to DM
ctx.UserID
ctx.Text

// CommandContext
ctx.OpenModal(modal)      // Open a form
ctx.Ack()                 // Acknowledge silently
ctx.Command               // "/feedback"
ctx.Text                  // Arguments after command
ctx.TriggerID             // For opening modals

// ViewContext (form submissions)
ctx.GetValue(blockID, actionID)  // Extract form value
ctx.Reply(text)                  // DM the user
ctx.Values                       // Raw form state

// ActionContext (button clicks)
ctx.OpenModal(modal)
ctx.UpdateMessage(text)   // Edit the message
ctx.ActionID
ctx.Value
```

### Building Modals

```go
modal := slacklib.NewModal("callback_id", "Title").
    WithSubmitText("Submit").
    WithCloseText("Cancel").
    WithMetadata("custom data").
    AddSection("*Instructions:* Fill out this form").
    AddTextInput("name_block", "name", "Name", "Enter your name").
    AddTextArea("desc_block", "desc", "Description", "Details...").
    AddSelect("cat_block", "cat", "Category", "Choose", []slacklib.SelectOption{
        {Text: "Option 1", Value: "1"},
        {Text: "Option 2", Value: "2"},
    }).
    AddRadioButtons("urgency_block", "urgency", "Urgency", []slacklib.SelectOption{
        {Text: "High", Value: "high"},
        {Text: "Low", Value: "low"},
    }).
    AddUserSelect("user_block", "user", "Assign To", "Select user").
    AddChannelSelect("chan_block", "chan", "Channel", "Select channel").
    AddDatePicker("date_block", "date", "Due Date", "Select date").
    AddDivider()
```

### Sending Messages

```go
// Simple message
bot.SendMessage(ctx, channelID, "Hello!")

// Thread reply
bot.SendMessageInThread(ctx, channelID, "Reply", threadTS)

// DM
bot.SendDM(ctx, userID, "Private message")

// With blocks
blocks := slacklib.NewBlocks().
    AddSection("*Hello!* Click below:").
    AddButton("my_action", "Click Me", "value").
    Build()
bot.SendMessageWithBlocks(ctx, channelID, blocks)

// Ephemeral (only visible to one user)
bot.SendEphemeral(ctx, channelID, userID, "Only you see this")
```

## Project Structure

```
slack-go/
├── main.go              # Full demo app entry point
├── handlers/            # Demo app handlers (old style)
├── slackclient/         # Demo app Slack client
├── slacklib/            # Reusable library
│   ├── bot.go           # Main Bot type and setup
│   ├── context.go       # Handler context types
│   ├── handlers.go      # Event routing
│   ├── messaging.go     # Message sending
│   ├── modals.go        # Modal builder
│   └── doc.go           # Package documentation
├── example/             # Minimal example using slacklib
├── frontend/            # React dashboard
└── project.toml         # Apps Platform config
```
