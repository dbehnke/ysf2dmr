package lookup

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/database"
	"gorm.io/gorm"
)

// DMRDatabaseAdapter provides a database-backed implementation of the DMRLookup interface
// This allows drop-in replacement of the file-based lookup with automatic RadioID.net sync
type DMRDatabaseAdapter struct {
	repository   *database.DMRUserRepository
	debugEnabled bool

	// Statistics tracking (similar to original DMRLookup)
	mutex        sync.RWMutex
	lookupCount  uint32
	hitCount     uint32
	missCount    uint32
	errorCount   uint32
	lastAccess   time.Time

	// Cache for performance (optional)
	enableCache  bool
	cacheSize    int
	idCache      map[uint32]string // Recent ID->Callsign lookups
	callsignCache map[string]uint32 // Recent Callsign->ID lookups
	cacheExpiry  time.Duration
	lastClearTime time.Time
}

// DMRDatabaseAdapterConfig holds configuration options for the database adapter
type DMRDatabaseAdapterConfig struct {
	EnableCache   bool          // Enable in-memory cache for frequently accessed lookups
	CacheSize     int           // Maximum cache size (default: 1000)
	CacheExpiry   time.Duration // Cache expiry time (default: 5 minutes)
}

// NewDMRDatabaseAdapter creates a new database-backed DMR lookup adapter
func NewDMRDatabaseAdapter(repository *database.DMRUserRepository) *DMRDatabaseAdapter {
	return NewDMRDatabaseAdapterWithConfig(repository, DMRDatabaseAdapterConfig{
		EnableCache: true,
		CacheSize:   1000,
		CacheExpiry: 5 * time.Minute,
	})
}

// NewDMRDatabaseAdapterWithConfig creates a new database adapter with custom configuration
func NewDMRDatabaseAdapterWithConfig(repository *database.DMRUserRepository, config DMRDatabaseAdapterConfig) *DMRDatabaseAdapter {
	adapter := &DMRDatabaseAdapter{
		repository:    repository,
		debugEnabled:  false,
		enableCache:   config.EnableCache,
		cacheSize:     config.CacheSize,
		cacheExpiry:   config.CacheExpiry,
		lastClearTime: time.Now(),
	}

	if adapter.enableCache {
		adapter.idCache = make(map[uint32]string)
		adapter.callsignCache = make(map[string]uint32)
	}

	return adapter
}

// SetDebug enables or disables debug logging (compatible with original interface)
func (d *DMRDatabaseAdapter) SetDebug(enabled bool) {
	d.debugEnabled = enabled
}

// FindCS finds callsign by DMR ID (compatible with original DMRLookup interface)
// Returns the callsign if found, or the ID as a string if not found
// Special case: ID 0xFFFFFFU always returns "ALL" (matching original behavior)
func (d *DMRDatabaseAdapter) FindCS(id uint32) string {
	d.updateAccessStats()

	// Special case for ALL ID (matching original implementation)
	if id == DMR_ID_ALL {
		return "ALL"
	}

	// Check cache first if enabled
	if d.enableCache {
		if callsign, found := d.getCachedCallsign(id); found {
			d.recordHit()
			return callsign
		}
	}

	// Query database
	user, err := d.repository.GetByRadioID(id)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			d.recordError()
			d.logDebug("Database error looking up ID %d: %v", id, err)
		} else {
			d.recordMiss()
		}
		// If not found, return the ID as a string (matching original behavior)
		return fmt.Sprintf("%d", id)
	}

	// Cache the result if caching is enabled
	if d.enableCache {
		d.cacheCallsign(id, user.Callsign)
	}

	d.recordHit()
	return user.Callsign
}

// FindID finds DMR ID by callsign (compatible with original DMRLookup interface)
// Returns the DMR ID if found, or 0 if not found
func (d *DMRDatabaseAdapter) FindID(callsign string) uint32 {
	d.updateAccessStats()

	// Normalize to uppercase for lookup (matching original behavior)
	upperCallsign := strings.ToUpper(strings.TrimSpace(callsign))

	if len(upperCallsign) == 0 {
		return DMR_ID_UNKNOWN
	}

	// Check cache first if enabled
	if d.enableCache {
		if id, found := d.getCachedID(upperCallsign); found {
			d.recordHit()
			return id
		}
	}

	// Query database
	user, err := d.repository.GetByCallsign(upperCallsign)
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			d.recordError()
			d.logDebug("Database error looking up callsign %s: %v", upperCallsign, err)
		} else {
			d.recordMiss()
		}
		return DMR_ID_UNKNOWN
	}

	// Cache the result if caching is enabled
	if d.enableCache {
		d.cacheID(upperCallsign, user.RadioID)
	}

	d.recordHit()
	return user.RadioID
}

