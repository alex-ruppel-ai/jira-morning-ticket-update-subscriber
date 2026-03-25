package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// summarizeUpdate calls OpenAI to generate two Slack posts from raw Jira data.
// Returns (tldr, breakdowns per parent, error).
// tldr = what changed in the last 24h; breakdowns = grouped work-stream narrative.
// Falls back to template-based messages if no API key is set or the call fails.
func summarizeUpdate(ctx context.Context, updates []parentUpdate) (string, []string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Printf("[Summarize] OPENAI_API_KEY not set — falling back to template messages")
		return "", nil, fmt.Errorf("no api key")
	}

	client := openai.NewClient(apiKey)

	tldr, err := generateTLDR(ctx, client, updates)
	if err != nil {
		log.Printf("[Summarize] TL;DR failed: %v — falling back to template", err)
		return "", nil, err
	}

	var breakdowns []string
	for _, u := range updates {
		breakdown, err := generateBreakdown(ctx, client, u)
		if err != nil {
			log.Printf("[Summarize] breakdown for %s failed: %v — using template", u.parent.Key, err)
			breakdowns = append(breakdowns, buildDetailedMessage(u))
			continue
		}
		breakdowns = append(breakdowns, breakdown)
	}

	return tldr, breakdowns, nil
}

// generateTLDR produces a short post covering ONLY what changed in the last 24h.
func generateTLDR(ctx context.Context, client *openai.Client, updates []parentUpdate) (string, error) {
	now := time.Now()
	activityText := buildRecentActivityText(updates)

	userPrompt := fmt.Sprintf(`Today is %s. Write a TL;DR for an engineering team's Slack channel covering ONLY what changed in the last 24 hours.

Recent activity (last 24h only):
%s

Rules:
- First line: "*TL;DR — %s*"
- Maximum 5 bullet points
- Each bullet = one concrete thing that happened: a status change, an assignment, a comment decision, a blocker surfaced
- If nothing happened, write: "No changes in the last 24h."
- Do NOT describe the overall backlog state. Do NOT mention tickets with no recent activity.
- Name people and work areas. Ticket keys only when directly relevant.
- Use Slack markdown (• for bullets, *bold* for names/areas)`,
		now.Format("Monday January 2, 2006"),
		activityText,
		now.Format("Mon Jan 2"),
	)

	log.Printf("[Summarize] calling OpenAI for TL;DR (%d parents)", len(updates))
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4oMini,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You write ultra-concise daily TL;DRs for engineering teams. You only report facts — things that actually happened. You never pad with backlog state or generic encouragement. Use Slack markdown.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		MaxTokens: 400,
	})
	if err != nil {
		return "", fmt.Errorf("openai API: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in openai response")
	}
	text := resp.Choices[0].Message.Content
	log.Printf("[Summarize] TL;DR generated (%d chars)", len(text))
	return text, nil
}

// generateBreakdown produces a grouped work-stream narrative for one parent ticket.
func generateBreakdown(ctx context.Context, client *openai.Client, u parentUpdate) (string, error) {
	userPrompt := fmt.Sprintf(`Write a Slack post breaking down the work under Jira ticket %s for an engineering team.

%s

Rules:
- First line: "*%s — Work Stream Breakdown*"
- Group tickets into 2–5 named work streams (e.g. "AEB", "Sightline Integration", "Lane Keeping"). Infer groups from ticket titles.
- For each group: 1–2 sentences on WHERE THINGS STAND and what needs to happen next.
- Only call out tickets needing attention (stale 3+ days, or recent change). Format: `+"`"+`KEY`+"`"+` — reason.
- Skip routine backlog tickets with no news — Jira already shows those.
- End with "*Needs action:*" — name people and specific asks.
- Use Slack markdown (*bold* for group headers)`,
		u.parent.Key,
		buildSingleParentText(u),
		u.parent.Key,
	)

	log.Printf("[Summarize] calling OpenAI for breakdown of %s", u.parent.Key)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You write Jira work-stream breakdowns for engineering Slack channels. You group and synthesize — never list every ticket. Focus on what needs attention. Be direct. Use Slack markdown.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("openai API: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in openai response")
	}
	text := resp.Choices[0].Message.Content
	log.Printf("[Summarize] breakdown for %s generated (%d chars)", u.parent.Key, len(text))
	return text, nil
}

