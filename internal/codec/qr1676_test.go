package codec

import (
	"testing"
)

// TestQR1676EncodeDecodeBasic tests basic encode/decode functionality
func TestQR1676EncodeDecodeBasic(t *testing.T) {
	// Test only valid 7-bit inputs (0x00-0x7F)
	testData := []uint8{0x00, 0x01, 0x7F, 0x55, 0x3F, 0x0F, 0x70}

	for _, input := range testData {
		// Create test data with input in bits 1-7 of first byte
		data := make([]uint8, 2)
		data[0] = input << 1 // Shift to bits 1-7

		// Encode
		QR1676Encode(data)

		// Decode
		result := QR1676Decode(data)

		if result != input {
			t.Errorf("QR1676 encode/decode failed: input=0x%02X, result=0x%02X", input, result)
		}
	}
}

// TestQR1676ConvenienceFunctions tests the convenience functions
func TestQR1676ConvenienceFunctions(t *testing.T) {
	for i := uint8(0); i < 128; i++ {
		// Test convenience functions
		byte0, byte1 := QR1676EncodeData(i)
		decoded := QR1676DecodeData(byte0, byte1)

		if decoded != i {
			t.Errorf("QR1676 convenience functions failed: input=0x%02X, decoded=0x%02X", i, decoded)
		}
	}
}

// TestQR1676ValidationFunction tests the validation function
func TestQR1676ValidationFunction(t *testing.T) {
	if !QR1676Validate() {
		t.Error("QR1676 validation failed - encoding/decoding tables are inconsistent")
	}
}

// TestQR1676ErrorCorrection tests error correction capability
func TestQR1676ErrorCorrection(t *testing.T) {
	testCases := []struct {
		data     uint8
		errorBit int // Bit position to flip (0-14 for 15-bit code)
	}{
		{0x00, 0},  // Single bit error at position 0
		{0x00, 1},  // Single bit error at position 1
		{0x00, 7},  // Single bit error at position 7
		{0x00, 14}, // Single bit error at position 14
		{0x7F, 3},  // Single bit error in maximum value
		{0x55, 8},  // Single bit error in pattern
		{0x2A, 12}, // Single bit error in another pattern
	}

	for _, test := range testCases {
		// Encode original data
		byte0, byte1 := QR1676EncodeData(test.data)

		// Extract 15-bit code
		code := (uint32(byte0) << 7) + (uint32(byte1) >> 1)

		// Introduce single bit error
		errorMask := uint32(1) << test.errorBit
		corruptedCode := code ^ errorMask

		// Pack back into bytes
		corruptedByte0 := uint8(corruptedCode >> 7)
		corruptedByte1 := uint8((corruptedCode << 1) & 0xFF)

		// Decode and verify correction
		corrected := QR1676DecodeData(corruptedByte0, corruptedByte1)

		if corrected != test.data {
			t.Errorf("QR1676 single bit error correction failed: original=0x%02X, errorBit=%d, corrected=0x%02X",
				test.data, test.errorBit, corrected)
		}
	}
}

// TestQR1676TwoBitErrorCorrection tests two-bit error correction
func TestQR1676TwoBitErrorCorrection(t *testing.T) {
	testCases := []struct {
		data      uint8
		errorBit1 int
		errorBit2 int
	}{
		{0x00, 0, 1},   // Adjacent bits
		{0x00, 0, 14},  // Far apart bits
		{0x7F, 3, 7},   // Two bits in the middle
		{0x55, 2, 10},  // Spread out errors
	}

	for _, test := range testCases {
		// Encode original data
		byte0, byte1 := QR1676EncodeData(test.data)

		// Extract 15-bit code
		code := (uint32(byte0) << 7) + (uint32(byte1) >> 1)

		// Introduce two bit errors
		errorMask := (uint32(1) << test.errorBit1) | (uint32(1) << test.errorBit2)
		corruptedCode := code ^ errorMask

		// Pack back into bytes
		corruptedByte0 := uint8(corruptedCode >> 7)
		corruptedByte1 := uint8((corruptedCode << 1) & 0xFF)

		// Decode and verify correction
		corrected := QR1676DecodeData(corruptedByte0, corruptedByte1)

		if corrected != test.data {
			t.Errorf("QR1676 two bit error correction failed: original=0x%02X, errorBits=%d,%d, corrected=0x%02X",
				test.data, test.errorBit1, test.errorBit2, corrected)
		}
	}
}

