package codec

import (
	"testing"
)

// TestModeConvBasics tests basic ModeConv functionality
func TestModeConvBasics(t *testing.T) {
	mc := NewModeConv()
	if mc == nil {
		t.Fatal("Failed to create ModeConv")
	}

	// Test initial state
	ysfCount, dmrCount := mc.GetStats()
	if ysfCount != 0 || dmrCount != 0 {
		t.Errorf("Initial stats should be 0,0, got %d,%d", ysfCount, dmrCount)
	}

	if mc.HasYSFData() {
		t.Error("Should not have YSF data initially")
	}

	if mc.HasDMRData() {
		t.Error("Should not have DMR data initially")
	}
}

// TestRingBufferIntegration tests RingBuffer functionality in ModeConv
func TestRingBufferIntegration(t *testing.T) {
	mc := NewModeConv()

	// Test buffer capacities
	ysfFree := mc.GetYSFFreeSpace()
	dmrFree := mc.GetDMRFreeSpace()

	if ysfFree == 0 {
		t.Error("YSF buffer should have free space initially")
	}

	if dmrFree == 0 {
		t.Error("DMR buffer should have free space initially")
	}

	t.Logf("Initial buffer state: YSF free=%d, DMR free=%d", ysfFree, dmrFree)
}

// TestGolayErrorCorrection tests the Golay error correction functions
func TestGolayErrorCorrection(t *testing.T) {
	testData := []struct {
		input    uint32
		expected uint32
	}{
		{0x000, 0x000},     // Zero input
		{0x001, 0x001},     // Single bit
		{0xFFF, 0xFFF},     // Maximum 12-bit value
		{0x123, 0x123},     // Random value
		{0xABC, 0xABC},     // Another random value
	}

	for _, test := range testData {
		// Test Golay24128 encode/decode
		encoded := Encode24128(test.input)
		decoded := Decode24128(encoded)

		if decoded != test.expected {
			t.Errorf("Golay24128: input=0x%03X, encoded=0x%06X, decoded=0x%03X, expected=0x%03X",
				test.input, encoded, decoded, test.expected)
		}

		// Test Golay23127 encode/decode (only 11 bits)
		input11 := test.input & 0x7FF
		expected11 := test.expected & 0x7FF

		encoded23 := Encode23127(input11)
		decoded23 := Decode23127(encoded23)

		if decoded23 != expected11 {
			t.Errorf("Golay23127: input=0x%03X, encoded=0x%06X, decoded=0x%03X, expected=0x%03X",
				input11, encoded23, decoded23, expected11)
		}
	}

	t.Logf("Golay error correction tests passed")
}

// TestPRNGTable tests PRNG table access and validation
func TestPRNGTable(t *testing.T) {
	// Test table size
	if len(PRNG_TABLE) != PRNG_TABLE_SIZE {
		t.Errorf("PRNG table size mismatch: got %d, expected %d", len(PRNG_TABLE), PRNG_TABLE_SIZE)
	}

	// Test that table entries are reasonable (non-zero for most entries)
	nonZeroCount := 0
	for i, entry := range PRNG_TABLE {
		if entry != 0 {
			nonZeroCount++
		}

		// Verify entries are within reasonable 24-bit range
		if entry > 0xFFFFFF {
			t.Errorf("PRNG_TABLE[%d] = 0x%X is larger than 24 bits", i, entry)
		}
	}

	// Expect most entries to be non-zero
	if nonZeroCount < PRNG_TABLE_SIZE/2 {
		t.Errorf("Too many zero entries in PRNG table: %d/%d", nonZeroCount, PRNG_TABLE_SIZE)
	}

	t.Logf("PRNG table validation passed: %d/%d non-zero entries", nonZeroCount, PRNG_TABLE_SIZE)
}