// buildRecentActivityText builds a prompt input containing ONLY tickets with recent activity.
func buildRecentActivityText(updates []parentUpdate) string {
	var sb strings.Builder
	for _, u := range updates {
		sb.WriteString(fmt.Sprintf("Parent: %s — %s\n", u.parent.Key, u.parent.Fields.Summary))
		hasActivity := false
		for _, c := range u.children {
			if len(c.comments) == 0 && len(c.changelog) == 0 {
				continue
			}
			hasActivity = true
			assignee := "unassigned"
			if c.issue.Fields.Assignee != nil {
				assignee = c.issue.Fields.Assignee.DisplayName
			}
			sb.WriteString(fmt.Sprintf("  %s: %s [%s] @%s\n", c.issue.Key, c.issue.Fields.Summary, c.issue.Fields.Status.Name, assignee))
			for _, comment := range c.comments {
				sb.WriteString(fmt.Sprintf("    Comment by %s: %s\n", comment.Author.DisplayName, extractCommentText(comment.Body)))
			}
			for _, entry := range c.changelog {
				for _, item := range entry.Items {
					if item.Field == "status" {
						sb.WriteString(fmt.Sprintf("    Status changed: %s → %s (by %s)\n", item.FromString, item.ToString, entry.Author.DisplayName))
					} else if item.Field == "assignee" {
						sb.WriteString(fmt.Sprintf("    Assigned to %s (by %s)\n", item.ToString, entry.Author.DisplayName))
					}
				}
			}
		}
		if !hasActivity {
			sb.WriteString("  (no activity in last 24h)\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildJiraDataText serializes all parent updates into readable text for the prompt.
func buildJiraDataText(updates []parentUpdate) string {
	var sb strings.Builder
	for _, u := range updates {
		sb.WriteString(buildSingleParentText(u))
		sb.WriteString("\n---\n")
	}
	return sb.String()
}

func buildSingleParentText(u parentUpdate) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Parent: %s — %s [%s]\n", u.parent.Key, u.parent.Fields.Summary, u.parent.Fields.Status.Name))
	sb.WriteString(fmt.Sprintf("Children (%d total):\n", len(u.children)))

	for _, c := range u.children {
		assignee := "unassigned"
		if c.issue.Fields.Assignee != nil {
			assignee = c.issue.Fields.Assignee.DisplayName
		}
		staleNote := ""
		if isStale(c.issue.Fields.Updated) {
			staleNote = " [NO UPDATE IN 3+ DAYS]"
		}
		sb.WriteString(fmt.Sprintf("  - %s: %s [%s] @%s%s\n",
			c.issue.Key, c.issue.Fields.Summary, c.issue.Fields.Status.Name, assignee, staleNote))

		if len(c.comments) > 0 {
			sb.WriteString("    Recent comments:\n")
			for _, comment := range c.comments {
				text := extractCommentText(comment.Body)
				sb.WriteString(fmt.Sprintf("      * \"%s\" — %s (%s)\n", text, comment.Author.DisplayName, comment.Created))
			}
		}

		if len(c.changelog) > 0 {
			sb.WriteString("    Recent changes:\n")
			for _, entry := range c.changelog {
				for _, item := range entry.Items {
					if item.Field == "status" {
						sb.WriteString(fmt.Sprintf("      * Status: %s → %s (by %s, %s)\n",
							item.FromString, item.ToString, entry.Author.DisplayName, entry.Created))
					} else if item.Field == "assignee" {
						sb.WriteString(fmt.Sprintf("      * Assigned to %s (by %s, %s)\n",
							item.ToString, entry.Author.DisplayName, entry.Created))
					}
				}
			}
		}
	}

	if len(u.linkedIssues) > 0 {
		sb.WriteString(fmt.Sprintf("Linked issues (%d total):\n", len(u.linkedIssues)))
		for _, c := range u.linkedIssues {
			assignee := "unassigned"
			if c.issue.Fields.Assignee != nil {
				assignee = c.issue.Fields.Assignee.DisplayName
			}
			staleNote := ""
			if isStale(c.issue.Fields.Updated) {
				staleNote = " [NO UPDATE IN 3+ DAYS]"
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s [%s] @%s%s\n",
				c.issue.Key, c.issue.Fields.Summary, c.issue.Fields.Status.Name, assignee, staleNote))
		}
	}

	return sb.String()
}
