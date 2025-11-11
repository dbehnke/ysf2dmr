package lookup

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DMRLookup provides DMR ID to callsign lookup functionality
// Equivalent to C++ CDMRLookup class with bidirectional lookup and background reload
type DMRLookup struct {
	filename    string
	reloadTime  uint32 // Reload interval in hours (0 = no auto-reload)

	// Bidirectional lookup maps (protected by mutex)
	idToCallsign map[uint32]string // ID -> Callsign
	callsignToID map[string]uint32 // Callsign -> ID (uppercase)

	// Thread safety
	mutex sync.RWMutex

	// Background reload management
	stopChan chan bool
	running  bool
	stopped  bool

	// Statistics
	totalEntries   uint32
	lastReloadTime time.Time
	reloadCount    uint32
	errorCount     uint32

	// Logging
	debugEnabled bool
}

// Special DMR ID constants (matching C++ implementation)
const (
	DMR_ID_ALL     = 0xFFFFFF // Special ID that always returns "ALL" (16777215)
	DMR_ID_UNKNOWN = 0        // Unknown/invalid ID
)

// NewDMRLookup creates a new DMR lookup instance
// Parameters match C++ constructor: filename and reloadTime (hours)
func NewDMRLookup(filename string, reloadTime uint32) *DMRLookup {
	return &DMRLookup{
		filename:       filename,
		reloadTime:     reloadTime,
		idToCallsign:   make(map[uint32]string),
		callsignToID:   make(map[string]uint32),
		stopChan:       make(chan bool, 1),
		running:        false,
		stopped:        false,
		lastReloadTime: time.Time{},
		debugEnabled:   false,
	}
}

// SetDebug enables or disables debug logging
func (d *DMRLookup) SetDebug(enabled bool) {
	d.debugEnabled = enabled
}

// Read loads the DMR ID database from file
// Returns error if file cannot be read or parsed
func (d *DMRLookup) Read() error {
	d.logDebug("Reading DMR ID database from: %s", d.filename)

	file, err := os.Open(d.filename)
	if err != nil {
		d.errorCount++
		return fmt.Errorf("failed to open DMR ID file %s: %v", d.filename, err)
	}
	defer file.Close()

	// Create new maps for atomic replacement
	newIdToCallsign := make(map[uint32]string)
	newCallsignToID := make(map[string]uint32)

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	entriesLoaded := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments (lines starting with #)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse line: ID CALLSIGN
		fields := strings.Fields(line)
		if len(fields) < 2 {
			d.logDebug("Skipping invalid line %d: %s", lineNumber, line)
			continue
		}

		// Parse DMR ID
		id, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			d.logDebug("Invalid DMR ID on line %d: %s", lineNumber, fields[0])
			continue
		}

		// Get callsign and convert to uppercase (matching C++ behavior)
		callsign := strings.ToUpper(fields[1])

		// Validate callsign length (reasonable limit)
		if len(callsign) == 0 || len(callsign) > 20 {
			d.logDebug("Invalid callsign on line %d: %s", lineNumber, callsign)
			continue
		}

		// Store in both maps
		dmrID := uint32(id)
		newIdToCallsign[dmrID] = callsign
		newCallsignToID[callsign] = dmrID

		entriesLoaded++
	}

	if err := scanner.Err(); err != nil {
		d.errorCount++
		return fmt.Errorf("error reading DMR ID file: %v", err)
	}

	// Atomically replace the maps (thread-safe)
	d.mutex.Lock()
	d.idToCallsign = newIdToCallsign
	d.callsignToID = newCallsignToID
	d.totalEntries = uint32(entriesLoaded)
	d.lastReloadTime = time.Now()
	d.mutex.Unlock()

	d.reloadCount++
	d.logDebug("Loaded %d DMR ID entries from %s", entriesLoaded, d.filename)

	return nil
}

