package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DMRUserRepository provides database operations for DMR users
type DMRUserRepository struct {
	db *gorm.DB
}

// NewDMRUserRepository creates a new repository instance
func NewDMRUserRepository(db *gorm.DB) *DMRUserRepository {
	return &DMRUserRepository{db: db}
}

// GetByRadioID finds a user by their DMR radio ID
func (r *DMRUserRepository) GetByRadioID(radioID uint32) (*DMRUser, error) {
	var user DMRUser
	err := r.db.Where("radio_id = ?", radioID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByCallsign finds a user by their callsign
func (r *DMRUserRepository) GetByCallsign(callsign string) (*DMRUser, error) {
	var user DMRUser
	err := r.db.Where("callsign = ?", callsign).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Upsert creates or updates a single DMR user
func (r *DMRUserRepository) Upsert(user *DMRUser) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}

	if !user.IsValid() {
		return fmt.Errorf("user is not valid: radio_id=%d, callsign=%s", user.RadioID, user.Callsign)
	}

	// Sanitize fields
	user.SanitizeFields()
	user.UpdatedAt = time.Now()

	// Use GORM's upsert functionality
	return r.db.Save(user).Error
}

// UpsertBatch creates or updates multiple DMR users in a transaction
func (r *DMRUserRepository) UpsertBatch(users []DMRUser) error {
	if len(users) == 0 {
		return nil
	}

	// Process in batches to avoid memory issues
	const batchSize = 1000

	for i := 0; i < len(users); i += batchSize {
		end := i + batchSize
		if end > len(users) {
			end = len(users)
		}

		batch := users[i:end]

		// Sanitize and validate each user in the batch
		validUsers := make([]DMRUser, 0, len(batch))
		for _, user := range batch {
			user.SanitizeFields()
			if user.IsValid() {
				user.UpdatedAt = time.Now()
				validUsers = append(validUsers, user)
			}
		}

		if len(validUsers) == 0 {
			continue
		}

		// Execute batch upsert in transaction
		err := r.db.Transaction(func(tx *gorm.DB) error {
			for _, user := range validUsers {
				if err := tx.Save(&user).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("batch upsert failed at batch starting at index %d: %w", i, err)
		}
	}

	return nil
}

// Count returns the total number of DMR users in the database
func (r *DMRUserRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&DMRUser{}).Count(&count).Error
	return count, err
}

// DeleteAll removes all DMR users from the database
func (r *DMRUserRepository) DeleteAll() error {
	return r.db.Where("1 = 1").Delete(&DMRUser{}).Error
}

// GetRecentlyUpdated returns users updated after the specified time
func (r *DMRUserRepository) GetRecentlyUpdated(since time.Time, limit int) ([]DMRUser, error) {
	var users []DMRUser
	err := r.db.Where("updated_at > ?", since).
		Order("updated_at DESC").
		Limit(limit).
		Find(&users).Error
	return users, err
}

// FindByCallsignPattern searches for callsigns matching a pattern
func (r *DMRUserRepository) FindByCallsignPattern(pattern string, limit int) ([]DMRUser, error) {
	var users []DMRUser
	err := r.db.Where("callsign LIKE ?", pattern+"%").
		Order("callsign ASC").
		Limit(limit).
		Find(&users).Error
	return users, err
}

// GetStatistics returns basic database statistics
func (r *DMRUserRepository) GetStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total count
	count, err := r.Count()
	if err != nil {
		return nil, err
	}
	stats["total_users"] = count

	// Most recent update
	var latestUser DMRUser
	err = r.db.Order("updated_at DESC").First(&latestUser).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if err != gorm.ErrRecordNotFound {
		stats["last_updated"] = latestUser.UpdatedAt
	}

	// Country distribution (top 10)
	var countryStats []struct {
		Country string `json:"country"`
		Count   int    `json:"count"`
	}
	err = r.db.Model(&DMRUser{}).
		Select("country, COUNT(*) as count").
		Where("country != ''").
		Group("country").
		Order("count DESC").
		Limit(10).
		Find(&countryStats).Error
	if err != nil {
		return nil, err
	}
	stats["top_countries"] = countryStats

	return stats, nil
}

// HealthCheck verifies the repository is working correctly
func (r *DMRUserRepository) HealthCheck() error {
	// Try a simple query
	var count int64
	return r.db.Model(&DMRUser{}).Count(&count).Error
}