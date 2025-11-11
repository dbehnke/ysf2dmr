package radioid

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/database"
)

const (
	// RadioIDURL is the URL to download the latest RadioID database
	RadioIDURL = "https://radioid.net/static/user.csv"

	// DefaultSyncInterval is how often to check for updates (24 hours)
	DefaultSyncInterval = 24 * time.Hour

	// RequestTimeout for HTTP requests
	RequestTimeout = 30 * time.Second

	// MaxRetries for failed downloads
	MaxRetries = 3

	// RetryDelay between retry attempts
	RetryDelay = 5 * time.Second
)

// Syncer handles automatic synchronization of DMR user data from RadioID.net
type Syncer struct {
	repository   *database.DMRUserRepository
	logger       *log.Logger
	syncInterval time.Duration
	httpClient   *http.Client
}

// SyncerConfig holds configuration for the syncer
type SyncerConfig struct {
	SyncInterval time.Duration // How often to sync (default: 24 hours)
	HTTPTimeout  time.Duration // HTTP request timeout (default: 30 seconds)
}

// NewSyncer creates a new RadioID syncer
func NewSyncer(repository *database.DMRUserRepository, logger *log.Logger) *Syncer {
	return NewSyncerWithConfig(repository, logger, SyncerConfig{
		SyncInterval: DefaultSyncInterval,
		HTTPTimeout:  RequestTimeout,
	})
}

// NewSyncerWithConfig creates a new RadioID syncer with custom configuration
func NewSyncerWithConfig(repository *database.DMRUserRepository, logger *log.Logger, config SyncerConfig) *Syncer {
	if config.SyncInterval <= 0 {
		config.SyncInterval = DefaultSyncInterval
	}
	if config.HTTPTimeout <= 0 {
		config.HTTPTimeout = RequestTimeout
	}

	return &Syncer{
		repository:   repository,
		logger:       logger,
		syncInterval: config.SyncInterval,
		httpClient: &http.Client{
			Timeout: config.HTTPTimeout,
		},
	}
}

// Start begins the automatic synchronization process
func (s *Syncer) Start(ctx context.Context) {
	if s.logger != nil {
		s.logger.Printf("RadioID syncer starting (interval: %v)", s.syncInterval)
	}

	// Run initial sync
	if err := s.SyncNow(ctx); err != nil {
		if s.logger != nil {
			s.logger.Printf("Initial RadioID sync failed: %v", err)
		}
	}

	// Set up periodic sync
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Printf("RadioID syncer stopping")
			}
			return

		case <-ticker.C:
			if err := s.SyncNow(ctx); err != nil {
				if s.logger != nil {
					s.logger.Printf("RadioID sync failed: %v", err)
				}
			}
		}
	}
}

