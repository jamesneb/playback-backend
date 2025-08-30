package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/playback/playback-backend/pkg/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run query.go \"SELECT * FROM table\"")
	}

	query := strings.Join(os.Args[1:], " ")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := cfg.Database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	// Get column names
	cols, err := rows.Columns()
	if err != nil {
		log.Fatalf("Failed to get columns: %v", err)
	}

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