// Exists checks if DMR ID exists in the database (compatible with original interface)
func (d *DMRDatabaseAdapter) Exists(id uint32) bool {
	// Special case for ALL ID
	if id == DMR_ID_ALL {
		return true
	}

	// Try to find the callsign; if we get back the ID as string, it doesn't exist
	callsign := d.FindCS(id)
	idAsString := fmt.Sprintf("%d", id)
	return callsign != idAsString
}

// GetStats returns statistics about the DMR lookup (compatible with original interface)
// Note: reloadCount and lastReload are not applicable for database adapter
func (d *DMRDatabaseAdapter) GetStats() (totalEntries, reloadCount, errorCount uint32, lastReload time.Time) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Get total entries from database
	count, err := d.repository.Count()
	if err != nil {
		d.logDebug("Error getting database count: %v", err)
		count = 0
	}

	// Return stats compatible with original interface
	// reloadCount is not applicable (database auto-syncs), lastReload is last access time
	return uint32(count), 0, d.errorCount, d.lastAccess
}

// GetEntryCount returns the current number of entries in the database (compatible interface)
func (d *DMRDatabaseAdapter) GetEntryCount() uint32 {
	count, err := d.repository.Count()
	if err != nil {
		d.logDebug("Error getting database count: %v", err)
		return 0
	}
	return uint32(count)
}

// ForceReload is a no-op for database adapter (database syncs automatically)
// Kept for interface compatibility
func (d *DMRDatabaseAdapter) ForceReload() error {
	d.logDebug("ForceReload called on database adapter (no-op - database syncs automatically)")

	// Clear cache if enabled to force fresh lookups
	if d.enableCache {
		d.clearCache()
	}

	return nil
}

// Start performs initial validation (compatible with original interface)
// For database adapter, this validates the database connection
func (d *DMRDatabaseAdapter) Start() error {
	// Validate database connection
	err := d.repository.HealthCheck()
	if err != nil {
		return fmt.Errorf("database connection check failed: %v", err)
	}

	count, err := d.repository.Count()
	if err != nil {
		return fmt.Errorf("failed to get initial database count: %v", err)
	}

	d.logDebug("Database adapter started with %d entries", count)
	return nil
}

// Stop is a no-op for database adapter (no background processes to stop)
// Kept for interface compatibility
func (d *DMRDatabaseAdapter) Stop() {
	d.logDebug("Stop called on database adapter (no-op)")
	if d.enableCache {
		d.clearCache()
	}
}

// IsRunning always returns true for database adapter (no concept of "stopped")
// Kept for interface compatibility
func (d *DMRDatabaseAdapter) IsRunning() bool {
	return true
}

// GetAllCallsigns returns all callsigns in the database (compatible interface)
// Note: This can be memory-intensive for large databases
func (d *DMRDatabaseAdapter) GetAllCallsigns() []string {
	// This is potentially expensive for large databases
	// For now, implement a simple version with a reasonable limit
	const maxResults = 10000

	users, err := d.repository.FindByCallsignPattern("", maxResults)
	if err != nil {
		d.logDebug("Error getting all callsigns: %v", err)
		return []string{}
	}

	callsigns := make([]string, len(users))
	for i, user := range users {
		callsigns[i] = user.Callsign
	}

	return callsigns
}

// GetAllIDs returns all DMR IDs in the database (compatible interface)
// Note: This can be memory-intensive for large databases
func (d *DMRDatabaseAdapter) GetAllIDs() []uint32 {
	// This is potentially expensive for large databases
	// For now, implement a simple version with a reasonable limit
	const maxResults = 10000

	users, err := d.repository.FindByCallsignPattern("", maxResults)
	if err != nil {
		d.logDebug("Error getting all IDs: %v", err)
		return []uint32{}
	}

	ids := make([]uint32, len(users))
	for i, user := range users {
		ids[i] = user.RadioID
	}

	return ids
}

// Extended methods specific to database adapter

// GetUserInfo returns full user information if available (database-specific feature)
func (d *DMRDatabaseAdapter) GetUserInfo(id uint32) (*database.DMRUser, error) {
	return d.repository.GetByRadioID(id)
}

// GetUserInfoByCallsign returns full user information by callsign (database-specific feature)
func (d *DMRDatabaseAdapter) GetUserInfoByCallsign(callsign string) (*database.DMRUser, error) {
	upperCallsign := strings.ToUpper(strings.TrimSpace(callsign))
	return d.repository.GetByCallsign(upperCallsign)
}

