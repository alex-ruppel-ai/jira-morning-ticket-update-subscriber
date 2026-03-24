package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"go.apps.applied.dev/lib/slacklib"
)

// --- Jira API structs ---

type jiraIssue struct {
	Key    string     `json:"key"`
	Fields jiraFields `json:"fields"`
}

type jiraFields struct {
	Summary  string        `json:"summary"`
	Status   jiraStatus    `json:"status"`
	Assignee *jiraUser     `json:"assignee"`
	Updated  string        `json:"updated"`
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