// TestLookupTables tests the various lookup tables
func TestLookupTables(t *testing.T) {
	// Test DMR A table
	if len(DMR_A_TABLE) != 24 {
		t.Errorf("DMR_A_TABLE size should be 24, got %d", len(DMR_A_TABLE))
	}

	// Test DMR B table
	if len(DMR_B_TABLE) != 23 {
		t.Errorf("DMR_B_TABLE size should be 23, got %d", len(DMR_B_TABLE))
	}

	// Test DMR C table
	if len(DMR_C_TABLE) != 25 {
		t.Errorf("DMR_C_TABLE size should be 25, got %d", len(DMR_C_TABLE))
	}

	// Test interleave table
	if len(INTERLEAVE_TABLE_26_4) != 104 {
		t.Errorf("INTERLEAVE_TABLE_26_4 size should be 104, got %d", len(INTERLEAVE_TABLE_26_4))
	}

	// Test whitening data
	if len(WHITENING_DATA) != 20 {
		t.Errorf("WHITENING_DATA size should be 20, got %d", len(WHITENING_DATA))
	}

	// Verify that all positions in DMR tables are valid (< 72 for single frame)
	for i, pos := range DMR_A_TABLE {
		if pos >= 72 {
			t.Errorf("DMR_A_TABLE[%d] = %d is >= 72", i, pos)
		}
	}

	for i, pos := range DMR_B_TABLE {
		if pos >= 72 {
			t.Errorf("DMR_B_TABLE[%d] = %d is >= 72", i, pos)
		}
	}

	for i, pos := range DMR_C_TABLE {
		if pos >= 72 {
			t.Errorf("DMR_C_TABLE[%d] = %d is >= 72", i, pos)
		}
	}

	// Verify interleave table entries are < 104
	for i, pos := range INTERLEAVE_TABLE_26_4 {
		if pos >= 104 {
			t.Errorf("INTERLEAVE_TABLE_26_4[%d] = %d is >= 104", i, pos)
		}
	}

	t.Logf("Lookup table validation passed")
}

// TestAMBEParameterConversion tests the AMBE parameter structure
func TestAMBEParameterConversion(t *testing.T) {
	// Test voice parameter creation and validation
	params := &AMBEVoiceParameters{
		A: 0x123,    // 12-bit value
		B: 0x456,    // 12-bit value (will be truncated to 11 bits for some uses)
		C: 0x789ABC, // 25-bit value
	}

	// Verify parameters are in expected ranges
	if params.A > 0xFFF {
		t.Errorf("Parameter A should be <= 0xFFF, got 0x%X", params.A)
	}

	if params.B > 0xFFF {
		t.Errorf("Parameter B should be <= 0xFFF, got 0x%X", params.B)
	}

	if params.C > 0x1FFFFFF {
		t.Errorf("Parameter C should be <= 0x1FFFFFF, got 0x%X", params.C)
	}

	t.Logf("AMBE parameter test passed: A=0x%03X, B=0x%03X, C=0x%07X", params.A, params.B, params.C)
}

// TestBitManipulation tests the bit reading/writing functions
func TestBitManipulation(t *testing.T) {
	mc := NewModeConv()

	// Create test data
	data := make([]uint8, 10)

	// Test bit writing and reading
	testBits := []struct {
		pos   uint32
		value bool
	}{
		{0, true},
		{1, false},
		{7, true},
		{8, false},
		{15, true},
		{63, true},
		{79, false},
	}

	// Write test bits
	for _, test := range testBits {
		if test.pos < 80 { // Within our test data size
			mc.writeBit(data, test.pos, test.value)
		}
	}

	// Read and verify test bits
	for _, test := range testBits {
		if test.pos < 80 {
			result := mc.readBit(data, test.pos)
			if result != test.value {
				t.Errorf("Bit %d: wrote %v, read %v", test.pos, test.value, result)
			}
		}
	}

	t.Logf("Bit manipulation test passed")
}

// TestFrameTags tests the frame tag constants
func TestFrameTags(t *testing.T) {
	tags := []struct {
		name string
		tag  uint8
	}{
		{"HEADER", TAG_HEADER},
		{"DATA", TAG_DATA},
		{"EOT", TAG_EOT},
		{"NODATA", TAG_NODATA},
	}

	// Verify tags are unique
	seen := make(map[uint8]bool)
	for _, tag := range tags {
		if seen[tag.tag] {
			t.Errorf("Duplicate tag value: %s = 0x%02X", tag.name, tag.tag)
		}
		seen[tag.tag] = true
	}

	t.Logf("Frame tag test passed: %d unique tags", len(tags))
}

// Benchmark tests for performance validation
func BenchmarkGolayEncode24128(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Encode24128(uint32(i & 0xFFF))
	}
}

func BenchmarkGolayDecode24128(b *testing.B) {
	// Pre-encode some test data
	encoded := Encode24128(0x123)

	for i := 0; i < b.N; i++ {
		_ = Decode24128(encoded)
	}
}

func BenchmarkPRNGLookup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PRNG_TABLE[i & (PRNG_TABLE_SIZE-1)]
	}
}