// FindCS finds callsign by DMR ID
// Returns the callsign if found, or the ID as a string if not found
// Special case: ID 0xFFFFFFU always returns "ALL" (matching C++ behavior)
func (d *DMRLookup) FindCS(id uint32) string {
	// Special case for ALL ID (matching C++ implementation)
	if id == DMR_ID_ALL {
		return "ALL"
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if callsign, exists := d.idToCallsign[id]; exists {
		return callsign
	}

	// If not found, return the ID as a string (matching C++ behavior)
	return fmt.Sprintf("%d", id)
}

// FindID finds DMR ID by callsign
// Returns the DMR ID if found, or 0 if not found
func (d *DMRLookup) FindID(callsign string) uint32 {
	// Normalize to uppercase for lookup
	upperCallsign := strings.ToUpper(strings.TrimSpace(callsign))

	if len(upperCallsign) == 0 {
		return DMR_ID_UNKNOWN
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if id, exists := d.callsignToID[upperCallsign]; exists {
		return id
	}

	return DMR_ID_UNKNOWN
}

// Exists checks if DMR ID exists in the database
// Returns true if the ID is found, false otherwise
func (d *DMRLookup) Exists(id uint32) bool {
	// Special case for ALL ID
	if id == DMR_ID_ALL {
		return true
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	_, exists := d.idToCallsign[id]
	return exists
}

// Start begins the background reload process if reloadTime > 0
// This method starts a goroutine that reloads the database periodically
func (d *DMRLookup) Start() error {
	// Initial load
	if err := d.Read(); err != nil {
		return fmt.Errorf("initial DMR ID database load failed: %v", err)
	}

	// Start background reload if reloadTime is configured
	if d.reloadTime > 0 {
		d.mutex.Lock()
		if !d.running && !d.stopped {
			d.running = true
			go d.reloadLoop()
			d.logDebug("Started DMR ID background reload with %d hour interval", d.reloadTime)
		}
		d.mutex.Unlock()
	} else {
		d.logDebug("DMR ID background reload disabled (reloadTime = 0)")
	}

	return nil
}

// Stop stops the background reload goroutine
// Blocks until the goroutine has fully stopped
func (d *DMRLookup) Stop() {
	d.mutex.Lock()
	if d.running && !d.stopped {
		d.stopped = true
		d.mutex.Unlock()

		// Signal stop and wait for acknowledgment
		select {
		case d.stopChan <- true:
		case <-time.After(5 * time.Second):
			log.Printf("Warning: DMR lookup stop timeout")
		}

		// Wait for goroutine to finish
		d.mutex.Lock()
		d.running = false
		d.mutex.Unlock()

		d.logDebug("DMR ID background reload stopped")
	} else {
		d.mutex.Unlock()
	}
}

// reloadLoop runs the background reload timer (private goroutine)
func (d *DMRLookup) reloadLoop() {
	defer func() {
		d.mutex.Lock()
		d.running = false
		d.mutex.Unlock()
	}()

	// Calculate reload interval
	reloadInterval := time.Duration(d.reloadTime) * time.Hour
	ticker := time.NewTicker(reloadInterval)
	defer ticker.Stop()

	d.logDebug("DMR ID reload loop started, interval: %v", reloadInterval)

	for {
		select {
		case <-d.stopChan:
			d.logDebug("DMR ID reload loop stopping")
			return

		case <-ticker.C:
			d.logDebug("DMR ID automatic reload triggered")
			if err := d.Read(); err != nil {
				log.Printf("DMR ID automatic reload failed: %v", err)
				d.errorCount++
			} else {
				d.logDebug("DMR ID automatic reload completed successfully")
			}
		}
	}
}

// GetStats returns statistics about the DMR lookup
func (d *DMRLookup) GetStats() (totalEntries, reloadCount, errorCount uint32, lastReload time.Time) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.totalEntries, d.reloadCount, d.errorCount, d.lastReloadTime
}

// GetEntryCount returns the current number of entries in the database
func (d *DMRLookup) GetEntryCount() uint32 {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.totalEntries
}

// IsRunning returns true if the background reload is currently running
func (d *DMRLookup) IsRunning() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.running
}

// ForceReload manually triggers a reload of the database
func (d *DMRLookup) ForceReload() error {
	d.logDebug("Manual DMR ID reload triggered")
	return d.Read()
}

// GetAllCallsigns returns a copy of all callsigns in the database
// This is useful for testing and debugging
func (d *DMRLookup) GetAllCallsigns() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	callsigns := make([]string, 0, len(d.callsignToID))
	for callsign := range d.callsignToID {
		callsigns = append(callsigns, callsign)
	}

	return callsigns
}

// GetAllIDs returns a copy of all DMR IDs in the database
// This is useful for testing and debugging
func (d *DMRLookup) GetAllIDs() []uint32 {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	ids := make([]uint32, 0, len(d.idToCallsign))
	for id := range d.idToCallsign {
		ids = append(ids, id)
	}

	return ids
}

// logDebug logs debug messages if debug is enabled
func (d *DMRLookup) logDebug(format string, args ...interface{}) {
	if d.debugEnabled {
		log.Printf("DMRLookup: "+format, args...)
	}
}

// ValidateFile checks if the DMR ID file exists and is readable
func (d *DMRLookup) ValidateFile() error {
	if d.filename == "" {
		return fmt.Errorf("DMR ID filename is empty")
	}

	info, err := os.Stat(d.filename)
	if err != nil {
		return fmt.Errorf("DMR ID file not accessible: %v", err)
	}

	if info.IsDir() {
		return fmt.Errorf("DMR ID path is a directory, not a file: %s", d.filename)
	}

	if info.Size() == 0 {
		return fmt.Errorf("DMR ID file is empty: %s", d.filename)
	}

	return nil
}

// GetFilename returns the currently configured filename
func (d *DMRLookup) GetFilename() string {
	return d.filename
}

// GetReloadTime returns the reload interval in hours
func (d *DMRLookup) GetReloadTime() uint32 {
	return d.reloadTime
}

// SetReloadTime updates the reload interval (takes effect on next Start)
func (d *DMRLookup) SetReloadTime(hours uint32) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.reloadTime = hours
}