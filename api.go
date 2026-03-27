package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"slack-go-hackathon/eventlog"

	"go.apps.applied.dev/lib/anaheim"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"go.apps.applied.dev/lib/slacklib"
	"go.uber.org/zap"
)

// errorHandler is middleware that handles errors added via c.Error()
func errorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			zap.L().Error("api error: %v", zap.Error(err), zap.String("path", c.Request.URL.Path))

			status := c.Writer.Status()
			if status == http.StatusOK {
				status = http.StatusInternalServerError
			}
			c.JSON(status, gin.H{"success": false, "error": err.Error()})
		}
	}
}

func registerAPIRoutes(r *gin.Engine, bot *slacklib.Bot, anaheimClient *anaheim.Client, db *sql.DB) {
	api := r.Group("/api")
	api.Use(errorHandler())

	api.POST("/send-message", handleSendMessage(bot))
	api.POST("/send-message-with-button", handleSendMessageWithButton(bot))
	api.POST("/send-dm", handleSendDM(bot))
	api.GET("/members", handleGetMembers(bot))
	api.GET("/user", handleGetUser(bot))
	api.GET("/events", handleGetEvents())
	api.GET("/feedback", handleGetFeedback())

	// Anaheim API endpoints (if client is initialized)
	if anaheimClient != nil {
		api.GET("/anaheim/user/:email", handleAnaheimGetUser(anaheimClient))
		api.POST("/anaheim/users", handleAnaheimSearchUsers(anaheimClient))
	}

	// Daily Update routes (requires MySQL)
	if db != nil {
		api.GET("/tickets", handleListTickets(db))
		api.POST("/tickets", handleAddTicket(db))
		api.DELETE("/tickets/:key", handleRemoveTicket(db))
		api.GET("/update-config", handleGetUpdateConfig(db))
		api.PUT("/update-config", handleSaveUpdateConfig(db, bot))
		api.POST("/trigger-update", handleTriggerUpdate(db, bot))
	}

	// Cron endpoint — outside /api group so Cloud Scheduler can reach it directly
	if db != nil {
		r.POST("/internal/jobs/check-daily-update", handleCheckDailyUpdate(db, bot))
	}

	// Data API proxy routes
	registerDataAPIRoutes(api)
}

type sendMessageRequest struct {
	Channel  string `json:"channel" binding:"required"`
	Text     string `json:"text" binding:"required"`
	ThreadTS string `json:"thread_ts,omitempty"`
}

func handleSendMessage(bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sendMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		var result *slacklib.MessageResult
		var err error
		if req.ThreadTS != "" {
			result, err = bot.SendMessageInThread(c, req.Channel, req.Text, req.ThreadTS)
		} else {
			result, err = bot.SendMessage(c, req.Channel, req.Text)
		}
		if err != nil {
			c.Error(err)
			return
		}

		eventlog.Add("message_sent", "", req.Channel, req.Text)
		c.JSON(http.StatusOK, gin.H{"success": true, "channel": result.ChannelID, "timestamp": result.Timestamp})
	}
}

type sendMessageWithButtonRequest struct {
	Channel    string `json:"channel" binding:"required"`
	Text       string `json:"text" binding:"required"`
	ButtonText string `json:"button_text"`
	ActionID   string `json:"action_id"`
}

func handleSendMessageWithButton(bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sendMessageWithButtonRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		buttonText, actionID := req.ButtonText, req.ActionID
		if buttonText == "" {
			buttonText = "Click"
		}
		if actionID == "" {
			actionID = "button_click"
		}

		blocks := []slack.Block{
			slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, req.Text, false, false), nil, nil),
			slack.NewActionBlock("", slack.NewButtonBlockElement(actionID, "click", slack.NewTextBlockObject(slack.PlainTextType, buttonText, false, false))),
		}

		result, err := bot.SendMessageWithBlocks(c, req.Channel, blocks)
		if err != nil {
			c.Error(err)
			return
		}

		eventlog.Add("message_sent", "", req.Channel, req.Text+" [with button]")
		c.JSON(http.StatusOK, gin.H{"success": true, "channel": result.ChannelID, "timestamp": result.Timestamp})
	}
}

type sendDMRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Text   string `json:"text" binding:"required"`
}

func handleSendDM(bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req sendDMRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		result, err := bot.SendDM(c, req.UserID, req.Text)
		if err != nil {
			c.Error(err)
			return
		}

		eventlog.Add("dm_sent", req.UserID, result.ChannelID, req.Text)
		c.JSON(http.StatusOK, gin.H{"success": true, "channel": result.ChannelID})
	}
}

type getMembersRequest struct {
	Channel string `form:"channel" binding:"required"`
}

func handleGetMembers(bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req getMembersRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		members, err := bot.GetChannelMembers(c, req.Channel)
		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "members": members})
	}
}