// SyncNow performs an immediate synchronization
func (s *Syncer) SyncNow(ctx context.Context) error {
	startTime := time.Now()

	if s.logger != nil {
		s.logger.Printf("Starting RadioID sync from %s", RadioIDURL)
	}

	// Download CSV data with retries
	var csvData io.ReadCloser
	var err error

	for attempt := 1; attempt <= MaxRetries; attempt++ {
		csvData, err = s.downloadCSV(ctx)
		if err == nil {
			break
		}

		if s.logger != nil {
			s.logger.Printf("Download attempt %d/%d failed: %v", attempt, MaxRetries, err)
		}

		if attempt < MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(RetryDelay):
				// Continue to next attempt
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to download after %d attempts: %w", MaxRetries, err)
	}
	defer csvData.Close()

	// Parse and import data
	users, err := s.parseCSV(csvData)
	if err != nil {
		return fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(users) == 0 {
		return fmt.Errorf("no valid users found in CSV")
	}

	// Import to database
	if err := s.repository.UpsertBatch(users); err != nil {
		return fmt.Errorf("failed to import users: %w", err)
	}

	duration := time.Since(startTime)
	if s.logger != nil {
		s.logger.Printf("RadioID sync completed: %d users imported in %v", len(users), duration)
	}

	return nil
}

// downloadCSV downloads the CSV file from RadioID.net
func (s *Syncer) downloadCSV(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", RadioIDURL, nil)
	if err != nil {
		return nil, err
	}

	// Set user agent to identify our application
	req.Header.Set("User-Agent", "YSF2DMR-Go/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// parseCSV parses the RadioID CSV format and returns DMR users
func (s *Syncer) parseCSV(reader io.Reader) ([]database.DMRUser, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields

	// Pre-allocate slice for better performance (RadioID.net has ~100k+ users)
	users := make([]database.DMRUser, 0, 100000)

	lineNumber := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV at line %d: %w", lineNumber, err)
		}

		lineNumber++

		// Skip header row
		if lineNumber == 1 {
			continue
		}

		// Parse record
		user, err := s.parseCSVRecord(record, lineNumber)
		if err != nil {
			// Log but don't fail for invalid records
			if s.logger != nil {
				s.logger.Printf("Skipping invalid record at line %d: %v", lineNumber, err)
			}
			continue
		}

		if user != nil {
			users = append(users, *user)
		}

		// Log progress every 10,000 records
		if s.logger != nil && lineNumber%10000 == 0 {
			s.logger.Printf("Processed %d lines, %d valid users", lineNumber, len(users))
		}
	}

	return users, nil
}

// parseCSVRecord parses a single CSV record into a DMRUser
// Expected format: RADIO_ID,CALLSIGN,FIRST_NAME,LAST_NAME,CITY,STATE,COUNTRY
func (s *Syncer) parseCSVRecord(record []string, lineNumber int) (*database.DMRUser, error) {
	if len(record) < 7 {
		return nil, fmt.Errorf("insufficient fields (got %d, expected 7)", len(record))
	}

	// Parse radio ID
	radioIDStr := strings.TrimSpace(record[0])
	radioID, err := strconv.ParseUint(radioIDStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid radio ID '%s': %w", radioIDStr, err)
	}

	if radioID == 0 {
		return nil, fmt.Errorf("radio ID cannot be zero")
	}

	// Parse callsign
	callsign := strings.TrimSpace(record[1])
	if callsign == "" {
		return nil, fmt.Errorf("callsign cannot be empty")
	}

	// Create user record
	user := &database.DMRUser{
		RadioID:   uint32(radioID),
		Callsign:  strings.ToUpper(callsign),
		FirstName: strings.TrimSpace(record[2]),
		LastName:  strings.TrimSpace(record[3]),
		City:      strings.TrimSpace(record[4]),
		State:     strings.TrimSpace(record[5]),
		Country:   strings.TrimSpace(record[6]),
		UpdatedAt: time.Now(),
	}

	// Validate the user
	if !user.IsValid() {
		return nil, fmt.Errorf("user validation failed")
	}

	return user, nil
}

// GetLastSyncTime returns the timestamp of the most recently updated user
func (s *Syncer) GetLastSyncTime() (time.Time, error) {
	users, err := s.repository.GetRecentlyUpdated(time.Unix(0, 0), 1)
	if err != nil {
		return time.Time{}, err
	}

	if len(users) == 0 {
		return time.Time{}, nil // No users synced yet
	}

	return users[0].UpdatedAt, nil
}

// GetSyncStatistics returns statistics about the sync process
func (s *Syncer) GetSyncStatistics() (map[string]interface{}, error) {
	stats, err := s.repository.GetStatistics()
	if err != nil {
		return nil, err
	}

	// Add syncer-specific statistics
	lastSync, _ := s.GetLastSyncTime()
	stats["last_sync"] = lastSync
	stats["sync_interval"] = s.syncInterval.String()
	stats["next_sync"] = time.Now().Add(s.syncInterval)

	return stats, nil
}