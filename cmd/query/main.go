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

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run query.go \"SELECT * FROM table\"")
	}

	query := strings.Join(os.Args[1:], " ")

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
	defer clickhouseClient.Close()

	ctx := context.Background()
	rows, err := clickhouseClient.Query(ctx, query)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

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