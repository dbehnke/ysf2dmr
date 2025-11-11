package codec

import (
	"testing"
)

// TestRS129BasicEncodeDecode tests basic encode/decode functionality
func TestRS129BasicEncodeDecode(t *testing.T) {
	testData := [][9]uint8{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // All zeros
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, // All ones
		{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0x00}, // Sequential
		{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA}, // Pattern
		{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, 0x11}, // Random
		{0x87, 0x65, 0x43, 0x21, 0x0F, 0xED, 0xCB, 0xA9, 0x87}, // Reverse
	}

	for i, data := range testData {
		// Encode data to get 12-byte codeword
		encoded := RS129EncodeData(data)

		// Check if the encoded data passes validation
		if !RS129Check(encoded[:]) {
			t.Errorf("Test %d: Encoded data failed validation", i)
		}

		// Decode and verify we get back the original data
		decoded, valid := RS129DecodeData(encoded)
		if !valid {
			t.Errorf("Test %d: Decoding reported invalid codeword", i)
		}

		if decoded != data {
			t.Errorf("Test %d: Decoded data doesn't match original\nOriginal: %X\nDecoded:  %X",
				i, data, decoded)
		}
	}
}

// TestRS129EncodeParity tests the parity calculation
func TestRS129EncodeParity(t *testing.T) {
	// Test with known data
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	var parity [4]uint8

	RS129Encode(data, 9, parity[:])

	// Verify parity is not all zeros (unless data is all zeros)
	if parity[0] == 0 && parity[1] == 0 && parity[2] == 0 {
		t.Error("Parity calculation resulted in all zeros for non-zero data")
	}

	t.Logf("Data: %X", data)
	t.Logf("Parity: [%02X %02X %02X %02X]", parity[0], parity[1], parity[2], parity[3])
}

// TestRS129GaloisMult tests the Galois field multiplication
func TestRS129GaloisMult(t *testing.T) {
	// Test multiplication by zero
	if rs129GMult(0x00, 0x12) != 0x00 {
		t.Error("Multiplication by zero should give zero")
	}

	if rs129GMult(0x34, 0x00) != 0x00 {
		t.Error("Multiplication by zero should give zero")
	}

	// Test multiplication by one (alpha^0)
	if rs129GMult(0x01, 0x56) != 0x56 {
		t.Error("Multiplication by 1 should give the same value")
	}

	// Test some known multiplications in GF(256)
	// These values are based on the actual lookup tables
	testCases := []struct {
		a, b, expected uint8
	}{
		{0x02, 0x02, 0x04},     // 2 * 2 = 4 (simple case)
		{0x02, 0x80, 0x1D},     // 2 * 128 (overflow test)
		{0x03, 0x05, 0x0F},     // 3 * 5 = 15 (simple case)
		{0x07, 0x0B, 0x31},     // 7 * 11 (based on our tables)
	}

	for _, tc := range testCases {
		result := rs129GMult(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("GMult(0x%02X, 0x%02X) = 0x%02X, expected 0x%02X",
				tc.a, tc.b, result, tc.expected)
		}

		// Verify commutativity: a*b = b*a
		result2 := rs129GMult(tc.b, tc.a)
		if result != result2 {
			t.Errorf("GMult not commutative: 0x%02X*0x%02X=0x%02X, but 0x%02X*0x%02X=0x%02X",
				tc.a, tc.b, result, tc.b, tc.a, result2)
		}
	}
}

// TestRS129Check tests the validation function
func TestRS129Check(t *testing.T) {
	// Create a valid codeword
	data := [9]uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}
	encoded := RS129EncodeData(data)

	// Should pass validation
	if !RS129Check(encoded[:]) {
		t.Error("Valid codeword failed check")
	}

	// Corrupt one byte and verify it fails
	corrupted := encoded
	corrupted[5] ^= 0x01 // Flip one bit

	if RS129Check(corrupted[:]) {
		t.Error("Corrupted codeword passed check")
	}

	// Test with insufficient data
	shortData := []uint8{0x01, 0x02, 0x03}
	if RS129Check(shortData) {
		t.Error("Short data should fail check")
	}
}

// TestRS129Syndromes tests syndrome calculation
func TestRS129Syndromes(t *testing.T) {
	// Create valid codeword - should have zero syndromes
	data := [9]uint8{0xAB, 0xCD, 0xEF, 0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}
	encoded := RS129EncodeData(data)

	syndromes := RS129GetSyndromes(encoded[:])

	// Valid codeword should have all syndromes zero
	for i, syndrome := range syndromes {
		if syndrome != 0x00 {
			t.Errorf("Valid codeword has non-zero syndrome[%d] = 0x%02X", i, syndrome)
		}
	}

	// Corrupt data and verify syndromes are non-zero
	corrupted := encoded
	corrupted[3] ^= 0xFF // Introduce error

	syndromes = RS129GetSyndromes(corrupted[:])

	// At least one syndrome should be non-zero
	hasNonZero := false
	for _, syndrome := range syndromes {
		if syndrome != 0x00 {
			hasNonZero = true
			break
		}
	}

	if !hasNonZero {
		t.Error("Corrupted codeword has all zero syndromes")
	}

	t.Logf("Corrupted syndromes: [%02X %02X %02X]", syndromes[0], syndromes[1], syndromes[2])
}

// TestRS129Validation tests the built-in validation function
func TestRS129Validation(t *testing.T) {
	if !RS129Validate() {
		t.Error("RS129 validation failed")
	}
}