// TestQR1676SyndromeCalculation tests the syndrome calculation function
func TestQR1676SyndromeCalculation(t *testing.T) {
	// Test with known valid codewords (syndrome should be 0)
	for i := uint8(0); i < 128; i++ {
		// Get valid codeword
		codeword := ENCODING_TABLE_1676[i]

		// Extract 15-bit pattern the same way the decode function does
		data := []uint8{uint8(codeword >> 8), uint8(codeword & 0xFF)}
		pattern := (uint32(data[0]) << 7) + (uint32(data[1]) >> 1)

		// Calculate syndrome
		syndrome := QR1676GetSyndrome(pattern)

		if syndrome != 0 {
			t.Errorf("Valid codeword has non-zero syndrome: data=0x%02X, codeword=0x%04X, pattern=0x%04X, syndrome=0x%02X",
				i, codeword, pattern, syndrome)
		}
	}
}

// TestQR1676Constants tests the QR1676 constants
func TestQR1676Constants(t *testing.T) {
	// Verify constant values match expected values
	if QR_X14 != 0x00004000 {
		t.Errorf("QR_X14 constant incorrect: got 0x%08X, expected 0x00004000", QR_X14)
	}

	if QR_X8 != 0x00000100 {
		t.Errorf("QR_X8 constant incorrect: got 0x%08X, expected 0x00000100", QR_X8)
	}

	if QR_MASK7 != 0xffffff00 {
		t.Errorf("QR_MASK7 constant incorrect: got 0x%08X, expected 0xffffff00", QR_MASK7)
	}

	if QR_GENPOL != 0x00000139 {
		t.Errorf("QR_GENPOL constant incorrect: got 0x%08X, expected 0x00000139", QR_GENPOL)
	}
}

// TestQR1676LookupTables tests the lookup table sizes and basic properties
func TestQR1676LookupTables(t *testing.T) {
	// Test encoding table size
	if len(ENCODING_TABLE_1676) != 128 {
		t.Errorf("ENCODING_TABLE_1676 size incorrect: got %d, expected 128", len(ENCODING_TABLE_1676))
	}

	// Test decoding table size
	if len(DECODING_TABLE_1576) != 256 {
		t.Errorf("DECODING_TABLE_1576 size incorrect: got %d, expected 256", len(DECODING_TABLE_1576))
	}

	// Verify encoding table entries are within 16-bit range
	for i, entry := range ENCODING_TABLE_1676 {
		if entry > 0xFFFF {
			t.Errorf("ENCODING_TABLE_1676[%d] = 0x%X exceeds 16-bit range", i, entry)
		}
	}

	// Verify decoding table entries are within 16-bit range
	for i, entry := range DECODING_TABLE_1576 {
		if entry > 0xFFFF {
			t.Errorf("DECODING_TABLE_1576[%d] = 0x%X exceeds 16-bit range", i, entry)
		}
	}
}

// TestQR1676EdgeCases tests edge cases and boundary conditions
func TestQR1676EdgeCases(t *testing.T) {
	// Test with insufficient buffer size
	smallData := make([]uint8, 1)
	smallData[0] = 0x02 // Some test data

	// Encode should handle gracefully
	QR1676Encode(smallData) // Should not crash

	// Decode should return 0
	result := QR1676Decode(smallData)
	if result != 0 {
		t.Errorf("QR1676Decode with insufficient buffer should return 0, got 0x%02X", result)
	}

	// Test with nil slice
	var nilData []uint8
	QR1676Encode(nilData) // Should not crash
	result = QR1676Decode(nilData)
	if result != 0 {
		t.Errorf("QR1676Decode with nil buffer should return 0, got 0x%02X", result)
	}

	// Test maximum input values
	maxData := make([]uint8, 2)
	maxData[0] = 0xFF // Maximum possible input
	maxData[1] = 0xFF

	QR1676Encode(maxData)
	decoded := QR1676Decode(maxData)

	// Should decode to 0x7F (maximum 7-bit value)
	expected := uint8(0x7F)
	if decoded != expected {
		t.Errorf("QR1676 with maximum input: got 0x%02X, expected 0x%02X", decoded, expected)
	}
}

// BenchmarkQR1676Encode benchmarks encoding performance
func BenchmarkQR1676Encode(b *testing.B) {
	data := make([]uint8, 2)

	for i := 0; i < b.N; i++ {
		data[0] = uint8(i<<1) & 0xFE // Vary the input
		QR1676Encode(data)
	}
}

// BenchmarkQR1676Decode benchmarks decoding performance
func BenchmarkQR1676Decode(b *testing.B) {
	// Pre-encoded data
	data := []uint8{0x12, 0x34}

	for i := 0; i < b.N; i++ {
		QR1676Decode(data)
	}
}

// BenchmarkQR1676EncodeData benchmarks convenience function performance
func BenchmarkQR1676EncodeData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		QR1676EncodeData(uint8(i & 0x7F))
	}
}

// BenchmarkQR1676DecodeData benchmarks convenience function performance
func BenchmarkQR1676DecodeData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		QR1676DecodeData(0x12, 0x34)
	}
}