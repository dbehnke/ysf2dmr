package database

import (
	"fmt"
	"strings"
	"time"
)

// DMRUser represents a DMR radio user record
type DMRUser struct {
	RadioID   uint32    `gorm:"primarykey;not null" json:"radio_id"`
	Callsign  string    `gorm:"index;size:20" json:"callsign"`
	FirstName string    `gorm:"size:50" json:"first_name"`
	LastName  string    `gorm:"size:50" json:"last_name"`
	City      string    `gorm:"size:50" json:"city"`
	State     string    `gorm:"size:50" json:"state"`
	Country   string    `gorm:"size:50" json:"country"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (DMRUser) TableName() string {
	return "dmr_users"
}

// FullName returns the formatted full name
func (u DMRUser) FullName() string {
	parts := []string{}
	if u.FirstName != "" {
		parts = append(parts, u.FirstName)
	}
	if u.LastName != "" {
		parts = append(parts, u.LastName)
	}
	return strings.Join(parts, " ")
}

// Location returns the formatted location string
func (u DMRUser) Location() string {
	parts := []string{}
	if u.City != "" {
		parts = append(parts, u.City)
	}
	if u.State != "" {
		if len(parts) > 0 {
			parts = append(parts, u.State)
		} else {
			parts = append(parts, u.State)
		}
	}
	if u.Country != "" {
		if len(parts) > 0 {
			parts = append(parts, u.Country)
		} else {
			parts = append(parts, u.Country)
		}
	}
	return strings.Join(parts, ", ")
}

// String returns a formatted string representation
func (u DMRUser) String() string {
	fullName := u.FullName()
	location := u.Location()

	result := fmt.Sprintf("%s (%d)", u.Callsign, u.RadioID)

	if fullName != "" {
		result += fmt.Sprintf(" - %s", fullName)
	}

	if location != "" {
		result += fmt.Sprintf(" [%s]", location)
	}

	return result
}

// IsValid checks if the user record has required fields
func (u DMRUser) IsValid() bool {
	return u.RadioID > 0 && u.Callsign != ""
}

// SanitizeCallsign cleans up the callsign format
func (u *DMRUser) SanitizeCallsign() {
	u.Callsign = strings.ToUpper(strings.TrimSpace(u.Callsign))
}

// SanitizeFields cleans up all user fields
func (u *DMRUser) SanitizeFields() {
	u.SanitizeCallsign()
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)
	u.City = strings.TrimSpace(u.City)
	u.State = strings.TrimSpace(u.State)
	u.Country = strings.TrimSpace(u.Country)
}