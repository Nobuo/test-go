package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	mysqlOnce sync.Once
	mysqlDB   *sql.DB
	mysqlErr  error
)

// Connection returns a singleton MySQL connection that is initialized from
// environment variables. The DSN is built from the following variables:
//
//	MYSQL_HOST, MYSQL_PORT, MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE
//
// The returned *sql.DB is safe for concurrent use by multiple goroutines.
func Connection() (*sql.DB, error) {
	mysqlOnce.Do(func() {
		mysqlDB, mysqlErr = openFromEnv()
	})

	return mysqlDB, mysqlErr
}

func openFromEnv() (*sql.DB, error) {
	host := getenvDefault("MYSQL_HOST", "localhost")
	port := getenvDefault("MYSQL_PORT", "3306")
	user := os.Getenv("MYSQL_USER")
	pass := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_DATABASE")

	if user == "" || dbName == "" {
		return nil, fmt.Errorf("missing MYSQL_USER or MYSQL_DATABASE")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open failed: %w", err)
	}

	// Ensure the connection configuration is valid and the server is reachable.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping failed: %w", err)
	}

	// Provide reasonable defaults suitable for Lambda's execution model while
	// still working well for local development.
	db.SetConnMaxIdleTime(1 * time.Minute)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)

	return db, nil
}

func getenvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
