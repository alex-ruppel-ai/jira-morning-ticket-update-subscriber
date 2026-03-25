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
	PostTime     string `json:"post_time"`
	Timezone     string `json:"timezone"`
	Channel      string `json:"channel"`
	RequestToken string `json:"request_token"`
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
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tracked_tickets (
			id       INT AUTO_INCREMENT PRIMARY KEY,
			jira_key VARCHAR(50) NOT NULL UNIQUE,
			summary  VARCHAR(500),
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS update_config (
			id            INT AUTO_INCREMENT PRIMARY KEY,
			post_time     VARCHAR(5)   NOT NULL DEFAULT '09:00',
			timezone      VARCHAR(50)  NOT NULL DEFAULT 'America/Los_Angeles',
			channel       VARCHAR(100) NOT NULL DEFAULT '',
			request_token VARCHAR(500) NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS daily_update_log (
			id        INT AUTO_INCREMENT PRIMARY KEY,
			post_date DATE NOT NULL UNIQUE,
			posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			slack_ts  VARCHAR(100)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
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
		return UpdateConfig{PostTime: "09:00", Timezone: "America/Los_Angeles"}, nil
	}
	return cfg, err
}

func dbSaveUpdateConfig(db *sql.DB, cfg UpdateConfig) error {
	if _, err := db.Exec("DELETE FROM update_config"); err != nil {
		return err
	}
	_, err := db.Exec(
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
