// Package main provides the database migration tool for GitLab Reviewer Roulette.
// It supports up, down, version, and force commands for managing database schema.
package main

import (
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/aimd54/gitlab-reviewer-roulette/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <up|down|version|force>")
		os.Exit(1)
	}

	command := os.Args[1]

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Build database connection string
	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.Postgres.User,
		cfg.Database.Postgres.Password,
		cfg.Database.Postgres.Host,
		cfg.Database.Postgres.Port,
		cfg.Database.Postgres.Database,
		cfg.Database.Postgres.SSLMode,
	)

	// Create migration instance
	m, err := migrate.New(
		"file://migrations",
		dbURL,
	)
	if err != nil {
		fmt.Printf("Failed to create migration instance: %v\n", err)
		os.Exit(1)
	}

	// Execute command
	var exitCode int
	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			fmt.Printf("Failed to run migrations: %v\n", err)
			exitCode = 1
		} else {
			fmt.Println("Migrations applied successfully")
		}

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			fmt.Printf("Failed to rollback migrations: %v\n", err)
			exitCode = 1
		} else {
			fmt.Println("Migrations rolled back successfully")
		}

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			fmt.Printf("Failed to get version: %v\n", err)
			exitCode = 1
		} else {
			fmt.Printf("Current version: %d (dirty: %v)\n", version, dirty)
		}

	case "force":
		if len(os.Args) < 3 {
			fmt.Println("Usage: migrate force <version>")
			exitCode = 1
		} else {
			var version int
			_, _ = fmt.Sscanf(os.Args[2], "%d", &version)
			if err := m.Force(version); err != nil {
				fmt.Printf("Failed to force version: %v\n", err)
				exitCode = 1
			} else {
				fmt.Printf("Forced to version %d\n", version)
			}
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Available commands: up, down, version, force")
		exitCode = 1
	}

	// Close migration instance before exiting
	sourceErr, dbErr := m.Close()
	if sourceErr != nil {
		fmt.Printf("Error closing migration source: %v\n", sourceErr)
	}
	if dbErr != nil {
		fmt.Printf("Error closing database: %v\n", dbErr)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
