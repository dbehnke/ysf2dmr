package lookup

import "time"

// DMRLookupInterface defines the interface for DMR ID to callsign lookup services
// This interface can be implemented by both file-based and database-backed lookup services
type DMRLookupInterface interface {
	// Core lookup methods
	FindCS(id uint32) string              // Find callsign by DMR ID
	FindID(callsign string) uint32        // Find DMR ID by callsign
	Exists(id uint32) bool                // Check if DMR ID exists

	// Lifecycle management
	Start() error                         // Initialize the lookup service
	Stop()                               // Stop the lookup service
	IsRunning() bool                     // Check if service is running

	// Statistics and debugging
	GetStats() (totalEntries, reloadCount, errorCount uint32, lastReload time.Time)
	GetEntryCount() uint32               // Get current number of entries
	SetDebug(enabled bool)               // Enable/disable debug logging

	// Manual operations
	ForceReload() error                  // Force reload/sync of data

	// Data access (for testing and debugging)
	GetAllCallsigns() []string           // Get all callsigns (may be expensive)
	GetAllIDs() []uint32                 // Get all IDs (may be expensive)
}