// TestRS129LookupTables tests the lookup table integrity
func TestRS129LookupTables(t *testing.T) {
	// Test EXP_TABLE size
	if len(RS129_EXP_TABLE) != 512 {
		t.Errorf("EXP_TABLE size incorrect: got %d, expected 512", len(RS129_EXP_TABLE))
	}

	// Test LOG_TABLE size
	if len(RS129_LOG_TABLE) != 256 {
		t.Errorf("LOG_TABLE size incorrect: got %d, expected 256", len(RS129_LOG_TABLE))
	}

	// Test that EXP and LOG tables are inverse operations
	for i := uint8(1); i < 255; i++ { // Skip 0 since log(0) is undefined
		exp_log_i := RS129_EXP_TABLE[RS129_LOG_TABLE[i]]
		if exp_log_i != i {
			t.Errorf("EXP[LOG[%d]] = %d, expected %d", i, exp_log_i, i)
		}
	}

	// Test specific values
	if RS129_EXP_TABLE[0] != 0x01 {
		t.Errorf("EXP_TABLE[0] should be 1, got 0x%02X", RS129_EXP_TABLE[0])
	}

	if RS129_LOG_TABLE[1] != 0x00 {
		t.Errorf("LOG_TABLE[1] should be 0, got 0x%02X", RS129_LOG_TABLE[1])
	}
}

// TestRS129PolynomialTable tests the generator polynomial
func TestRS129PolynomialTable(t *testing.T) {
	// Test polynomial size
	if len(RS129_POLY) != 12 {
		t.Errorf("Generator polynomial size incorrect: got %d, expected 12", len(RS129_POLY))
	}

	// Verify known values
	if RS129_POLY[0] != 64 {
		t.Errorf("POLY[0] should be 64, got %d", RS129_POLY[0])
	}

	if RS129_POLY[1] != 56 {
		t.Errorf("POLY[1] should be 56, got %d", RS129_POLY[1])
	}

	if RS129_POLY[2] != 14 {
		t.Errorf("POLY[2] should be 14, got %d", RS129_POLY[2])
	}

	if RS129_POLY[3] != 1 {
		t.Errorf("POLY[3] should be 1, got %d", RS129_POLY[3])
	}

	// Remaining coefficients should be zero
	for i := 4; i < 12; i++ {
		if RS129_POLY[i] != 0 {
			t.Errorf("POLY[%d] should be 0, got %d", i, RS129_POLY[i])
		}
	}
}

// TestRS129ErrorPatterns tests various error patterns
func TestRS129ErrorPatterns(t *testing.T) {
	data := [9]uint8{0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A}
	encoded := RS129EncodeData(data)

	// Test single bit errors in each position
	for pos := 0; pos < 12; pos++ {
		for bit := 0; bit < 8; bit++ {
			corrupted := encoded
			corrupted[pos] ^= (1 << bit) // Flip single bit

			// RS(12,9) can correct up to 1 symbol error
			// So single bit errors should be detectable
			if RS129Check(corrupted[:]) {
				t.Errorf("Single bit error at position %d bit %d was not detected", pos, bit)
			}
		}
	}

	// Test single byte errors (more severe than bit errors)
	for pos := 0; pos < 12; pos++ {
		corrupted := encoded
		corrupted[pos] ^= 0xFF // Flip all bits in byte

		if RS129Check(corrupted[:]) {
			t.Errorf("Single byte error at position %d was not detected", pos)
		}
	}
}

// TestRS129EdgeCases tests edge cases and boundary conditions
func TestRS129EdgeCases(t *testing.T) {
	// Test with nil/empty slices
	var nilSlice []uint8
	if RS129Check(nilSlice) {
		t.Error("nil slice should fail check")
	}

	emptySlice := make([]uint8, 0)
	if RS129Check(emptySlice) {
		t.Error("empty slice should fail check")
	}

	// Test with insufficient parity buffer
	data := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	shortParity := make([]uint8, 2) // Too short

	// Should not crash, but won't work correctly
	RS129Encode(data, 9, shortParity)

	// Test with more than 9 bytes of data
	longData := make([]uint8, 20)
	for i := range longData {
		longData[i] = uint8(i)
	}

	var parity [4]uint8
	RS129Encode(longData, 20, parity[:]) // Should handle gracefully

	t.Logf("Long data parity: [%02X %02X %02X %02X]", parity[0], parity[1], parity[2], parity[3])
}

// BenchmarkRS129Encode benchmarks encoding performance
func BenchmarkRS129Encode(b *testing.B) {
	data := []uint8{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99}
	var parity [4]uint8

	for i := 0; i < b.N; i++ {
		RS129Encode(data, 9, parity[:])
	}
}

// BenchmarkRS129Check benchmarks validation performance
func BenchmarkRS129Check(b *testing.B) {
	data := [9]uint8{0xAB, 0xCD, 0xEF, 0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC}
	encoded := RS129EncodeData(data)

	for i := 0; i < b.N; i++ {
		RS129Check(encoded[:])
	}
}

// BenchmarkRS129GMult benchmarks Galois field multiplication
func BenchmarkRS129GMult(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rs129GMult(uint8(i&0xFF), 0x12)
	}
}

// BenchmarkRS129EncodeData benchmarks convenience function performance
func BenchmarkRS129EncodeData(b *testing.B) {
	data := [9]uint8{0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A}

	for i := 0; i < b.N; i++ {
		RS129EncodeData(data)
	}
}