package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.apps.applied.dev/lib/slacklib"
)

// --- Jira API structs ---

type jiraIssue struct {
	Key    string     `json:"key"`
	Fields jiraFields `json:"fields"`
}

type jiraFields struct {
	Summary    string           `json:"summary"`
	Status     jiraStatus       `json:"status"`
	Assignee   *jiraUser        `json:"assignee"`
	Updated    string           `json:"updated"`
	Subtasks   []jiraSubtask    `json:"subtasks"`
	IssueLinks []jiraIssueLink  `json:"issuelinks"`
}

type jiraIssueLink struct {
	Type         jiraIssueLinkType `json:"type"`
	InwardIssue  *jiraIssue        `json:"inwardIssue"`
	OutwardIssue *jiraIssue        `json:"outwardIssue"`
}

type jiraIssueLinkType struct {
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
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

// callJira makes a server-side GET to the Jira integration using the stored X-Request-Token.
func callJira(token, path string) ([]byte, error) {
	targetURL := dataAPIURL() + "/api/data/jira" + path
	log.Printf("[Jira] GET %s", targetURL)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Request-Token", token)

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
	data, err := callJira(token, fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status,subtasks,issuelinks", key))
	if err != nil {
		return nil, err
	}
	var issue jiraIssue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, fmt.Errorf("unmarshal issue: %w", err)
	}
	return &issue, nil
}

// fetchChildIssue fetches a child ticket with status, assignee, and updated time.
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

// jiraSearchResponse wraps the JQL search result.
type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

// fetchChildrenByJQL fetches all child issues of a parent using JQL.
// This covers subtasks, next-gen children, and classic hierarchy children.
func fetchChildrenByJQL(token, parentKey string) ([]jiraIssue, error) {
	path := fmt.Sprintf("/rest/api/3/search/jql?jql=parent%%3D%s&fields=summary,status,assignee,updated,issuelinks&maxResults=100", parentKey)
	data, err := callJira(token, path)
	if err != nil {
		return nil, err
	}
	var resp jiraSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal search response: %w", err)
	}
	log.Printf("[Jira] JQL parent=%s returned %d children", parentKey, len(resp.Issues))
	return resp.Issues, nil
}

// buildUpdateMessage builds a condensed Slack message for a parent ticket and its children.
func buildUpdateMessage(parent *jiraIssue, children []*jiraIssue) string {
	var sb strings.Builder
	now := time.Now()
	sb.WriteString(fmt.Sprintf("*Daily Update — %s*\n\n", now.Format("Mon Jan 2")))
	sb.WriteString(fmt.Sprintf("*%s* · %s  [%s]\n", parent.Key, parent.Fields.Summary, parent.Fields.Status.Name))

	var done, inProgress, blocked int
	for _, c := range children {
		switch strings.ToLower(c.Fields.Status.Name) {
		case "done", "closed", "resolved":
			done++
		case "blocked":
			blocked++
		default:
			inProgress++
		}
	}

	sb.WriteString(fmt.Sprintf("  %d done  %d in progress", done, inProgress))
	if blocked > 0 {
		sb.WriteString(fmt.Sprintf("  %d blocked", blocked))
	}
	sb.WriteString(fmt.Sprintf("\n  (%d children total)\n", len(children)))

	return sb.String()
}

// --- Comment and changelog types (used by runDailyUpdate) ---

type jiraCommentResponse struct {
	Comments []jiraComment `json:"comments"`
}

type jiraComment struct {
	Author  jiraUser        `json:"author"`
	Body    json.RawMessage `json:"body"`
	Created string          `json:"created"`
}

type jiraChangelogResponse struct {
	Values []jiraChangelogEntry `json:"values"`
}

type jiraChangelogEntry struct {
	Author  jiraUser         `json:"author"`
	Created string           `json:"created"`
	Items   []jiraChangeItem `json:"items"`
}

type jiraChangeItem struct {
	Field      string `json:"field"`
	FromString string `json:"fromString"`
	ToString   string `json:"toString"`
}

type childDetail struct {
	issue     *jiraIssue
	comments  []jiraComment
	changelog []jiraChangelogEntry
}

type parentUpdate struct {
	parent       *jiraIssue
	children     []childDetail // all descendants, recursively
	linkedIssues []childDetail // issues linked via issuelinks field
}

