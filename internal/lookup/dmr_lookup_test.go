package lookup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDMRLookupCreation tests basic creation and configuration
func TestDMRLookupCreation(t *testing.T) {
	lookup := NewDMRLookup("test.dat", 24)

	if lookup == nil {
		t.Fatal("NewDMRLookup returned nil")
	}

	if lookup.GetFilename() != "test.dat" {
		t.Errorf("Expected filename 'test.dat', got '%s'", lookup.GetFilename())
	}

	if lookup.GetReloadTime() != 24 {
		t.Errorf("Expected reload time 24, got %d", lookup.GetReloadTime())
	}

	if lookup.IsRunning() {
		t.Error("Lookup should not be running initially")
	}

	if lookup.GetEntryCount() != 0 {
		t.Error("Initial entry count should be 0")
	}
}

// TestDMRLookupFileValidation tests file validation
func TestDMRLookupFileValidation(t *testing.T) {
	// Test with non-existent file
	lookup := NewDMRLookup("nonexistent.dat", 0)
	err := lookup.ValidateFile()
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test with empty filename
	lookup = NewDMRLookup("", 0)
	err = lookup.ValidateFile()
	if err == nil {
		t.Error("Expected error for empty filename")
	}

	// Test with directory instead of file
	tempDir := t.TempDir()
	lookup = NewDMRLookup(tempDir, 0)
	err = lookup.ValidateFile()
	if err == nil {
		t.Error("Expected error for directory instead of file")
	}

	// Test with empty file
	emptyFile := filepath.Join(tempDir, "empty.dat")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	lookup = NewDMRLookup(emptyFile, 0)
	err = lookup.ValidateFile()
	if err == nil {
		t.Error("Expected error for empty file")
	}

	// Test with valid file
	validFile := createTestDMRFile(t, tempDir, getTestDMRData())
	lookup = NewDMRLookup(validFile, 0)
	err = lookup.ValidateFile()
	if err != nil {
		t.Errorf("Unexpected error for valid file: %v", err)
	}
}

// TestDMRLookupBasicOperations tests file reading and basic lookup operations
func TestDMRLookupBasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestDMRFile(t, tempDir, getTestDMRData())

	lookup := NewDMRLookup(testFile, 0)
	lookup.SetDebug(true)

	// Test initial read
	err := lookup.Read()
	if err != nil {
		t.Fatalf("Failed to read DMR file: %v", err)
	}

	expectedEntries := 5 // From test data
	if count := lookup.GetEntryCount(); count != uint32(expectedEntries) {
		t.Errorf("Expected %d entries, got %d", expectedEntries, count)
	}

	// Test ID to callsign lookup
	testCases := []struct {
		id       uint32
		expected string
	}{
		{3113, "G4KLX"},
		{4, "N0CALL"},
		{9990, "TG9990"},
		{16777215, "ALL"}, // Special ALL ID
		{999999, "999999"}, // Non-existent ID should return ID as string
	}

	for _, tc := range testCases {
		result := lookup.FindCS(tc.id)
		if result != tc.expected {
			t.Errorf("FindCS(%d): expected '%s', got '%s'", tc.id, tc.expected, result)
		}
	}

	// Test callsign to ID lookup
	callsignTestCases := []struct {
		callsign string
		expected uint32
	}{
		{"G4KLX", 3113},
		{"g4klx", 3113}, // Case insensitive
		{"N0CALL", 4},
		{"TG9990", 9990},
		{"NONEXISTENT", 0}, // Non-existent callsign should return 0
		{"", 0},            // Empty callsign should return 0
	}

	for _, tc := range callsignTestCases {
		result := lookup.FindID(tc.callsign)
		if result != tc.expected {
			t.Errorf("FindID('%s'): expected %d, got %d", tc.callsign, tc.expected, result)
		}
	}

	// Test Exists function
	existsTestCases := []struct {
		id     uint32
		exists bool
	}{
		{3113, true},
		{4, true},
		{16777215, true}, // Special ALL ID
		{999999, false},  // Non-existent ID
	}

	for _, tc := range existsTestCases {
		result := lookup.Exists(tc.id)
		if result != tc.exists {
			t.Errorf("Exists(%d): expected %t, got %t", tc.id, tc.exists, result)
		}
	}
}

// TestDMRLookupFileFormats tests various file format scenarios
func TestDMRLookupFileFormats(t *testing.T) {
	tempDir := t.TempDir()

	// Test file with comments and whitespace
	testData := `# This is a comment
# Another comment
3113 G4KLX
	4	N0CALL   # Inline comment
9990   TG9990

# Empty line above and below
16777215 ALL`

	testFile := createTestDMRFile(t, tempDir, testData)
	lookup := NewDMRLookup(testFile, 0)

	err := lookup.Read()
	if err != nil {
		t.Fatalf("Failed to read DMR file: %v", err)
	}

	if count := lookup.GetEntryCount(); count != 4 {
		t.Errorf("Expected 4 entries, got %d", count)
	}

	// Test malformed file
	malformedData := `3113 G4KLX
invalid_line_here
4 N0CALL
just_one_field
9990 TG9990 extra_field`

	testFile2 := createTestDMRFile(t, tempDir, malformedData)
	lookup2 := NewDMRLookup(testFile2, 0)

	err = lookup2.Read()
	if err != nil {
		t.Fatalf("Failed to read malformed DMR file: %v", err)
	}

	// Should only load valid entries (3113, 4, 9990)
	if count := lookup2.GetEntryCount(); count != 3 {
		t.Errorf("Expected 3 valid entries from malformed file, got %d", count)
	}
}

