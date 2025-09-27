package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// isValidQuery validates that the query is safe (read-only)
func isValidQuery(query string) bool {
	// Convert to uppercase for comparison
	query = strings.ToUpper(strings.TrimSpace(query))

	// Only allow read-only operations
	allowedPrefixes := []string{
		"SELECT",
		"SHOW",
		"DESCRIBE",
		"EXPLAIN",
		"WITH", // Common Table Expressions starting with WITH
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(query, prefix) {
			return true
		}
	}

	return false
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run query.go \"SELECT * FROM table\"")
	}

	// Validate that this is a safe query command-line tool (read-only operations)
	query := strings.Join(os.Args[1:], " ")
	if !isValidQuery(query) {
		log.Fatal("Only SELECT, SHOW, DESCRIBE, and EXPLAIN queries are allowed")
	}

	env := "local"
	if envVar := os.Getenv("ENV"); envVar != "" {
		env = envVar
	}

	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create ClickHouse client using the same pattern as the main app
	clickhouseClient, err := storage.NewClickHouseClient(&storage.ClickHouseConfig{
		Host:               cfg.Database.ClickHouse.Host,
		Database:           cfg.Database.ClickHouse.Database,
		Username:           cfg.Database.ClickHouse.Username,
		Password:           cfg.Database.ClickHouse.Password,
		MaxConnections:     cfg.Database.ClickHouse.MaxConnections,
		MaxIdleConnections: cfg.Database.ClickHouse.MaxIdleConnections,
	})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer func() {
		if err := clickhouseClient.Close(); err != nil {
			log.Printf("Failed to close ClickHouse client: %v", err)
		}
	}()

	ctx := context.Background()
	rows, err := clickhouseClient.Query(ctx, query)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	// Get column names
	cols := rows.Columns()

	// Print header
	fmt.Println(strings.Join(cols, "\t"))

	// Print rows
	values := make([]interface{}, len(cols))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		err := rows.Scan(scanArgs...)
		if err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}

		var rowData []string
		for _, value := range values {
			if value == nil {
				rowData = append(rowData, "NULL")
			} else {
				rowData = append(rowData, fmt.Sprintf("%v", value))
			}
		}
		fmt.Println(strings.Join(rowData, "\t"))
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Row iteration error: %v", err)
	}
}