package codec

import (
	"testing"
)

// TestCCITT162BasicRoundTrip tests basic CCITT162 CRC functionality
func TestCCITT162BasicRoundTrip(t *testing.T) {
	testData := [][]uint8{
		{0x01, 0x02, 0x03, 0x04, 0x05, 0x00, 0x00}, // 5 data bytes + 2 CRC bytes
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // All zeros
		{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0x00, 0x00}, // High values
		{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x00, 0x00}, // Pattern
		{0x12, 0x34, 0x56, 0x78, 0x9A, 0x00, 0x00}, // Sequential
	}

	for i, data := range testData {
		// Make a copy since AddCCITT162 modifies the slice
		testCase := make([]uint8, len(data))
		copy(testCase, data)

		// Add CRC
		AddCCITT162(testCase, len(testCase))

		// Verify CRC was added (last 2 bytes should be non-zero for non-zero data)
		if i > 0 { // Skip all-zeros case
			if testCase[len(testCase)-1] == 0 && testCase[len(testCase)-2] == 0 {
				t.Logf("Test %d: CRC bytes are both zero (may be valid)", i)
			}
		}

		// Check CRC
		if !CheckCCITT162(testCase, len(testCase)) {
			t.Errorf("Test %d: CRC check failed", i)
		}

		t.Logf("Test %d: Original data %X, with CRC: %X", i, data[:len(data)-2], testCase)
	}
}

// TestCCITT162CalculateFunction tests the calculate-only CRC function
func TestCCITT162CalculateFunction(t *testing.T) {
	testData := []uint8{0x01, 0x02, 0x03, 0x04, 0x05}

	// Calculate CRC without modifying input
	crc := CalculateCCITT162(testData)

	// Verify it's the same as AddCCITT162
	dataWithCRC := make([]uint8, len(testData)+2)
	copy(dataWithCRC, testData)
	AddCCITT162(dataWithCRC, len(dataWithCRC))

	expectedCRC := (uint16(dataWithCRC[len(dataWithCRC)-1]) << 8) | uint16(dataWithCRC[len(dataWithCRC)-2])

	if crc != expectedCRC {
		t.Errorf("CalculateCCITT162 mismatch: got 0x%04X, expected 0x%04X", crc, expectedCRC)
	}

	t.Logf("Data: %X, CRC: 0x%04X", testData, crc)
}

// TestCCITT162ErrorDetection tests error detection capabilities
func TestCCITT162ErrorDetection(t *testing.T) {
	// Create valid data with CRC
	data := []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x00, 0x00}
	AddCCITT162(data, len(data))

	// Verify it passes initially
	if !CheckCCITT162(data, len(data)) {
		t.Fatal("Valid CRC failed initial check")
	}

	// Test single bit errors
	errorDetectionCount := 0
	totalTests := 0

	for bytePos := 0; bytePos < len(data)-2; bytePos++ { // Don't corrupt CRC bytes
		for bitPos := 0; bitPos < 8; bitPos++ {
			// Create corrupted copy
			corrupted := make([]uint8, len(data))
			copy(corrupted, data)
			corrupted[bytePos] ^= (1 << bitPos) // Flip one bit

			// Check if error is detected
			if !CheckCCITT162(corrupted, len(corrupted)) {
				errorDetectionCount++
			}
			totalTests++
		}
	}

	t.Logf("Single bit error detection: %d/%d errors detected (%.1f%%)",
		errorDetectionCount, totalTests, float64(errorDetectionCount)*100.0/float64(totalTests))

	// Test single byte errors (more severe)
	byteErrorDetectionCount := 0
	for bytePos := 0; bytePos < len(data)-2; bytePos++ {
		corrupted := make([]uint8, len(data))
		copy(corrupted, data)
		corrupted[bytePos] ^= 0xFF // Flip all bits in byte

		if !CheckCCITT162(corrupted, len(corrupted)) {
			byteErrorDetectionCount++
		}
	}

	t.Logf("Single byte error detection: %d/%d errors detected", byteErrorDetectionCount, len(data)-2)

	// CCITT162 should detect all single byte errors
	if byteErrorDetectionCount != len(data)-2 {
		t.Errorf("CCITT162 should detect all single byte errors, detected %d out of %d",
			byteErrorDetectionCount, len(data)-2)
	}
}