// TestDMRLookupStartStop tests the start/stop functionality
func TestDMRLookupStartStop(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestDMRFile(t, tempDir, getTestDMRData())

	// Test without auto-reload
	lookup := NewDMRLookup(testFile, 0)
	err := lookup.Start()
	if err != nil {
		t.Fatalf("Failed to start lookup: %v", err)
	}

	if lookup.GetEntryCount() == 0 {
		t.Error("Expected entries to be loaded after start")
	}

	if lookup.IsRunning() {
		t.Error("Lookup should not be running without auto-reload")
	}

	lookup.Stop() // Should be safe even if not running

	// Test with auto-reload
	lookup2 := NewDMRLookup(testFile, 1) // 1 hour reload
	err = lookup2.Start()
	if err != nil {
		t.Fatalf("Failed to start lookup with auto-reload: %v", err)
	}

	if !lookup2.IsRunning() {
		t.Error("Lookup should be running with auto-reload enabled")
	}

	lookup2.Stop()

	// Give goroutine time to stop
	time.Sleep(100 * time.Millisecond)

	if lookup2.IsRunning() {
		t.Error("Lookup should not be running after stop")
	}
}

// TestDMRLookupStatistics tests statistics tracking
func TestDMRLookupStatistics(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestDMRFile(t, tempDir, getTestDMRData())

	lookup := NewDMRLookup(testFile, 0)

	// Check initial stats
	totalEntries, reloadCount, errorCount, lastReload := lookup.GetStats()
	if totalEntries != 0 || reloadCount != 0 || errorCount != 0 || !lastReload.IsZero() {
		t.Error("Initial stats should be zero/empty")
	}

	// Load file
	err := lookup.Read()
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// Check stats after load
	totalEntries, reloadCount, errorCount, lastReload = lookup.GetStats()
	if totalEntries != 5 {
		t.Errorf("Expected 5 total entries, got %d", totalEntries)
	}
	if reloadCount != 1 {
		t.Errorf("Expected 1 reload, got %d", reloadCount)
	}
	if errorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}
	if lastReload.IsZero() {
		t.Error("Last reload time should be set")
	}

	// Force reload
	err = lookup.ForceReload()
	if err != nil {
		t.Fatalf("Force reload failed: %v", err)
	}

	// Check stats after force reload
	_, reloadCount2, _, _ := lookup.GetStats()
	if reloadCount2 != 2 {
		t.Errorf("Expected 2 reloads after force reload, got %d", reloadCount2)
	}
}

// TestDMRLookupErrorHandling tests error scenarios
func TestDMRLookupErrorHandling(t *testing.T) {
	// Test with non-existent file
	lookup := NewDMRLookup("nonexistent.dat", 0)
	err := lookup.Read()
	if err == nil {
		t.Error("Expected error reading non-existent file")
	}

	_, _, errorCount, _ := lookup.GetStats()
	if errorCount == 0 {
		t.Error("Error count should be incremented after read failure")
	}

	// Test start with bad file
	err = lookup.Start()
	if err == nil {
		t.Error("Expected error starting with non-existent file")
	}
}

// TestDMRLookupConcurrency tests thread safety
func TestDMRLookupConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestDMRFile(t, tempDir, getTestDMRData())

	lookup := NewDMRLookup(testFile, 0)
	err := lookup.Read()
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// Run concurrent lookups
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()

			// Perform random lookups
			ids := []uint32{3113, 4, 9990, 16777215, 999999}
			for j := 0; j < 100; j++ {
				id := ids[j%len(ids)]
				_ = lookup.FindCS(id)
				_ = lookup.Exists(id)
			}

			callsigns := []string{"G4KLX", "N0CALL", "TG9990", "NONEXISTENT"}
			for j := 0; j < 100; j++ {
				cs := callsigns[j%len(callsigns)]
				_ = lookup.FindID(cs)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timeout")
		}
	}
}

// TestDMRLookupReloadTime tests reload time configuration
func TestDMRLookupReloadTime(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestDMRFile(t, tempDir, getTestDMRData())

	lookup := NewDMRLookup(testFile, 24)

	if lookup.GetReloadTime() != 24 {
		t.Errorf("Expected reload time 24, got %d", lookup.GetReloadTime())
	}

	lookup.SetReloadTime(12)
	if lookup.GetReloadTime() != 12 {
		t.Errorf("Expected reload time 12 after set, got %d", lookup.GetReloadTime())
	}
}