// fetchDescendantsRecursive recursively processes a slice of already-fetched issues in parallel.
// Each issue's comments, changelog, and sub-children JQL are fetched concurrently.
// visited is protected by mu; depth prevents runaway recursion.
func fetchDescendantsRecursive(token string, issues []jiraIssue, since time.Time, depth int, visited *sync.Map) []childDetail {
	if depth == 0 || len(issues) == 0 {
		return nil
	}

	type itemResult struct {
		detail   childDetail
		subIssues []jiraIssue
	}

	results := make([]itemResult, len(issues))
	var wg sync.WaitGroup

	for i := range issues {
		issue := issues[i]
		if _, loaded := visited.LoadOrStore(issue.Key, true); loaded {
			continue
		}
		wg.Add(1)
		go func(idx int, iss jiraIssue) {
			defer wg.Done()
			issueCopy := iss

			var comments []jiraComment
			var changelog []jiraChangelogEntry
			// Only fetch activity for non-backlog tickets updated in the last 24h
			status := strings.ToLower(iss.Fields.Status.Name)
			log.Printf("[Jira] issue %s: status=%q updated=%s recentlyUpdated=%v", iss.Key, status, iss.Fields.Updated, isRecentlyUpdated(iss.Fields.Updated))
			if isRecentlyUpdated(iss.Fields.Updated) && status != "backlog" {
				var cwg sync.WaitGroup
				cwg.Add(2)
				go func() {
					defer cwg.Done()
					comments, _ = fetchComments(token, iss.Key, since)
				}()
				go func() {
					defer cwg.Done()
					changelog, _ = fetchChangelog(token, iss.Key, since)
				}()
				cwg.Wait()
			}

			log.Printf("[Jira] issue %s: fetched %d comments, %d changelog entries", iss.Key, len(comments), len(changelog))
			subIssues, _ := fetchChildrenByJQL(token, iss.Key)
			results[idx] = itemResult{
				detail:    childDetail{issue: &issueCopy, comments: comments, changelog: changelog},
				subIssues: subIssues,
			}
		}(i, issue)
	}
	wg.Wait()

	var all []childDetail
	for _, r := range results {
		if r.detail.issue == nil {
			continue
		}
		all = append(all, r.detail)
		if len(r.subIssues) > 0 {
			all = append(all, fetchDescendantsRecursive(token, r.subIssues, since, depth-1, visited)...)
		}
	}
	return all
}

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

func extractCommentText(raw json.RawMessage) string {
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

func isStale(updatedStr string) bool {
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedStr)
	if err != nil {
		return false
	}
	return time.Since(t) > 72*time.Hour
}