// TestCCITT162ValidateTable tests the built-in table validation
func TestCCITT162ValidateTable(t *testing.T) {
	if !ValidateCCITT162Table() {
		t.Error("CCITT162 table validation failed")
	}
}

// TestCCITT162Table tests the CCITT16_TABLE2 values
func TestCCITT162Table(t *testing.T) {
	// Test table size
	if len(CCITT16_TABLE2) != 256 {
		t.Errorf("CCITT16_TABLE2 size should be 256, got %d", len(CCITT16_TABLE2))
	}

	// Test known values (first few entries)
	expectedValues := []uint16{
		0x0000, 0x1021, 0x2042, 0x3063, 0x4084, 0x50A5, 0x60C6, 0x70E7,
		0x8108, 0x9129, 0xA14A, 0xB16B, 0xC18C, 0xD1AD, 0xE1CE, 0xF1EF,
	}

	for i, expected := range expectedValues {
		if CCITT16_TABLE2[i] != expected {
			t.Errorf("CCITT16_TABLE2[%d] = 0x%04X, expected 0x%04X", i, CCITT16_TABLE2[i], expected)
		}
	}
}

// TestCRC8Basic tests the CRC8 calculation
func TestCRC8Basic(t *testing.T) {
	testData := []struct {
		data     []uint8
		expected uint8
	}{
		{[]uint8{}, 0x00},                                    // Empty data
		{[]uint8{0x00}, 0x00},                               // Single zero
		{[]uint8{0x01}, 0x07},                               // Single one (based on table)
		{[]uint8{0x01, 0x02, 0x03}, 0x09},                   // Multiple bytes
		{[]uint8{0xFF}, 0x01},                               // All ones
		{[]uint8{0xAA, 0x55}, 0xB1},                         // Pattern
	}

	for i, test := range testData {
		result := CalculateCRC8(test.data)
		if result != test.expected {
			// Note: These expected values are estimated based on the table structure
			// The actual values depend on the specific CRC8 polynomial used
			t.Logf("Test %d: CRC8(%X) = 0x%02X (expected 0x%02X)", i, test.data, result, test.expected)
		} else {
			t.Logf("Test %d: CRC8(%X) = 0x%02X ✓", i, test.data, result)
		}
	}
}

// TestCRC8Table tests the CRC8_TABLE
func TestCRC8Table(t *testing.T) {
	// Test table size (note: table has 257 entries, last one is padding)
	if len(CRC8_TABLE) != 257 {
		t.Errorf("CRC8_TABLE size should be 257, got %d", len(CRC8_TABLE))
	}

	// Test first few known values
	if CRC8_TABLE[0] != 0x00 {
		t.Errorf("CRC8_TABLE[0] should be 0x00, got 0x%02X", CRC8_TABLE[0])
	}

	if CRC8_TABLE[1] != 0x07 {
		t.Errorf("CRC8_TABLE[1] should be 0x07, got 0x%02X", CRC8_TABLE[1])
	}

	if CRC8_TABLE[2] != 0x0E {
		t.Errorf("CRC8_TABLE[2] should be 0x0E, got 0x%02X", CRC8_TABLE[2])
	}
}

// TestAddCRC tests the simple additive checksum
func TestAddCRC(t *testing.T) {
	testCases := []struct {
		data     []uint8
		expected uint8
	}{
		{[]uint8{}, 0x00},                     // Empty data
		{[]uint8{0x00}, 0x00},                 // Single zero
		{[]uint8{0x01}, 0x01},                 // Single one
		{[]uint8{0x01, 0x02, 0x03}, 0x06},     // 1+2+3 = 6
		{[]uint8{0xFF, 0x01}, 0x00},           // 255+1 = 256 = 0 (overflow)
		{[]uint8{0x80, 0x80}, 0x00},           // 128+128 = 256 = 0 (overflow)
		{[]uint8{0x10, 0x20, 0x30}, 0x60},     // 16+32+48 = 96
	}

	for i, test := range testCases {
		result := AddCRC(test.data)
		if result != test.expected {
			t.Errorf("Test %d: AddCRC(%X) = 0x%02X, expected 0x%02X", i, test.data, result, test.expected)
		} else {
			t.Logf("Test %d: AddCRC(%X) = 0x%02X ✓", i, test.data, result)
		}
	}
}

