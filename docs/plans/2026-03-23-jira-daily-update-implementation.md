# Jira Daily Update Bot — Implementation Plan

**Goal:** Build a feature that automatically posts a daily Jira status update to a configured Slack channel, with a condensed summary as the main message and a detailed breakdown in the thread.

**Architecture:** A new `db.go` file handles MySQL (3 tables: tracked tickets, schedule config, daily post log). A new `jira_update.go` handles Jira data fetching and Slack message building. `api.go` gains 7 new routes. A React tab (`DailyUpdateTab.tsx`) lets users configure tickets and schedule. An Apps Platform cron fires every minute and posts when the time matches.

---

### Task 1: Create `db.go` — MySQL init, migrations, and data access

**Description**
Create a new Go file that handles all database concerns: connecting to MySQL (locally and in Cloud Run), running schema migrations on startup, and providing typed functions for reading and writing all three tables.

**Files:**
- Create: `db.go`

**Step 1: Verify vendor packages exist**

Run: `ls vendor/github.com/go-sql-driver/mysql && ls vendor/cloud.google.com/go/cloudsqlconn`

Expected: both directories exist. If either is missing, stop — do not modify go.mod.

**Step 2: Create `db.go`**

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	mysqldriver "github.com/go-sql-driver/mysql"
)

// --- Structs ---

type TrackedTicket struct {
	ID      int       `json:"id"`
	JiraKey string    `json:"jira_key"`
	Summary string    `json:"summary"`
	AddedAt time.Time `json:"added_at"`
}

type UpdateConfig struct {
	PostTime    string `json:"post_time"`    // HH:MM, e.g. "09:00"
	Timezone    string `json:"timezone"`     // IANA, e.g. "America/Los_Angeles"
	Channel     string `json:"channel"`      // e.g. "#adp-daily"
	RequestToken string `json:"request_token"` // stored X-Request-Token for server-side Jira calls
}

// --- Init ---

func initMySQL(ctx context.Context) (*sql.DB, error) {
	dbUser := os.Getenv("MYSQL_DB_USER")
	dbName := os.Getenv("MYSQL_DB_NAME")
	if dbUser == "" || dbName == "" {
		return nil, fmt.Errorf("missing MYSQL_DB_USER or MYSQL_DB_NAME env vars")
	}

	instanceConnectionName := os.Getenv("MYSQL_INSTANCE_CONNECTION_NAME")
	if instanceConnectionName == "" {
		// Local development: connect via Cloud SQL proxy (apps-platform app connect-db)
		dsn := fmt.Sprintf("%s@tcp(localhost:3306)/%s?parseTime=true", dbUser, dbName)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("sql.Open (local): %w", err)
		}
		if err := db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("db.Ping (local): %w", err)
		}
		log.Printf("[MySQL] connected locally to %s", dbName)
		return db, nil
	}

	// Cloud Run: use Cloud SQL connector with IAM auth
	dialer, err := cloudsqlconn.NewDialer(ctx,
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()),
	)
	if err != nil {
		return nil, fmt.Errorf("cloudsqlconn.NewDialer: %w", err)
	}

	mysqldriver.RegisterDialContext("cloudsql", func(ctx context.Context, addr string) (net.Conn, error) {
		return dialer.Dial(ctx, instanceConnectionName)
	})

	dsn := fmt.Sprintf("%s@cloudsql(%s)/%s?parseTime=true", dbUser, instanceConnectionName, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open (cloud): %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db.Ping (cloud): %w", err)
	}
	log.Printf("[MySQL] connected via Cloud SQL IAM auth to %s", dbName)
	return db, nil
}

