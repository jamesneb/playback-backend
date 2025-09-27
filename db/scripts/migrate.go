package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jamesneb/playback-backend/pkg/config"
)

type Migration struct {
	Version int
	Name    string
	Path    string
	Content string
}

type MigrationRunner struct {
	db          *sql.DB
	dbName      string
	migrationsPath string
	envPath     string
}

func NewMigrationRunner(dsn, dbName, migrationsPath, envPath string) (*MigrationRunner, error) {
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &MigrationRunner{
		db:          db,
		dbName:      dbName,
		migrationsPath: migrationsPath,
		envPath:     envPath,
	}, nil
}

func (mr *MigrationRunner) Close() error {
	return mr.db.Close()
}

// Load migrations from directory, sorted by version number
func (mr *MigrationRunner) LoadMigrations() ([]Migration, error) {
	files, err := os.ReadDir(mr.migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []Migration
	migrationRegex := regexp.MustCompile(`^(\d{4})_(.+)\.sql$`)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := migrationRegex.FindStringSubmatch(file.Name())
		if len(matches) != 3 {
			log.Printf("Warning: skipping file with invalid name format: %s", file.Name())
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Printf("Warning: invalid version number in file: %s", file.Name())
			continue
		}

		filePath := filepath.Join(mr.migrationsPath, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    matches[2],
			Path:    filePath,
			Content: string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Initialize schema_migrations table
func (mr *MigrationRunner) InitializeMigrationsTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.schema_migrations (
			version UInt64,
			name String,
			applied_at DateTime64(9) DEFAULT now64()
		) ENGINE = MergeTree()
		ORDER BY version
	`, mr.dbName)

	_, err := mr.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	return nil
}

// Get applied migration versions
func (mr *MigrationRunner) GetAppliedMigrations() (map[int]bool, error) {
	query := fmt.Sprintf("SELECT version FROM %s.schema_migrations ORDER BY version", mr.dbName)
	rows, err := mr.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[version] = true
	}

	return applied, nil
}

// Apply environment configuration
func (mr *MigrationRunner) ApplyEnvironmentConfig() error {
	if mr.envPath == "" {
		log.Println("No environment file specified, skipping environment setup")
		return nil
	}

	content, err := os.ReadFile(mr.envPath)
	if err != nil {
		return fmt.Errorf("failed to read environment file %s: %w", mr.envPath, err)
	}

	// Replace ${DB} placeholder with actual database name
	envSQL := strings.ReplaceAll(string(content), "${DB}", mr.dbName)
	
	// Split by semicolon and execute each statement
	statements := strings.Split(envSQL, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		if _, err := mr.db.Exec(stmt); err != nil {
			log.Printf("Warning: failed to execute environment statement '%s': %v", stmt, err)
			// Don't fail on environment setup errors, just warn
		}
	}

	log.Printf("Applied environment configuration from %s", mr.envPath)
	return nil
}

// Apply a single migration
func (mr *MigrationRunner) ApplyMigration(migration Migration) error {
	// Replace ${DB} placeholder with actual database name
	sql := strings.ReplaceAll(migration.Content, "${DB}", mr.dbName)
	
	// Split by semicolon and execute each statement
	statements := strings.Split(sql, ";")
	
	// Begin transaction-like behavior (ClickHouse is autocommit, so we track manually)
	log.Printf("Applying migration %04d_%s", migration.Version, migration.Name)
	
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		
		// Remove comment lines but keep the actual SQL
		var sqlLines []string
		for _, line := range strings.Split(stmt, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "--") {
				sqlLines = append(sqlLines, line)
			}
		}
		
		cleanSQL := strings.TrimSpace(strings.Join(sqlLines, "\n"))
		
		if cleanSQL == "" {
			continue
		}

		if _, err := mr.db.Exec(cleanSQL); err != nil {
			return fmt.Errorf("failed to execute statement in migration %d: %w\nStatement: %s", migration.Version, err, cleanSQL)
		}
	}

	// Record successful migration
	recordQuery := fmt.Sprintf(
		"INSERT INTO %s.schema_migrations (version, name) VALUES (?, ?)",
		mr.dbName,
	)
	
	if _, err := mr.db.Exec(recordQuery, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
	}

	log.Printf("Successfully applied migration %04d_%s", migration.Version, migration.Name)
	return nil
}

// Run all pending migrations
func (mr *MigrationRunner) Migrate() error {
	// Apply environment config first
	if err := mr.ApplyEnvironmentConfig(); err != nil {
		return fmt.Errorf("failed to apply environment config: %w", err)
	}

	// Initialize migrations table
	if err := mr.InitializeMigrationsTable(); err != nil {
		return err
	}

	// Load all migrations
	migrations, err := mr.LoadMigrations()
	if err != nil {
		return err
	}

	// Get applied migrations
	applied, err := mr.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Apply pending migrations
	pendingCount := 0
	for _, migration := range migrations {
		if applied[migration.Version] {
			log.Printf("Migration %04d_%s already applied, skipping", migration.Version, migration.Name)
			continue
		}

		if err := mr.ApplyMigration(migration); err != nil {
			return err
		}
		pendingCount++
	}

	if pendingCount == 0 {
		log.Println("No pending migrations found")
	} else {
		log.Printf("Successfully applied %d migrations", pendingCount)
	}

	return nil
}

func main() {
	// Load configuration using the same config system as the main app
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Extract ClickHouse configuration
	host := cfg.Database.ClickHouse.Host
	username := cfg.Database.ClickHouse.Username
	password := cfg.Database.ClickHouse.Password
	database := cfg.Database.ClickHouse.Database

	log.Printf("Connecting to ClickHouse at %s, target database: %s", host, database)

	// First, connect to default database to ensure target database exists
	defaultDSN := fmt.Sprintf("clickhouse://%s:%s@%s/default", username, password, host)
	defaultDB, err := sql.Open("clickhouse", defaultDSN)
	if err != nil {
		log.Fatalf("Failed to connect to default database: %v", err)
	}
	defer func() {
		if err := defaultDB.Close(); err != nil {
			log.Printf("Failed to close default database connection: %v", err)
		}
	}()

	// Create target database if it doesn't exist
	createDBQuery := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", database)
	if _, err := defaultDB.Exec(createDBQuery); err != nil {
		log.Fatalf("Failed to create database %s: %v", database, err)
	}
	log.Printf("Ensured database %s exists", database)

	// Build DSN for target database
	dsn := fmt.Sprintf("clickhouse://%s:%s@%s/%s", username, password, host, database)

	// Paths
	migrationsPath := getEnv("MIGRATIONS_PATH", "./db/migrations")
	// Skip environment file - migration system handles database creation and variable substitution
	envPath := ""

	// Create migration runner
	runner, err := NewMigrationRunner(dsn, database, migrationsPath, envPath)
	if err != nil {
		log.Fatalf("Failed to create migration runner: %v", err)
	}
	defer func() {
		if err := runner.Close(); err != nil {
			log.Printf("Failed to close migration runner: %v", err)
		}
	}()

	// Run migrations
	if err := runner.Migrate(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migration completed successfully!")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