type getUserRequest struct {
	UserID string `form:"user_id" binding:"required"`
}

func handleGetUser(bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req getUserRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		user, err := bot.GetUserInfo(c, req.UserID)
		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user": gin.H{
				"id":        user.ID,
				"name":      user.Name,
				"real_name": user.RealName,
				"email":     user.Profile.Email,
				"title":     user.Profile.Title,
				"image":     user.Profile.Image72,
			},
		})
	}
}

func handleGetEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		events := eventlog.GetRecent()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"events":  events,
		})
	}
}

func handleGetFeedback() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Feedback is stored in eventlog as "feedback_submitted" events
		// Filter those out and return them
		allEvents := eventlog.GetRecent()
		var submissions []gin.H

		for _, e := range allEvents {
			if e.Type == "feedback_submitted" {
				submissions = append(submissions, gin.H{
					"id":           e.ID,
					"user_id":      e.User,
					"description":  e.Text,
					"category":     "feedback",
					"urgency":      "medium",
					"submitted_at": e.Timestamp,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"submissions": submissions,
		})
	}
}

// --- Daily Update handlers ---

func handleListTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] GET /api/tickets")
		tickets, err := dbGetTrackedTickets(db)
		if err != nil {
			log.Printf("[API] error listing tickets: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if tickets == nil {
			tickets = []TrackedTicket{}
		}
		c.JSON(http.StatusOK, gin.H{"tickets": tickets})
	}
}

func handleAddTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			JiraKey string `json:"jira_key" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		key := strings.ToUpper(strings.TrimSpace(body.JiraKey))
		log.Printf("[API] POST /api/tickets key=%s", key)

		token := c.GetHeader("X-Request-Token")
		summary := key
		if token != "" {
			if issue, err := fetchParentIssue(token, key); err == nil {
				summary = issue.Fields.Summary
			} else {
				log.Printf("[API] could not fetch Jira summary for %s: %v", key, err)
			}
		}

		if err := dbAddTrackedTicket(db, key, summary); err != nil {
			log.Printf("[API] error adding ticket %s: %v", key, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Persist token so cron can use it server-side
		if token != "" {
			if cfg, err := dbGetUpdateConfig(db); err == nil {
				cfg.RequestToken = token
				if err := dbSaveUpdateConfig(db, cfg); err != nil {
					log.Printf("[API] warning: could not persist request token: %v", err)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "jira_key": key, "summary": summary})
	}
}

func handleRemoveTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.ToUpper(c.Param("key"))
		log.Printf("[API] DELETE /api/tickets/%s", key)
		if err := dbRemoveTrackedTicket(db, key); err != nil {
			log.Printf("[API] error removing ticket %s: %v", key, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleGetUpdateConfig(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] GET /api/update-config")
		cfg, err := dbGetUpdateConfig(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Never expose the stored token to the frontend
		c.JSON(http.StatusOK, gin.H{
			"post_time": cfg.PostTime,
			"timezone":  cfg.Timezone,
			"channel":   cfg.Channel,
		})
	}
}

func handleSaveUpdateConfig(db *sql.DB, bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			PostTime string `json:"post_time" binding:"required"`
			Timezone string `json:"timezone" binding:"required"`
			Channel  string `json:"channel" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[API] PUT /api/update-config time=%s tz=%s channel=%s", body.PostTime, body.Timezone, body.Channel)

		existing, _ := dbGetUpdateConfig(db)
		token := c.GetHeader("X-Request-Token")
		log.Printf("[API] save-config: X-Request-Token present=%v", token != "")
		if token == "" {
			token = existing.RequestToken
			log.Printf("[API] save-config: falling back to stored token (len=%d)", len(token))
		}

		cfg := UpdateConfig{
			PostTime:     body.PostTime,
			Timezone:     body.Timezone,
			Channel:      body.Channel,
			RequestToken: token,
		}
		if err := dbSaveUpdateConfig(db, cfg); err != nil {
			log.Printf("[API] error saving config: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Auto-trigger if today's update hasn't been posted yet
		alreadyPosted, _ := dbHasPostedToday(db, cfg.Timezone)
		if !alreadyPosted {
			log.Printf("[API] no post yet today — auto-triggering daily update after save")
			go func() {
				slackTS, err := runDailyUpdate(db, bot)
				if err != nil {
					log.Printf("[API] auto-trigger error: %v", err)
					return
				}
				if err := dbRecordDailyPost(db, slackTS, cfg.Timezone); err != nil {
					log.Printf("[API] warning: could not record auto-triggered post: %v", err)
				}
			}()
			c.JSON(http.StatusOK, gin.H{"success": true, "triggered": true})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleTriggerUpdate(db *sql.DB, bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] POST /api/trigger-update (manual)")

		// Refresh stored token from this request if present
		token := c.GetHeader("X-Request-Token")
		if token != "" {
			if cfg, err := dbGetUpdateConfig(db); err == nil {
				cfg.RequestToken = token
				dbSaveUpdateConfig(db, cfg)
			}
		}

		slackTS, err := runDailyUpdate(db, bot)
		if err != nil {
			log.Printf("[API] trigger-update error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "slack_ts": slackTS})
	}
}

func handleCheckDailyUpdate(db *sql.DB, bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[CronCheck] /internal/jobs/check-daily-update fired")

		cfg, err := dbGetUpdateConfig(db)
		if err != nil {
			log.Printf("[CronCheck] could not load config: %v", err)
			c.JSON(http.StatusOK, gin.H{"skipped": "config error"})
			return
		}

		if cfg.Channel == "" || cfg.RequestToken == "" {
			log.Printf("[CronCheck] skipping: channel=%q token_set=%v", cfg.Channel, cfg.RequestToken != "")
			c.JSON(http.StatusOK, gin.H{"skipped": "not configured"})
			return
		}

		alreadyPosted, err := dbHasPostedToday(db, cfg.Timezone)
		if err != nil {
			log.Printf("[CronCheck] could not check daily log: %v", err)
			c.JSON(http.StatusOK, gin.H{"skipped": "log check error"})
			return
		}
		if alreadyPosted {
			log.Printf("[CronCheck] already posted today — skipping")
			c.JSON(http.StatusOK, gin.H{"skipped": "already posted today"})
			return
		}

		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		// Parse configured post time as today's time in the configured timezone
		var postHour, postMin int
		fmt.Sscanf(cfg.PostTime, "%d:%d", &postHour, &postMin)
		configuredAt := time.Date(now.Year(), now.Month(), now.Day(), postHour, postMin, 0, 0, loc)
		// Fire if the configured time falls within the last 10 minutes
		elapsed := now.Sub(configuredAt)
		if elapsed < 0 || elapsed > 10*time.Minute {
			log.Printf("[CronCheck] outside 10min window (now=%s configured=%s elapsed=%s) — skipping", now.Format("15:04"), cfg.PostTime, elapsed.Round(time.Second))
			c.JSON(http.StatusOK, gin.H{"skipped": "not time yet"})
			return
		}

		log.Printf("[CronCheck] within 10min window of %s (elapsed %s) — running daily update", cfg.PostTime, elapsed.Round(time.Second))
		slackTS, err := runDailyUpdate(db, bot)
		if err != nil {
			log.Printf("[CronCheck] runDailyUpdate error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := dbRecordDailyPost(db, slackTS, cfg.Timezone); err != nil {
			log.Printf("[CronCheck] warning: could not record daily post: %v", err)
		}

		log.Printf("[CronCheck] daily update posted, slack_ts=%s", slackTS)
		c.JSON(http.StatusOK, gin.H{"success": true, "slack_ts": slackTS})
	}
}

// Anaheim API handlers

func handleAnaheimGetUser(client *anaheim.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Param("email")

		user, err := client.GetUserByEmail(c, email)
		if err != nil {
			if apiErr, ok := err.(*anaheim.APIError); ok {
				c.AbortWithError(apiErr.StatusCode, err)
				return
			}
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user":    user,
		})
	}
}

type searchUsersRequest struct {
	Query string `json:"query" binding:"required"`
}

func handleAnaheimSearchUsers(client *anaheim.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req searchUsersRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		query := strings.ToLower(strings.TrimSpace(req.Query))
		if query == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"users":   []anaheim.Employee{},
			})
			return
		}

		var users []anaheim.Employee
		var err error

		// Fast path: if query contains @, treat as email search
		if strings.Contains(query, "@") {
			users, err = client.GetUsers(c, anaheim.UserFilter{
				Emails: []string{query},
			})
		} else {
			// Slow path: fetch all users and filter by name client-side
			// This is necessary because UserFilter doesn't have a Names field
			users, err = client.GetUsers(c, anaheim.UserFilter{})
			if err == nil {
				var filtered []anaheim.Employee
				for _, user := range users {
					firstNameMatch := strings.Contains(strings.ToLower(user.FirstName), query)
					lastNameMatch := strings.Contains(strings.ToLower(user.LastName), query)
					fullNameMatch := strings.Contains(strings.ToLower(user.FirstName+" "+user.LastName), query)

					if firstNameMatch || lastNameMatch || fullNameMatch {
						filtered = append(filtered, user)
						// Limit to first 50 results to avoid huge responses
						if len(filtered) >= 50 {
							break
						}
					}
				}
				users = filtered
			}
		}

		if err != nil {
			if apiErr, ok := err.(*anaheim.APIError); ok {
				c.AbortWithError(apiErr.StatusCode, err)
				return
			}
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"users":   users,
		})
	}
}