func isRecentlyUpdated(updatedStr string) bool {
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedStr)
	if err != nil {
		return false
	}
	return time.Since(t) < 24*time.Hour
}

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
			for _, entry := range c.changelog {
				for _, item := range entry.Items {
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

// runDailyUpdate reads config and tickets from DB, fetches Jira data, and posts to Slack.
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
		return "", fmt.Errorf("no Jira token stored — add a ticket from the UI first to capture your token")
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

		// Fetch direct children via JQL
		directChildren, err := fetchChildrenByJQL(cfg.RequestToken, t.JiraKey)
		if err != nil {
			log.Printf("[DailyUpdate] JQL failed for %s: %v — falling back to subtasks", t.JiraKey, err)
			for _, subtask := range parent.Fields.Subtasks {
				directChildren = append(directChildren, jiraIssue{Key: subtask.Key, Fields: subtask.Fields})
			}
		}

		// Recursively fetch all descendants in parallel (visited map prevents cycles)
		visited := &sync.Map{}
		visited.Store(t.JiraKey, true)
		children := fetchDescendantsRecursive(cfg.RequestToken, directChildren, since, 10, visited)
		log.Printf("[DailyUpdate] %s: %d total descendants fetched", t.JiraKey, len(children))

		// Collect linked issues from the parent
		var linkedIssues []childDetail
		for _, link := range parent.Fields.IssueLinks {
			var linked *jiraIssue
			if link.OutwardIssue != nil {
				linked = link.OutwardIssue
			} else if link.InwardIssue != nil {
				linked = link.InwardIssue
			}
			if linked == nil {
				continue
			}
			if _, loaded := visited.LoadOrStore(linked.Key, true); loaded {
				continue
			}
			fetched, err := fetchChildIssue(cfg.RequestToken, linked.Key)
			if err != nil {
				log.Printf("[DailyUpdate] error fetching linked issue %s: %v — skipping", linked.Key, err)
				continue
			}
			var comments []jiraComment
			var changelog []jiraChangelogEntry
			if isRecentlyUpdated(fetched.Fields.Updated) {
				comments, _ = fetchComments(cfg.RequestToken, fetched.Key, since)
				changelog, _ = fetchChangelog(cfg.RequestToken, fetched.Key, since)
			}
			linkedIssues = append(linkedIssues, childDetail{issue: fetched, comments: comments, changelog: changelog})
		}
		log.Printf("[DailyUpdate] %s: %d linked issues fetched", t.JiraKey, len(linkedIssues))

		updates = append(updates, parentUpdate{parent: parent, children: children, linkedIssues: linkedIssues})
	}

	if len(updates) == 0 {
		return "", fmt.Errorf("no Jira data fetched")
	}

	// Try Claude summaries first, fall back to templates if unavailable
	condensed, details, summaryErr := summarizeUpdate(context.Background(), updates)
	if summaryErr != nil {
		log.Printf("[DailyUpdate] using template messages (Claude unavailable: %v)", summaryErr)
		condensed = buildCondensedMessage(updates)
		details = nil
		for _, u := range updates {
			details = append(details, buildDetailedMessage(u))
		}
	}

	// Build thread title listing tracked parent tickets
	var parentKeys []string
	for _, u := range updates {
		parentKeys = append(parentKeys, u.parent.Key)
	}
	title := fmt.Sprintf(":thread: Summary over %s — %s", strings.Join(parentKeys, ", "), time.Now().Format("Mon Jan 2"))

	// Post the thread title to the channel
	log.Printf("[DailyUpdate] posting thread title to %s", cfg.Channel)
	result, err := bot.SendMessage(context.Background(), cfg.Channel, title)
	if err != nil {
		return "", fmt.Errorf("post title: %w", err)
	}
	mainTS := result.Timestamp
	log.Printf("[DailyUpdate] thread title posted, ts=%s", mainTS)

	// Thread reply 1: TL;DR of what changed in the last 24h
	if _, err := bot.SendMessageInThread(context.Background(), cfg.Channel, condensed, mainTS); err != nil {
		log.Printf("[DailyUpdate] error posting TL;DR: %v", err)
	}

	// Thread reply 2: Work-stream breakdown per parent
	for i, u := range updates {
		var breakdown string
		if i < len(details) {
			breakdown = details[i]
		} else {
			breakdown = buildDetailedMessage(u)
		}
		if _, err := bot.SendMessageInThread(context.Background(), cfg.Channel, breakdown, mainTS); err != nil {
			log.Printf("[DailyUpdate] error posting breakdown for %s: %v", u.parent.Key, err)
		}
	}

	log.Printf("[DailyUpdate] update complete")
	return mainTS, nil
}

// runUpdate fetches a single parent ticket + children and posts to Slack.
func runUpdate(ctx context.Context, token, jiraKey, channel string, bot *slacklib.Bot) error {
	log.Printf("[Update] starting update for %s -> %s", jiraKey, channel)

	parent, err := fetchParentIssue(token, jiraKey)
	if err != nil {
		return fmt.Errorf("fetch parent %s: %w", jiraKey, err)
	}
	log.Printf("[Update] fetched parent %s: %s [%s]", parent.Key, parent.Fields.Summary, parent.Fields.Status.Name)

	var children []*jiraIssue
	for _, subtask := range parent.Fields.Subtasks {
		child, err := fetchChildIssue(token, subtask.Key)
		if err != nil {
			log.Printf("[Update] error fetching child %s: %v — skipping", subtask.Key, err)
			continue
		}
		children = append(children, child)
		log.Printf("[Update] fetched child %s: %s [%s]", child.Key, child.Fields.Summary, child.Fields.Status.Name)
	}

	msg := buildUpdateMessage(parent, children)
	log.Printf("[Update] posting to channel %s", channel)

	if _, err := bot.SendMessage(ctx, channel, msg); err != nil {
		return fmt.Errorf("post to slack: %w", err)
	}

	log.Printf("[Update] done — posted update for %s", jiraKey)
	return nil
}