// GetDatabaseStatistics returns detailed database statistics
func (d *DMRDatabaseAdapter) GetDatabaseStatistics() (map[string]interface{}, error) {
	dbStats, err := d.repository.GetStatistics()
	if err != nil {
		return nil, err
	}

	// Add adapter-specific statistics
	d.mutex.RLock()
	adapterStats := map[string]interface{}{
		"lookup_count": d.lookupCount,
		"hit_count":    d.hitCount,
		"miss_count":   d.missCount,
		"error_count":  d.errorCount,
		"last_access":  d.lastAccess,
	}
	d.mutex.RUnlock()

	if d.enableCache {
		adapterStats["cache_enabled"] = true
		adapterStats["cache_size"] = len(d.idCache) + len(d.callsignCache)
		adapterStats["cache_expiry"] = d.cacheExpiry.String()
	} else {
		adapterStats["cache_enabled"] = false
	}

	// Combine database and adapter stats
	result := make(map[string]interface{})
	for k, v := range dbStats {
		result[k] = v
	}
	for k, v := range adapterStats {
		result[k] = v
	}

	return result, nil
}

// Cache management methods (private)

func (d *DMRDatabaseAdapter) getCachedCallsign(id uint32) (string, bool) {
	if !d.enableCache {
		return "", false
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	d.clearExpiredCache()

	callsign, exists := d.idCache[id]
	return callsign, exists
}

func (d *DMRDatabaseAdapter) getCachedID(callsign string) (uint32, bool) {
	if !d.enableCache {
		return 0, false
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	d.clearExpiredCache()

	id, exists := d.callsignCache[callsign]
	return id, exists
}

func (d *DMRDatabaseAdapter) cacheCallsign(id uint32, callsign string) {
	if !d.enableCache {
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.clearExpiredCache()

	// Limit cache size
	if len(d.idCache) >= d.cacheSize {
		d.clearOldestEntries()
	}

	d.idCache[id] = callsign
}

func (d *DMRDatabaseAdapter) cacheID(callsign string, id uint32) {
	if !d.enableCache {
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.clearExpiredCache()

	// Limit cache size
	if len(d.callsignCache) >= d.cacheSize {
		d.clearOldestEntries()
	}

	d.callsignCache[callsign] = id
}

func (d *DMRDatabaseAdapter) clearCache() {
	if !d.enableCache {
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.idCache = make(map[uint32]string)
	d.callsignCache = make(map[string]uint32)
	d.lastClearTime = time.Now()
}

func (d *DMRDatabaseAdapter) clearExpiredCache() {
	// Check if cache should be cleared due to expiry
	if time.Since(d.lastClearTime) > d.cacheExpiry {
		d.idCache = make(map[uint32]string)
		d.callsignCache = make(map[string]uint32)
		d.lastClearTime = time.Now()
	}
}

func (d *DMRDatabaseAdapter) clearOldestEntries() {
	// Simple cache eviction: clear half the cache
	// More sophisticated LRU could be implemented if needed
	for id := range d.idCache {
		delete(d.idCache, id)
		if len(d.idCache) <= d.cacheSize/2 {
			break
		}
	}

	for callsign := range d.callsignCache {
		delete(d.callsignCache, callsign)
		if len(d.callsignCache) <= d.cacheSize/2 {
			break
		}
	}
}

// Statistics tracking methods (private)

func (d *DMRDatabaseAdapter) updateAccessStats() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.lookupCount++
	d.lastAccess = time.Now()
}

func (d *DMRDatabaseAdapter) recordHit() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.hitCount++
}

func (d *DMRDatabaseAdapter) recordMiss() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.missCount++
}

func (d *DMRDatabaseAdapter) recordError() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.errorCount++
}

// logDebug logs debug messages if debug is enabled (compatible with original interface)
func (d *DMRDatabaseAdapter) logDebug(format string, args ...interface{}) {
	if d.debugEnabled {
		log.Printf("DMRDatabaseAdapter: "+format, args...)
	}
}

// Interface compatibility validation (these methods exist in original DMRLookup but not used in adapter)

// ValidateFile is not applicable for database adapter but kept for compatibility
func (d *DMRDatabaseAdapter) ValidateFile() error {
	return fmt.Errorf("ValidateFile not applicable for database adapter")
}

// GetFilename is not applicable for database adapter but kept for compatibility
func (d *DMRDatabaseAdapter) GetFilename() string {
	return "(database)"
}

// GetReloadTime is not applicable for database adapter but kept for compatibility
func (d *DMRDatabaseAdapter) GetReloadTime() uint32 {
	return 0 // Auto-sync, no manual reload interval
}

// SetReloadTime is not applicable for database adapter but kept for compatibility
func (d *DMRDatabaseAdapter) SetReloadTime(hours uint32) {
	d.logDebug("SetReloadTime(%d) ignored - database adapter syncs automatically", hours)
}

// Read is not applicable for database adapter but kept for compatibility
func (d *DMRDatabaseAdapter) Read() error {
	d.logDebug("Read() called on database adapter (no-op - data comes from database)")
	return nil
}