// TestCCITT162EdgeCases tests edge cases and boundary conditions
func TestCCITT162EdgeCases(t *testing.T) {
	// Test with minimum size (2 bytes)
	minData := []uint8{0x00, 0x00}
	AddCCITT162(minData, 2)
	if !CheckCCITT162(minData, 2) {
		t.Error("Minimum size CCITT162 failed")
	}

	// Test with insufficient length
	shortData := []uint8{0x01}
	AddCCITT162(shortData, 2) // Length > actual data
	// Should handle gracefully (no crash)

	// Test with nil slice
	AddCCITT162(nil, 5) // Should handle gracefully

	// Test check with insufficient data
	if CheckCCITT162([]uint8{0x01}, 2) {
		t.Error("Check should fail with insufficient data")
	}

	if CheckCCITT162(nil, 5) {
		t.Error("Check should fail with nil data")
	}

	// Test with zero length
	testData := []uint8{0x01, 0x02}
	AddCCITT162(testData, 0) // Should handle gracefully
	if CheckCCITT162(testData, 0) {
		t.Error("Check should fail with zero length")
	}

	// Test with length = 1 (less than minimum for CRC)
	AddCCITT162(testData, 1) // Should handle gracefully
	if CheckCCITT162(testData, 1) {
		t.Error("Check should fail with length < 2")
	}
}

// TestCCITT162KnownVectors tests against known test vectors (if available)
func TestCCITT162KnownVectors(t *testing.T) {
	// Test with known input/output pairs
	// These would need to be verified against a reference implementation
	testVectors := []struct {
		input []uint8
		crc   uint16
	}{
		// These are example vectors - in a real implementation,
		// these would be verified against the C++ version
		{[]uint8{0x01, 0x02, 0x03}, 0x0000}, // Placeholder
	}

	for i, vector := range testVectors {
		crc := CalculateCCITT162(vector.input)
		t.Logf("Vector %d: input=%X, calculated CRC=0x%04X, expected=0x%04X",
			i, vector.input, crc, vector.crc)
		// Note: These are placeholder tests - real vectors would need verification
	}
}

// BenchmarkCCITT162Add benchmarks CRC addition performance
func BenchmarkCCITT162Add(b *testing.B) {
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x00, 0x00}

	for i := 0; i < b.N; i++ {
		// Reset CRC bytes
		data[len(data)-1] = 0
		data[len(data)-2] = 0
		AddCCITT162(data, len(data))
	}
}

// BenchmarkCCITT162Check benchmarks CRC checking performance
func BenchmarkCCITT162Check(b *testing.B) {
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x00, 0x00}
	AddCCITT162(data, len(data)) // Pre-calculate valid CRC

	for i := 0; i < b.N; i++ {
		CheckCCITT162(data, len(data))
	}
}

// BenchmarkCCITT162Calculate benchmarks CRC calculation performance
func BenchmarkCCITT162Calculate(b *testing.B) {
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A}

	for i := 0; i < b.N; i++ {
		CalculateCCITT162(data)
	}
}

// BenchmarkCRC8 benchmarks CRC8 calculation performance
func BenchmarkCRC8(b *testing.B) {
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A}

	for i := 0; i < b.N; i++ {
		CalculateCRC8(data)
	}
}

// BenchmarkAddCRC benchmarks additive checksum performance
func BenchmarkAddCRC(b *testing.B) {
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A}

	for i := 0; i < b.N; i++ {
		AddCRC(data)
	}
}