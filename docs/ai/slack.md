# Slack Bot Development

Build Slack bots using **slacklib**, our Go library that handles authentication, event routing, and message handling.

## Quick Start

```bash
apps-platform app init --template slack-go  # Create from template
cd my-slack-bot
make deploy                                  # Deploy and auto-provision Slack app
```

After deploying:
1. Go to https://apps.applied.dev/apps/my-apps
2. Click "Install to Slack" on your app
3. Wait for approval in #eng-apps-platform-v2-slack-approvals
4. Invite your bot to a channel: `/invite @my-slack-bot`

## project.toml for Slack

```toml
name = "my-slack-bot"
enable_slack = true

[metadata]
owner = "you@applied.co"
```

## slacklib Setup

```go
import "go.apps.applied.dev/lib/slacklib"

func main() {
    bot := slacklib.New(slacklib.Config{})

    // Register handlers (see below)

    r := gin.Default()
    bot.RegisterRoutes(r.Group("/slack"))
    r.Run(":8080")
}
```

## Event Handlers

### @Mentions

```go
bot.OnMention(func(ctx *slacklib.MentionContext) {
    // ctx.UserID - who mentioned the bot
    // ctx.ChannelID - where
    // ctx.Text - message (mention stripped)
    ctx.Reply("Hello!")
})
```

### Direct Messages

```go
bot.OnDM(func(ctx *slacklib.DMContext) {
    ctx.Reply("Got your DM!")
})
```

### All Messages

```go
bot.OnMessage(func(ctx *slacklib.MessageContext) {
    if ctx.IsDM() {
        ctx.Reply("This is a DM")
    } else {
        ctx.ReplyInThread("Replying in thread")
    }
})
```

### Slash Commands

```go
bot.Command("/hello", func(ctx *slacklib.CommandContext) {
    ctx.Reply("Hi!")           // Ephemeral (only user sees)
    ctx.ReplyInChannel("Hi!")  // Public (everyone sees)
})
```

## Sending Messages

```go
// Simple message
bot.SendMessage(ctx, channelID, "Hello")

// In a thread
bot.SendMessageInThread(ctx, channelID, "Reply", threadTS)

// Direct message to user
bot.SendDM(ctx, userID, "Private message")

// Ephemeral (only one user sees)
bot.SendEphemeral(ctx, channelID, userID, "Only you see this")
```

## Buttons and Actions

```go
// Send message with button
blocks := slacklib.NewBlocks().
    AddSection("Do you approve?").
    AddButton("approve-btn", "Approve", "approved").
    Build()

bot.SendMessageWithBlocks(ctx, channelID, blocks, nil)

// Handle button click
bot.Action("approve-btn", func(ctx *slacklib.ActionContext) {
    ctx.UpdateMessage("Approved!")
})
```

## Modals

```go
// Open modal from slash command
bot.Command("/feedback", func(ctx *slacklib.CommandContext) {
    modal := slacklib.NewModal("feedback-form", "Submit Feedback").
        WithSubmitText("Send").
        AddTextInput("name-block", "name", "Your Name", "Enter name").
        AddTextArea("feedback-block", "feedback", "Feedback", "What's on your mind?").
        AddSelect("type-block", "type", "Type", "Select...", []slacklib.Option{
            {Text: "Bug", Value: "bug"},
            {Text: "Feature", Value: "feature"},
        })

    ctx.OpenModal(modal)
})

// Handle submission
bot.ViewSubmission("feedback-form", func(ctx *slacklib.ViewContext) {
    name := ctx.GetValue("name-block", "name")
    feedback := ctx.GetValue("feedback-block", "feedback")
    ctx.Reply("Thanks " + name + "!")
})
```

### Modal Input Types

```go
modal.AddTextInput(blockID, actionID, label, placeholder)
modal.AddTextArea(blockID, actionID, label, placeholder)
modal.AddSelect(blockID, actionID, label, placeholder, options)
modal.AddRadioButtons(blockID, actionID, label, options)
modal.AddCheckboxes(blockID, actionID, label, options)
modal.AddUserSelect(blockID, actionID, label, placeholder)
modal.AddChannelSelect(blockID, actionID, label, placeholder)
modal.AddDatePicker(blockID, actionID, label, placeholder)
modal.AddSection(markdownText)
modal.AddDivider()
```

## Context Fields

### MentionContext
- `UserID` - who mentioned the bot
- `ChannelID` - where
- `Text` - message (mention stripped)
- `ThreadTS` - thread timestamp (if in thread)

### CommandContext
- `Command` - command name (e.g., "/hello")
- `Text` - text after the command
- `UserID` - who invoked it
- `ChannelID` - where invoked
- `TriggerID` - for opening modals

### ActionContext
- `ActionID` - the action identifier
- `Value` - the action value
- `UserID` - who triggered it
- `MessageTS` - original message timestamp

### ViewContext
- `CallbackID` - modal callback ID
- `UserID` - who submitted
- `GetValue(blockID, actionID)` - get input value
- `GetSelectedOptions(blockID, actionID)` - get multi-select values

## Advanced: Direct slack-go Access

For operations not covered by slacklib:

```go
client, err := bot.Client()  // Returns *slack.Client

// List channels
channels, _, _ := client.GetConversationsContext(ctx, &slack.GetConversationsParameters{
    Types: []string{"public_channel"},
})

// Add reaction
client.AddReactionContext(ctx, "thumbsup", slack.ItemRef{
    Channel:   channelID,
    Timestamp: messageTS,
})
```

## Architecture

```
Slack API → Slack Proxy (edge) → Your App (Cloud Run)
```

- **Slack Proxy** handles signature verification—your app doesn't need to
- **Tokens** are automatically loaded from Secret Manager
- **Applied Bots workspace** is separate from applied-home for security

## Important Notes

1. **3-second timeout**: Slack requires responses within 3 seconds. For longer operations, acknowledge immediately and reply asynchronously with `bot.SendMessage`

2. **Bot messages filtered**: Messages from bots are automatically ignored to prevent loops

3. **Channel access**: Bots operate in the Applied Bots workspace. Only non-confidential channels are available. Request channel access in #eng-apps-platform-v2-slack-approvals

## Troubleshooting

### Bot not responding
1. Verify routes are registered: `bot.RegisterRoutes(r.Group("/slack"))`
2. Check handlers complete within 3 seconds
3. Ensure bot is invited to the channel: `/invite @your-bot`

### Token errors
1. Verify `enable_slack = true` in project.toml
2. Redeploy your app
3. Check deployment logs for errors

### Need more scopes?
Request additional permissions in #eng-apps-platform-v2-slack-approvals
