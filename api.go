package main

import (
	"log"
	"net/http"
	"strings"

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

func registerAPIRoutes(r *gin.Engine, bot *slacklib.Bot, anaheimClient *anaheim.Client) {
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

	// Daily Update routes
	api.POST("/trigger-update", handleTriggerUpdate(bot))

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

type triggerUpdateRequest struct {
	JiraKey string `json:"jira_key" binding:"required"`
	Channel string `json:"channel" binding:"required"`
}

func handleTriggerUpdate(bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req triggerUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[API] POST /api/trigger-update key=%s channel=%s", req.JiraKey, req.Channel)

		token := c.GetHeader("X-Request-Token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Jira not connected — please connect Jira via the integration bar first"})
			return
		}

		if err := runUpdate(c.Request.Context(), token, req.JiraKey, req.Channel, bot); err != nil {
			log.Printf("[API] trigger-update error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
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