func migrateMySQL(db *sql.DB) error {
	log.Printf("[MySQL] running migrations")
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tracked_tickets (
			id       INT AUTO_INCREMENT PRIMARY KEY,
			jira_key VARCHAR(50) NOT NULL UNIQUE,
			summary  VARCHAR(500),
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create tracked_tickets: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS update_config (
			id            INT AUTO_INCREMENT PRIMARY KEY,
			post_time     VARCHAR(5)   NOT NULL DEFAULT '09:00',
			timezone      VARCHAR(50)  NOT NULL DEFAULT 'America/Los_Angeles',
			channel       VARCHAR(100) NOT NULL DEFAULT '',
			request_token VARCHAR(500) NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("create update_config: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_update_log (
			id        INT AUTO_INCREMENT PRIMARY KEY,
			post_date DATE NOT NULL UNIQUE,
			posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			slack_ts  VARCHAR(100)
		)
	`)
	if err != nil {
		return fmt.Errorf("create daily_update_log: %w", err)
	}

	log.Printf("[MySQL] migrations complete")
	return nil
}

// --- Tracked Tickets ---

func dbGetTrackedTickets(db *sql.DB) ([]TrackedTicket, error) {
	rows, err := db.Query("SELECT id, jira_key, summary, added_at FROM tracked_tickets ORDER BY added_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []TrackedTicket
	for rows.Next() {
		var t TrackedTicket
		if err := rows.Scan(&t.ID, &t.JiraKey, &t.Summary, &t.AddedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

func dbAddTrackedTicket(db *sql.DB, key, summary string) error {
	_, err := db.Exec(
		"INSERT INTO tracked_tickets (jira_key, summary) VALUES (?, ?) ON DUPLICATE KEY UPDATE summary=VALUES(summary)",
		key, summary,
	)
	return err
}

func dbRemoveTrackedTicket(db *sql.DB, key string) error {
	_, err := db.Exec("DELETE FROM tracked_tickets WHERE jira_key = ?", key)
	return err
}

// --- Update Config ---

func dbGetUpdateConfig(db *sql.DB) (UpdateConfig, error) {
	var cfg UpdateConfig
	err := db.QueryRow("SELECT post_time, timezone, channel, request_token FROM update_config LIMIT 1").
		Scan(&cfg.PostTime, &cfg.Timezone, &cfg.Channel, &cfg.RequestToken)
	if err == sql.ErrNoRows {
		// Return defaults if not configured yet
		return UpdateConfig{PostTime: "09:00", Timezone: "America/Los_Angeles"}, nil
	}
	return cfg, err
}

func dbSaveUpdateConfig(db *sql.DB, cfg UpdateConfig) error {
	// Delete all rows then insert one (single-row config table)
	_, err := db.Exec("DELETE FROM update_config")
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO update_config (post_time, timezone, channel, request_token) VALUES (?, ?, ?, ?)",
		cfg.PostTime, cfg.Timezone, cfg.Channel, cfg.RequestToken,
	)
	return err
}

// --- Daily Update Log ---

func dbHasPostedToday(db *sql.DB, tz string) (bool, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc).Format("2006-01-02")
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM daily_update_log WHERE post_date = ?", today).Scan(&count)
	return count > 0, err
}

func dbRecordDailyPost(db *sql.DB, slackTS, tz string) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc).Format("2006-01-02")
	_, err = db.Exec(
		"INSERT INTO daily_update_log (post_date, slack_ts) VALUES (?, ?) ON DUPLICATE KEY UPDATE slack_ts=VALUES(slack_ts)",
		today, slackTS,
	)
	return err
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`

Expected: no errors. If there are import errors for `cloudsqlconn` or `mysql`, verify the vendor directories from Step 1 exist.

---

### Task 2: Wire MySQL into `main.go`

**Description**
Add MySQL initialization and migration calls to `main.go`, and pass the `*sql.DB` handle to `registerAPIRoutes` so all handlers can use it.

**Files:**
- Modify: `main.go`

**Step 1: Import database/sql in main.go**

The import block in `main.go` currently includes `context`. Add `"database/sql"` if it's not already there. (Check first — it may already be imported.)

**Step 2: Add MySQL init after the bot is created**

Find the line `bot := slacklib.New(...)` (around line 42). After the Anaheim client block (around line 56), add:

```go
	// Initialize MySQL
	db, err := initMySQL(context.Background())
	if err != nil {
		logger.Warn("failed to initialize MySQL — database features disabled",
			zap.Error(err),
			zap.String("hint", "Deploy to Cloud Run to use Cloud SQL, or use apps-platform app connect-db locally"),
		)
		db = nil
	} else {
		if err := migrateMySQL(db); err != nil {
			logger.Error("MySQL migration failed", zap.Error(err))
		}
		defer db.Close()
	}
```

**Step 3: Update the registerAPIRoutes call to pass db**

Find:
```go
	registerAPIRoutes(r, bot, anaheimClient)
```

Change to:
```go
	registerAPIRoutes(r, bot, anaheimClient, db)
```

**Step 4: Verify it compiles**

Run: `go build ./...`

Expected: error about `registerAPIRoutes` signature mismatch — that's expected, you'll fix it in Task 4.

---

### Task 3: Create `jira_update.go` — Jira data fetching and Slack message building

**Description**
Create a new Go file that handles all Jira API calls (using a stored token for server-side access) and builds the condensed + detailed Slack message text. Also handles posting to Slack.

**Files:**
- Create: `jira_update.go`

**Step 1: Create the file**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"database/sql"

	"go.apps.applied.dev/lib/slacklib"
)

// --- Jira API structs ---

type jiraIssue struct {
	Key    string      `json:"key"`
	Fields jiraFields  `json:"fields"`
}

type jiraFields struct {
	Summary  string        `json:"summary"`
	Status   jiraStatus    `json:"status"`
	Assignee *jiraUser     `json:"assignee"`
	Updated  string        `json:"updated"` // ISO 8601
	Subtasks []jiraSubtask `json:"subtasks"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraUser struct {
	DisplayName string `json:"displayName"`
}

type jiraSubtask struct {
	Key    string     `json:"key"`
	Fields jiraFields `json:"fields"`
}

type jiraCommentResponse struct {
	Comments []jiraComment `json:"comments"`
}

type jiraComment struct {
	Author  jiraUser        `json:"author"`
	Body    json.RawMessage `json:"body"` // ADF format
	Created string          `json:"created"`
}

type jiraChangelogResponse struct {
	Values []jiraChangelogEntry `json:"values"`
}

type jiraChangelogEntry struct {
	Author  jiraUser       `json:"author"`
	Created string         `json:"created"`
	Items   []jiraChangeItem `json:"items"`
}

type jiraChangeItem struct {
	Field      string `json:"field"`
	FromString string `json:"fromString"`
	ToString   string `json:"toString"`
}

// --- Server-side Jira API call helper ---

// callJira makes a server-side call to the Jira integration using the stored X-Request-Token.
func callJira(token, path string) ([]byte, error) {
	targetURL := dataAPIURL() + "/api/data/jira" + path
	log.Printf("[Jira] GET %s", targetURL)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Request-Token", token)

	// Attach IAM token if available (required in production)
	if iamTokenSource != nil {
		idToken, err := iamTokenSource.Token()
		if err != nil {
			log.Printf("[Jira] warning: failed to mint IAM token: %v", err)
		} else {
			req.Header.Set("Authorization", "Bearer "+idToken.AccessToken)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Printf("[Jira] error response %d for %s: %s", resp.StatusCode, path, string(body))
		return nil, fmt.Errorf("jira API %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[Jira] success %d for %s", resp.StatusCode, path)
	return body, nil
}

// fetchParentIssue fetches a parent ticket with its subtask keys.
func fetchParentIssue(token, key string) (*jiraIssue, error) {
	data, err := callJira(token, fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status,subtasks", key))
	if err != nil {
		return nil, err
	}
	var issue jiraIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal issue: %w", err)
	}
	return &issue, nil
}

// fetchChildIssue fetches a child ticket with full detail.
func fetchChildIssue(token, key string) (*jiraIssue, error) {
	data, err := callJira(token, fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status,assignee,updated", key))
	if err != nil {
		return nil, err
	}
	var issue jiraIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal child issue: %w", err)
	}
	return &issue, nil
}

// fetchComments fetches comments on an issue, filtering to those created after `since`.
func fetchComments(token, key string, since time.Time) ([]jiraComment, error) {
	data, err := callJira(token, fmt.Sprintf("/rest/api/3/issue/%s/comment?orderBy=-created&maxResults=20", key))
	if err != nil {
		return nil, err
	}
	var resp jiraCommentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var recent []jiraComment
	for _, c := range resp.Comments {
		t, err := time.Parse("2006-01-02T15:04:05.000-0700", c.Created)
		if err != nil {
			continue
		}
		if t.After(since) {
			recent = append(recent, c)
		}
	}
	return recent, nil
}

// fetchChangelog fetches field changes on an issue after `since`.
func fetchChangelog(token, key string, since time.Time) ([]jiraChangelogEntry, error) {
	data, err := callJira(token, fmt.Sprintf("/rest/api/3/issue/%s/changelog", key))
	if err != nil {
		return nil, err
	}
	var resp jiraChangelogResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var recent []jiraChangelogEntry
	for _, e := range resp.Values {
		t, err := time.Parse("2006-01-02T15:04:05.000-0700", e.Created)
		if err != nil {
			continue
		}
		if t.After(since) {
			recent = append(recent, e)
		}
	}
	return recent, nil
}

// extractCommentText extracts plain text from a Jira ADF body.
func extractCommentText(raw json.RawMessage) string {
	// ADF is nested; just grab all "text" leaf nodes
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "(comment)"
	}
	var texts []string
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch node := v.(type) {
		case map[string]interface{}:
			if t, ok := node["type"].(string); ok && t == "text" {
				if txt, ok := node["text"].(string); ok {
					texts = append(texts, txt)
				}
			}
			if content, ok := node["content"]; ok {
				walk(content)
			}
		case []interface{}:
			for _, item := range node {
				walk(item)
			}
		}
	}
	walk(doc)
	text := strings.Join(texts, " ")
	if len(text) > 120 {
		text = text[:117] + "..."
	}
	return text
}

// isStale returns true if the issue has not been updated in the last 72 hours.
func isStale(updatedStr string) bool {
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedStr)
	if err != nil {
		return false
	}
	return time.Since(t) > 72*time.Hour
}

// isRecentlyUpdated returns true if the issue was updated in the last 24 hours.
func isRecentlyUpdated(updatedStr string) bool {
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedStr)
	if err != nil {
		return false
	}
	return time.Since(t) < 24*time.Hour
}

// --- Message building ---

type childDetail struct {
	issue     *jiraIssue
	comments  []jiraComment
	changelog []jiraChangelogEntry
}

type parentUpdate struct {
	parent   *jiraIssue
	children []childDetail
}

// buildCondensedMessage builds the main Slack message across all tracked parents.
func buildCondensedMessage(updates []parentUpdate) string {
	now := time.Now()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Daily Update — %s*\n\n", now.Format("Mon Jan 2")))

	var blockers []string
	var recentActivity []string

	for _, u := range updates {
		var done, inProgress, blocked int
		for _, c := range u.children {
			status := strings.ToLower(c.issue.Fields.Status.Name)
			switch {
			case status == "done" || status == "closed" || status == "resolved":
				done++
			case status == "blocked":
				blocked++
				blockers = append(blockers, c.issue.Key)
			default:
				inProgress++
			}
			if isStale(c.issue.Fields.Updated) && status != "done" && status != "closed" && status != "resolved" {
				blockers = append(blockers, fmt.Sprintf("%s (no update in 3d)", c.issue.Key))
			}
			// Recent activity: status changed or assignee changed in last 24h
			for _, log := range c.changelog {
				for _, item := range log.Items {
					if item.Field == "status" || item.Field == "assignee" {
						var detail string
						if item.Field == "status" {
							detail = fmt.Sprintf("%s: %s -> %s", c.issue.Key, item.FromString, item.ToString)
						} else {
							detail = fmt.Sprintf("%s: assigned to %s", c.issue.Key, item.ToString)
						}
						recentActivity = append(recentActivity, detail)
					}
				}
			}
		}

		sb.WriteString(fmt.Sprintf("*%s* · %s  [%s]\n", u.parent.Key, u.parent.Fields.Summary, u.parent.Fields.Status.Name))
		sb.WriteString(fmt.Sprintf("  %d done  %d in progress", done, inProgress))
		if blocked > 0 {
			sb.WriteString(fmt.Sprintf("  %d blocked", blocked))
		}
		sb.WriteString("\n\n")
	}

	if len(blockers) > 0 {
		// Deduplicate
		seen := map[string]bool{}
		var unique []string
		for _, b := range blockers {
			if !seen[b] {
				seen[b] = true
				unique = append(unique, b)
			}
		}
		sb.WriteString(fmt.Sprintf("*Blockers:* %s\n", strings.Join(unique, ", ")))
	}
	if len(recentActivity) > 0 {
		limit := 5
		if len(recentActivity) < limit {
			limit = len(recentActivity)
		}
		sb.WriteString(fmt.Sprintf("*Recent (24h):* %s\n", strings.Join(recentActivity[:limit], " · ")))
	}

	return sb.String()
}

// buildDetailedMessage builds the thread reply for a single parent ticket.
func buildDetailedMessage(u parentUpdate) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Full breakdown — %s*\n\n", u.parent.Key))

	sb.WriteString("*Children:*\n")
	for _, c := range u.children {
		assignee := "unassigned"
		if c.issue.Fields.Assignee != nil {
			assignee = c.issue.Fields.Assignee.DisplayName
		}
		sb.WriteString(fmt.Sprintf("  `%s`  %s  [%s]  %s\n",
			c.issue.Key, c.issue.Fields.Summary, c.issue.Fields.Status.Name, assignee))
	}

	// Comments from the last 24h
	var commentLines []string
	for _, c := range u.children {
		for _, comment := range c.comments {
			text := extractCommentText(comment.Body)
			commentLines = append(commentLines, fmt.Sprintf("  %s: \"%s\" — %s", c.issue.Key, text, comment.Author.DisplayName))
		}
	}
	if len(commentLines) > 0 {
		sb.WriteString("\n*Recent comments:*\n")
		for _, l := range commentLines {
			sb.WriteString(l + "\n")
		}
	}

	// Changelog from the last 24h
	var changeLines []string
	for _, c := range u.children {
		for _, entry := range c.changelog {
			for _, item := range entry.Items {
				if item.Field == "status" || item.Field == "assignee" {
					var detail string
					if item.Field == "status" {
						detail = fmt.Sprintf("  %s: %s <- %s (%s)", c.issue.Key, item.ToString, item.FromString, entry.Author.DisplayName)
					} else {
						detail = fmt.Sprintf("  %s: assigned to %s (%s)", c.issue.Key, item.ToString, entry.Author.DisplayName)
					}
					changeLines = append(changeLines, detail)
				}
			}
		}
	}
	if len(changeLines) > 0 {
		sb.WriteString("\n*Changelog (24h):*\n")
		for _, l := range changeLines {
			sb.WriteString(l + "\n")
		}
	}

	return sb.String()
}

// --- Orchestration ---

// runDailyUpdate fetches all Jira data and posts to Slack. Returns the Slack message timestamp.
func runDailyUpdate(db *sql.DB, bot *slacklib.Bot) (string, error) {
	log.Printf("[DailyUpdate] starting update run")

	cfg, err := dbGetUpdateConfig(db)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	if cfg.Channel == "" {
		return "", fmt.Errorf("no Slack channel configured")
	}
	if cfg.RequestToken == "" {
		return "", fmt.Errorf("no Jira token stored — user must save config from the UI first")
	}

	tickets, err := dbGetTrackedTickets(db)
	if err != nil {
		return "", fmt.Errorf("load tickets: %w", err)
	}
	if len(tickets) == 0 {
		return "", fmt.Errorf("no tickets configured")
	}

	since := time.Now().Add(-24 * time.Hour)
	var updates []parentUpdate

	for _, t := range tickets {
		log.Printf("[DailyUpdate] fetching parent %s", t.JiraKey)
		parent, err := fetchParentIssue(cfg.RequestToken, t.JiraKey)
		if err != nil {
			log.Printf("[DailyUpdate] error fetching %s: %v — skipping", t.JiraKey, err)
			continue
		}

		var children []childDetail
		for _, subtask := range parent.Fields.Subtasks {
			child, err := fetchChildIssue(cfg.RequestToken, subtask.Key)
			if err != nil {
				log.Printf("[DailyUpdate] error fetching child %s: %v — skipping", subtask.Key, err)
				continue
			}

			var comments []jiraComment
			var changelog []jiraChangelogEntry

			if isRecentlyUpdated(child.Fields.Updated) {
				comments, _ = fetchComments(cfg.RequestToken, child.Key, since)
				changelog, _ = fetchChangelog(cfg.RequestToken, child.Key, since)
			}

			children = append(children, childDetail{
				issue:     child,
				comments:  comments,
				changelog: changelog,
			})
		}

		updates = append(updates, parentUpdate{parent: parent, children: children})
	}

	if len(updates) == 0 {
		return "", fmt.Errorf("no Jira data fetched")
	}

	// Post condensed message
	condensed := buildCondensedMessage(updates)
	log.Printf("[DailyUpdate] posting condensed message to %s", cfg.Channel)
	result, err := bot.SendMessage(nil, cfg.Channel, condensed)
	if err != nil {
		return "", fmt.Errorf("post condensed: %w", err)
	}
	mainTS := result.Timestamp
	log.Printf("[DailyUpdate] condensed message posted, ts=%s", mainTS)

	// Post detailed message as thread replies (one per parent)
	for _, u := range updates {
		detail := buildDetailedMessage(u)
		if _, err := bot.SendMessageInThread(nil, cfg.Channel, detail, mainTS); err != nil {
			log.Printf("[DailyUpdate] error posting detail for %s: %v", u.parent.Key, err)
		}
	}

	log.Printf("[DailyUpdate] update complete")
	return mainTS, nil
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`

Expected: may fail because `registerAPIRoutes` signature is still old. That's fine — we'll fix it next.

---

### Task 4: Add API routes to `api.go`

**Description**
Update `registerAPIRoutes` to accept a `*sql.DB`, add 7 new route handlers for ticket CRUD, config CRUD, manual trigger, and the cron check endpoint. The cron endpoint is registered at `/internal/jobs/check-daily-update` on the root router (not the `/api` group).

**Files:**
- Modify: `api.go`

**Step 1: Update the function signature**

Find the line:
```go
func registerAPIRoutes(r *gin.Engine, bot *slacklib.Bot, anaheimClient *anaheim.Client) {
```

Change to:
```go
func registerAPIRoutes(r *gin.Engine, bot *slacklib.Bot, anaheimClient *anaheim.Client, db *sql.DB) {
```

Also add `"database/sql"` to the imports block at the top of `api.go` if it's not already there.

**Step 2: Add the new routes inside `registerAPIRoutes`**

After the existing `api.GET("/feedback", handleGetFeedback())` line, add:

```go
	// Daily Update routes (enabled if MySQL is available)
	if db != nil {
		api.GET("/tickets", handleListTickets(db))
		api.POST("/tickets", handleAddTicket(db))
		api.DELETE("/tickets/:key", handleRemoveTicket(db))
		api.GET("/update-config", handleGetUpdateConfig(db))
		api.PUT("/update-config", handleSaveUpdateConfig(db))
		api.POST("/trigger-update", handleTriggerUpdate(db, bot))
	}

	// Cron endpoint — registered outside the /api group so the cron can reach it
	// (Cloud Scheduler hits /internal/jobs/check-daily-update directly)
	if db != nil {
		r.POST("/internal/jobs/check-daily-update", handleCheckDailyUpdate(db, bot))
	}
```

**Step 3: Add the handler functions**

Add these functions at the bottom of `api.go`:

```go
// --- Tracked Ticket handlers ---

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

		// Fetch summary from Jira using the request token
		token := c.GetHeader("X-Request-Token")
		summary := key // fallback if Jira fetch fails
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

		// Save the token so the cron job can use it later
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

// --- Update Config handlers ---

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

func handleSaveUpdateConfig(db *sql.DB) gin.HandlerFunc {
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

		// Preserve existing token
		existing, _ := dbGetUpdateConfig(db)
		token := c.GetHeader("X-Request-Token")
		if token == "" {
			token = existing.RequestToken
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
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// --- Trigger handlers ---

func handleTriggerUpdate(db *sql.DB, bot *slacklib.Bot) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] POST /api/trigger-update (manual)")

		// Save token from this request before running
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

		// Check idempotency — have we already posted today?
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

		// Check if now matches the configured post time (within 1 minute)
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			loc = time.UTC
		}
		now := time.Now().In(loc)
		configuredTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
		if configuredTime != cfg.PostTime {
			log.Printf("[CronCheck] not time yet (now=%s configured=%s) — skipping", configuredTime, cfg.PostTime)
			c.JSON(http.StatusOK, gin.H{"skipped": "not time yet"})
			return
		}

		log.Printf("[CronCheck] time matched — running daily update")
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
```

**Step 4: Add missing imports to api.go**

Add to the import block in `api.go`:
```go
"database/sql"
"fmt"
"log"
"time"
```

(Some of these may already be imported — check first. Do not add duplicates.)

**Step 5: Verify it compiles**

Run: `go build ./...`

Expected: no errors. Fix any undefined symbol or import errors before continuing.

---

### Task 5: Register the cron job via Apps Platform

**Description**
Create a Cloud Scheduler job that fires every minute and calls the cron check endpoint. This step must be done after deploying (Task 8), because the endpoint only exists on the deployed URL. Record it here for reference.

**Files:** None (CLI command only)

**Step 1: After deploying, run this command**

```bash
apps-platform app schedule create daily-update \
  --endpoint /internal/jobs/check-daily-update \
  --cron "* * * * *" \
  --timezone America/Los_Angeles \
  --service driver-behaviour-analysis
```

Expected output: confirmation that the schedule was created.

**Step 2: Verify the schedule exists**

```bash
apps-platform app schedule list --service driver-behaviour-analysis
```

Expected: `daily-update` appears in the list with cron `* * * * *`.

---

### Task 6: Create `DailyUpdateTab.tsx`

**Description**
Create the React component for the Daily Update settings tab. It has two panels: Tracked Tickets (add/remove Jira keys) and Schedule Settings (time, timezone, channel). A "Post Now" button triggers the update immediately.

**Files:**
- Create: `frontend/src/components/DailyUpdateTab.tsx`

**Step 1: Create the file**

```tsx
import { useState, useEffect } from 'react'

interface Ticket {
  id: number
  jira_key: string
  summary: string
  added_at: string
}

interface Config {
  post_time: string
  timezone: string
  channel: string
}

const TIMEZONES = [
  { label: 'Pacific (PT)', value: 'America/Los_Angeles' },
  { label: 'Eastern (ET)', value: 'America/New_York' },
  { label: 'UTC', value: 'UTC' },
]

export default function DailyUpdateTab() {
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [newKey, setNewKey] = useState('')
  const [addError, setAddError] = useState<string | null>(null)
  const [addLoading, setAddLoading] = useState(false)

  const [config, setConfig] = useState<Config>({ post_time: '09:00', timezone: 'America/Los_Angeles', channel: '' })
  const [configSaved, setConfigSaved] = useState(false)
  const [configError, setConfigError] = useState<string | null>(null)
  const [configLoading, setConfigLoading] = useState(false)

  const [triggerLoading, setTriggerLoading] = useState(false)
  const [triggerResult, setTriggerResult] = useState<string | null>(null)

  useEffect(() => {
    loadTickets()
    loadConfig()
  }, [])

  const loadTickets = async () => {
    try {
      const res = await fetch('/api/tickets')
      if (res.ok) {
        const data = await res.json()
        setTickets(data.tickets || [])
      }
    } catch {
      // ignore
    }
  }

  const loadConfig = async () => {
    try {
      const res = await fetch('/api/update-config')
      if (res.ok) {
        const data = await res.json()
        setConfig({ post_time: data.post_time || '09:00', timezone: data.timezone || 'America/Los_Angeles', channel: data.channel || '' })
      }
    } catch {
      // ignore
    }
  }

  const handleAddTicket = async () => {
    const key = newKey.trim().toUpperCase()
    if (!key) return
    setAddLoading(true)
    setAddError(null)
    try {
      const res = await fetch('/api/tickets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ jira_key: key }),
      })
      if (!res.ok) {
        const data = await res.json()
        setAddError(data.error || 'Failed to add ticket')
        return
      }
      setNewKey('')
      await loadTickets()
    } catch {
      setAddError('Network error')
    } finally {
      setAddLoading(false)
    }
  }

  const handleRemoveTicket = async (key: string) => {
    try {
      await fetch(`/api/tickets/${key}`, { method: 'DELETE' })
      await loadTickets()
    } catch {
      // ignore
    }
  }

  const handleSaveConfig = async () => {
    setConfigLoading(true)
    setConfigError(null)
    setConfigSaved(false)
    try {
      const res = await fetch('/api/update-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      })
      if (!res.ok) {
        const data = await res.json()
        setConfigError(data.error || 'Failed to save')
        return
      }
      setConfigSaved(true)
      setTimeout(() => setConfigSaved(false), 2000)
    } catch {
      setConfigError('Network error')
    } finally {
      setConfigLoading(false)
    }
  }

  const handlePostNow = async () => {
    setTriggerLoading(true)
    setTriggerResult(null)
    try {
      const res = await fetch('/api/trigger-update', { method: 'POST' })
      if (!res.ok) {
        const data = await res.json()
        setTriggerResult('Error: ' + (data.error || 'Unknown error'))
        return
      }
      setTriggerResult('Posted! Check your Slack channel.')
    } catch {
      setTriggerResult('Network error')
    } finally {
      setTriggerLoading(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto space-y-8 py-4">

      {/* Tracked Tickets panel */}
      <div className="bg-white rounded-xl shadow p-6">
        <h2 className="text-xl font-semibold text-gray-800 mb-1">Tracked Tickets</h2>
        <p className="text-sm text-gray-500 mb-4">
          Add parent Jira ticket keys to track. The bot will follow all child tickets under each parent.
        </p>

        <div className="flex gap-2 mb-3">
          <input
            type="text"
            value={newKey}
            onChange={e => setNewKey(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleAddTicket()}
            placeholder="e.g. ADP-123"
            className="flex-1 border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button
            onClick={handleAddTicket}
            disabled={addLoading || !newKey.trim()}
            className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 text-white px-4 py-2 rounded-lg text-sm font-medium"
          >
            {addLoading ? 'Adding...' : 'Add'}
          </button>
        </div>

        {addError && <p className="text-red-600 text-sm mb-3">{addError}</p>}

        {tickets.length === 0 ? (
          <p className="text-gray-400 text-sm">No tickets added yet.</p>
        ) : (
          <ul className="divide-y divide-gray-100">
            {tickets.map(t => (
              <li key={t.jira_key} className="flex items-center justify-between py-3">
                <div>
                  <span className="font-mono text-sm font-medium text-blue-700">{t.jira_key}</span>
                  <span className="text-gray-600 text-sm ml-2">{t.summary}</span>
                </div>
                <button
                  onClick={() => handleRemoveTicket(t.jira_key)}
                  className="text-gray-400 hover:text-red-600 text-sm ml-4"
                >
                  Remove
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Schedule Settings panel */}
      <div className="bg-white rounded-xl shadow p-6">
        <h2 className="text-xl font-semibold text-gray-800 mb-1">Schedule</h2>
        <p className="text-sm text-gray-500 mb-4">
          The bot will post a daily update to your Slack channel at the configured time.
        </p>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Post time</label>
            <input
              type="time"
              value={config.post_time}
              onChange={e => setConfig(prev => ({ ...prev, post_time: e.target.value }))}
              className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Timezone</label>
            <select
              value={config.timezone}
              onChange={e => setConfig(prev => ({ ...prev, timezone: e.target.value }))}
              className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {TIMEZONES.map(tz => (
                <option key={tz.value} value={tz.value}>{tz.label}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Slack channel</label>
            <input
              type="text"
              value={config.channel}
              onChange={e => setConfig(prev => ({ ...prev, channel: e.target.value }))}
              placeholder="#adp-daily"
              className="border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 w-full"
            />
          </div>

          <div className="flex items-center gap-3 pt-2">
            <button
              onClick={handleSaveConfig}
              disabled={configLoading}
              className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-300 text-white px-5 py-2 rounded-lg text-sm font-medium"
            >
              {configLoading ? 'Saving...' : configSaved ? 'Saved!' : 'Save'}
            </button>
            <button
              onClick={handlePostNow}
              disabled={triggerLoading}
              className="bg-gray-700 hover:bg-gray-800 disabled:bg-gray-300 text-white px-5 py-2 rounded-lg text-sm font-medium"
            >
              {triggerLoading ? 'Posting...' : 'Post Now'}
            </button>
          </div>

          {configError && <p className="text-red-600 text-sm">{configError}</p>}
          {triggerResult && (
            <p className={`text-sm ${triggerResult.startsWith('Error') ? 'text-red-600' : 'text-green-600'}`}>
              {triggerResult}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
```

---

### Task 7: Wire the tab into `App.tsx` and `Sidebar.tsx`

**Description**
Add "Daily Update" as a new tab in the sidebar and update `App.tsx` to render `DailyUpdateTab` when that tab is active.

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/Sidebar.tsx`

**Step 1: Update `App.tsx`**

Find:
```tsx
export type Tab = 'home'
```

Change to:
```tsx
export type Tab = 'home' | 'daily-update'
```

Add the import at the top:
```tsx
import DailyUpdateTab from './components/DailyUpdateTab'
```

Find:
```tsx
  const renderContent = () => {
    return <Home />
  }
```

Change to:
```tsx
  const renderContent = () => {
    switch (activeTab) {
      case 'daily-update': return <DailyUpdateTab />
      default: return <Home />
    }
  }
```

**Step 2: Update `Sidebar.tsx`**

Find:
```tsx
const tabs: { id: Tab; label: string; icon: string }[] = [
  { id: 'home', label: 'Home', icon: '🏠' },
]
```

Change to:
```tsx
const tabs: { id: Tab; label: string; icon: string }[] = [
  { id: 'home', label: 'Home', icon: '🏠' },
  { id: 'daily-update', label: 'Daily Update', icon: '📋' },
]
```

---

### Task 8: Build, deploy, and verify

**Description**
Compile the Go binary, deploy to Cloud Run via the Apps Platform, and verify via logs that MySQL connected and migrations ran cleanly.

**Files:** None

**Step 1: Build**

Run: `go build ./...`

Expected: no errors. If there are errors, fix them before deploying.

**Step 2: Deploy**

Use the `apps-platform` skill to run:
```bash
apps-platform app deploy
```

Expected: build succeeds and a new revision is deployed.

**Step 3: Check startup logs**

Use the `apps-platform` skill to run:
```bash
apps-platform app logs --service driver-behaviour-analysis
```

Look for these log lines:
- `[MySQL] connected via Cloud SQL IAM auth` — confirms DB connection
- `[MySQL] migrations complete` — confirms tables were created

If you see MySQL errors, check that `enable_mysql = true` is set in `project.toml` (it already is).

**Step 4: Register the cron schedule** (see Task 5)

Run the `apps-platform app schedule create` command from Task 5 after the deploy succeeds.

**Step 5: Verify cron is firing**

Wait 1–2 minutes, then check logs again:

```bash
apps-platform app logs --service driver-behaviour-analysis
```

Look for: `[CronCheck] /internal/jobs/check-daily-update fired`

This confirms the scheduler is hitting the endpoint.

---

### Task 9: Walk the user through the app

**What was built:**
A Daily Update tab in your app where you configure which Jira parent tickets to track, set a daily post time and Slack channel, and post updates on demand or on a schedule. Every day at the configured time, the bot automatically posts a condensed Jira status summary to Slack, then replies in the thread with a detailed breakdown of every child ticket, recent comments, and status changes.

**How to use it:**

1. Open your deployed app and click **"Daily Update"** in the left sidebar.

2. In the **Tracked Tickets** panel, type a Jira parent ticket key (e.g. `ADP-123`) and click **Add**. The app will fetch the ticket's name from Jira and display it. Repeat for each parent ticket you want to track.

3. In the **Schedule** panel, set your preferred post time, timezone, and the Slack channel you want updates posted to (e.g. `#adp-daily`). Click **Save**.

4. Click **Post Now** to immediately post an update and verify it looks correct in Slack. You'll see a condensed summary in the channel, with a detailed thread reply below it.

5. Going forward, the bot will automatically post every day at your configured time.

**Setup required:**
- Make sure **Jira** is connected in the integration bar at the top of the app before adding tickets.
- The Slack bot must be invited to the target channel. In Slack, type `/invite @<your-bot-name>` in the channel.
- After the first deploy, run the `apps-platform app schedule create` command from Task 5 to activate the daily cron.

**If something looks wrong:**
- If "Post Now" returns an error like "no Jira token stored", try clicking Add on a ticket first (this refreshes the stored token), then click Post Now again.
- If the update posts but Jira data is missing, check that the Jira integration is connected and your tracked ticket keys are correct.
- Check logs with `apps-platform app logs` for detailed error messages.