// TestDMRLookupHelperMethods tests utility methods
func TestDMRLookupHelperMethods(t *testing.T) {
	tempDir := t.TempDir()
	testFile := createTestDMRFile(t, tempDir, getTestDMRData())

	lookup := NewDMRLookup(testFile, 0)
	err := lookup.Read()
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// Test GetAllCallsigns
	callsigns := lookup.GetAllCallsigns()
	if len(callsigns) != 5 {
		t.Errorf("Expected 5 callsigns, got %d", len(callsigns))
	}

	expectedCallsigns := map[string]bool{
		"G4KLX": true, "N0CALL": true, "TG9990": true, "TESTUSER": true, "ALL": true,
	}

	for _, cs := range callsigns {
		if !expectedCallsigns[cs] {
			t.Errorf("Unexpected callsign: %s", cs)
		}
	}

	// Test GetAllIDs
	ids := lookup.GetAllIDs()
	if len(ids) != 5 {
		t.Errorf("Expected 5 IDs, got %d", len(ids))
	}

	expectedIDs := map[uint32]bool{
		3113: true, 4: true, 9990: true, 12345: true, 16777215: true,
	}

	for _, id := range ids {
		if !expectedIDs[id] {
			t.Errorf("Unexpected ID: %d", id)
		}
	}
}

// TestDMRLookupSpecialCases tests edge cases and special scenarios
func TestDMRLookupSpecialCases(t *testing.T) {
	tempDir := t.TempDir()

	// Test with file containing only comments
	commentsOnlyData := `# This file has only comments
# No actual data entries
# Should result in 0 entries`

	testFile := createTestDMRFile(t, tempDir, commentsOnlyData)
	lookup := NewDMRLookup(testFile, 0)

	err := lookup.Read()
	if err != nil {
		t.Fatalf("Failed to read comments-only file: %v", err)
	}

	if count := lookup.GetEntryCount(); count != 0 {
		t.Errorf("Expected 0 entries from comments-only file, got %d", count)
	}

	// Test with very long callsign (should be rejected)
	longCallsignData := `3113 G4KLX
4 VERYLONGCALLSIGNTHATEXCEEDSLIMIT
9990 TG9990`

	testFile2 := createTestDMRFile(t, tempDir, longCallsignData)
	lookup2 := NewDMRLookup(testFile2, 0)

	err = lookup2.Read()
	if err != nil {
		t.Fatalf("Failed to read file with long callsign: %v", err)
	}

	// Should only load valid entries (3113, 9990)
	if count := lookup2.GetEntryCount(); count != 2 {
		t.Errorf("Expected 2 valid entries, got %d", count)
	}

	// Verify the long callsign was rejected
	if lookup2.Exists(4) {
		t.Error("Entry with overly long callsign should have been rejected")
	}
}

// Helper functions

// getTestDMRData returns test DMR ID data
func getTestDMRData() string {
	return `# Test DMR ID database
# Format: ID CALLSIGN
3113 G4KLX
4 N0CALL
9990 TG9990
12345 TESTUSER
16777215 ALL`
}

// createTestDMRFile creates a temporary DMR file with given data
func createTestDMRFile(t *testing.T, dir, data string) string {
	t.Helper()

	filename := filepath.Join(dir, fmt.Sprintf("test_dmr_%d.dat", time.Now().UnixNano()))
	err := os.WriteFile(filename, []byte(data), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return filename
}

// Benchmark tests

func BenchmarkDMRLookupFindCS(b *testing.B) {
	tempDir := b.TempDir()

	// Create a larger dataset for benchmarking
	var data strings.Builder
	data.WriteString("# Benchmark test data\n")
	for i := 1; i <= 1000; i++ {
		data.WriteString(fmt.Sprintf("%d CALL%d\n", i, i))
	}

	testFile := filepath.Join(tempDir, "benchmark.dat")
	err := os.WriteFile(testFile, []byte(data.String()), 0644)
	if err != nil {
		b.Fatal(err)
	}

	lookup := NewDMRLookup(testFile, 0)
	err = lookup.Read()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := uint32((i % 1000) + 1)
		_ = lookup.FindCS(id)
	}
}

func BenchmarkDMRLookupFindID(b *testing.B) {
	tempDir := b.TempDir()

	// Create a larger dataset for benchmarking
	var data strings.Builder
	data.WriteString("# Benchmark test data\n")
	for i := 1; i <= 1000; i++ {
		data.WriteString(fmt.Sprintf("%d CALL%d\n", i, i))
	}

	testFile := filepath.Join(tempDir, "benchmark.dat")
	err := os.WriteFile(testFile, []byte(data.String()), 0644)
	if err != nil {
		b.Fatal(err)
	}

	lookup := NewDMRLookup(testFile, 0)
	err = lookup.Read()
	if err != nil {
		b.Fatal(err)
	}

	callsigns := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		callsigns[i] = fmt.Sprintf("CALL%d", i+1)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		callsign := callsigns[i%1000]
		_ = lookup.FindID(callsign)
	}
}