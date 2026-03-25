package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"slack-go-hackathon/eventlog"

	"go.apps.applied.dev/lib/anaheim"

	"bufio"

	"github.com/gin-gonic/gin"
	"go.apps.applied.dev/lib/cloudlogger"
	"go.apps.applied.dev/lib/slacklib"
	"go.uber.org/zap"
)

const (
	// Secret names for Google Cloud Secret Manager
	anaheimSecretName = "shared-anaheim-service-account-key"
)

//go:generate sh -c "cd frontend && npm install && npm run build"

//go:embed frontend/dist
var frontendFS embed.FS

func main() {
	// Load .env file for local development
	loadDotEnv()

	logger := cloudlogger.New()
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	bot := slacklib.New(slacklib.Config{
		Logger: logger,
	})

	// Initialize Anaheim client from Google Secret Manager
	os.Setenv("ANAHEIM_SECRET_NAME", anaheimSecretName)
	anaheimClient, err := anaheim.New(context.Background())
	if err != nil {
		logger.Warn("failed to initialize anaheim client",
			zap.Error(err),
			zap.String("hint", fmt.Sprintf("Ensure %s secret exists in Secret Manager", anaheimSecretName)),
		)
	} else {
		logger.Info("anaheim client initialized successfully")
	}

	// Initialize MySQL
	db, err := initMySQL(context.Background())
	if err != nil {
		logger.Warn("failed to initialize MySQL — database features disabled",
			zap.Error(err),
		)
		db = nil
	} else {
		if err := migrateMySQL(db); err != nil {
			logger.Error("MySQL migration failed", zap.Error(err))
		}
		defer db.Close()
	}

	registerSlackHandlers(bot)

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	bot.RegisterRoutes(r.Group("/slack"))
	registerAPIRoutes(r, bot, anaheimClient, db)

	// Serve embedded frontend
	if os.Getenv("DEV") != "true" {
		distFS, err := fs.Sub(frontendFS, "frontend/dist")
		if err != nil {
			zap.L().Fatal("failed to load frontend: %v", zap.Error(err))
		}

		serveIndex := func(c *gin.Context) {
			data, _ := fs.ReadFile(distFS, "index.html")
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		}

		r.GET("/", serveIndex)
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if len(path) > 1 {
				if f, err := distFS.Open(path[1:]); err == nil {
					f.Close()
					c.FileFromFS(path[1:], http.FS(distFS))
					return
				}
			}
			serveIndex(c)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	initIDTokenSource()
	zap.L().Info("server starting", zap.String("port", port))
	if err := r.Run(":" + port); err != nil {
		zap.L().Fatal("failed to start server: %v", zap.Error(err))
	}
}

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func feedbackModal() *slacklib.Modal {
	return slacklib.NewModal("feedback_form", "Submit Feedback").
		WithSubmitText("Submit").
		AddSelect("category_block", "category_select", "Category", "Select category", []slacklib.SelectOption{
			{Text: "Bug Report", Value: "bug"},
			{Text: "Feature Request", Value: "feature"},
			{Text: "Other", Value: "other"},
		}).
		AddTextArea("description_block", "description_input", "Description", "Describe your feedback...")
}

func registerSlackHandlers(bot *slacklib.Bot) {
	// Handle @mentions
	bot.OnMention(func(ctx *slacklib.MentionContext) {
		eventlog.Add("app_mention", ctx.UserID, ctx.ChannelID, ctx.Text)

		blocks := slacklib.NewBlocks().
			AddSection(fmt.Sprintf("Hello <@%s>! I'm a Go Slack bot demo.", ctx.UserID)).
			AddButton("open_feedback_form", "Submit Feedback", "click").
			Build()

		// Reply in thread (use existing thread or start new thread from the mention)
		threadTS := ctx.ThreadTS
		if threadTS == "" {
			threadTS = ctx.EventTS
		}

		if _, err := bot.SendMessageWithBlocksInThread(ctx.Context(), ctx.ChannelID, blocks, threadTS); err != nil {
			zap.L().Error("failed to respond to mention: %v", zap.Error(err))
		}
	})

	// Handle DMs
	bot.OnDM(func(ctx *slacklib.DMContext) {
		eventlog.Add("dm_received", ctx.UserID, ctx.ChannelID, ctx.Text)

		response := fmt.Sprintf("Hi! You said: %q", ctx.Text)
		if strings.ToLower(ctx.Text) == "ping" {
			response = "Pong!"
		}

		if err := ctx.Reply(response); err != nil {
			zap.L().Error("failed to respond to DM: %v", zap.Error(err))
		}
	})

	// Handle /feedback command
	bot.Command("/feedback", func(ctx *slacklib.CommandContext) {
		eventlog.Add("slash_command", ctx.UserID, ctx.ChannelID, "/feedback")
		if err := ctx.OpenModal(feedbackModal()); err != nil {
			zap.L().Error("failed to open feedback modal: %v", zap.Error(err))
		}
	})

	// Handle feedback form submission
	bot.ViewSubmission("feedback_form", func(ctx *slacklib.ViewContext) {
		category := ctx.GetValue("category_block", "category_select")
		description := ctx.GetValue("description_block", "description_input")

		eventlog.Add("feedback_submitted", ctx.UserID, "", fmt.Sprintf("[%s] %s", category, description))

		zap.L().Info("feedback received",
			zap.String("user", ctx.UserID),
			zap.String("category", category),
			zap.String("description", description),
		)

		if err := ctx.Reply(fmt.Sprintf("Thanks for your feedback!\n*Category:* %s\n*Description:* %s", category, description)); err != nil {
			zap.L().Error("failed to send feedback confirmation: %v", zap.Error(err))
		}
	})

	// Handle button clicks to open feedback form
	bot.Action("open_feedback_form", func(ctx *slacklib.ActionContext) {
		eventlog.Add("form_opened", ctx.UserID, ctx.ChannelID, "Feedback form opened")
		if err := ctx.OpenModal(feedbackModal()); err != nil {
			zap.L().Error("failed to open feedback modal from button: %v", zap.Error(err))
		}
	})
}
