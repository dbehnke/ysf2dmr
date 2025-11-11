package database

import (
	"database/sql"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

// Config holds database configuration
type Config struct {
	Path string // Path to SQLite database file
}

// DB wraps the GORM database instance
type DB struct {
	db *gorm.DB
}

// NewDB creates a new database connection with pure Go SQLite driver
func NewDB(config Config, log *log.Logger) (*DB, error) {
	// Configure GORM logger
	var gormLog logger.Interface
	if log != nil {
		gormLog = logger.New(
			log,
			logger.Config{
				LogLevel:                  logger.Warn, // Only log warnings and errors
				IgnoreRecordNotFoundError: true,        // Don't log "record not found" errors
				Colorful:                  false,       // No color in logs
			},
		)
	} else {
		gormLog = logger.Default.LogMode(logger.Silent)
	}

	// Create dialector with pure Go SQLite driver
	dialector := sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        config.Path,
	}

	// Open database connection
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		return nil, err
	}

	// Get underlying SQL DB for PRAGMA settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure SQLite for optimal performance
	if err := configureSQLite(sqlDB); err != nil {
		return nil, err
	}

	// Auto-migrate database schema
	if err := db.AutoMigrate(&DMRUser{}); err != nil {
		return nil, err
	}

	if log != nil {
		log.Printf("Database initialized: %s", config.Path)
	}

	return &DB{db: db}, nil
}

// configureSQLite applies optimal SQLite settings
func configureSQLite(sqlDB *sql.DB) error {
	pragmaSettings := []string{
		"PRAGMA journal_mode=WAL",        // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL",      // Balanced safety/performance
		"PRAGMA busy_timeout=5000",       // 5 second timeout for busy database
		"PRAGMA cache_size=10000",        // Cache size in pages
		"PRAGMA foreign_keys=ON",         // Enable foreign key constraints
		"PRAGMA temp_store=memory",       // Store temporary tables in memory
	}

	for _, pragma := range pragmaSettings {
		if _, err := sqlDB.Exec(pragma); err != nil {
			return err
		}
	}

	return nil
}

// GetDB returns the underlying GORM database instance
func (db *DB) GetDB() *gorm.DB {
	return db.db
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Health checks if the database connection is healthy
func (db *DB) Health() error {
	sqlDB, err := db.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Stats returns database connection statistics
func (db *DB) Stats() sql.DBStats {
	sqlDB, _ := db.db.DB()
	return sqlDB.Stats()
}