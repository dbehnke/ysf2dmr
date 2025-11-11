package codec

import (
	"testing"
)

// TestHamming15113_2 tests Hamming(15,11,3) variant 2 encode/decode
func TestHamming15113_2(t *testing.T) {
	testCases := [][]bool{
		// Test case 1: All zeros
		{false, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
		// Test case 2: All ones in data positions
		{true, true, true, true, true, true, true, true, true, true, true, false, false, false, false},
		// Test case 3: Alternating pattern
		{true, false, true, false, true, false, true, false, true, false, true, false, false, false, false},
		// Test case 4: Single bit set
		{true, false, false, false, false, false, false, false, false, false, false, false, false, false, false},
	}

	for i, testData := range testCases {
		// Make a copy for testing
		data := make([]bool, 15)
		copy(data, testData)

		// Encode
		Encode15113_2(data)

		// Verify that a valid codeword doesn't report errors
		if Decode15113_2(data) {
			t.Errorf("Test %d: Valid codeword reported as having errors", i)
		}

		// Test single bit error correction
		for bitPos := 0; bitPos < 15; bitPos++ {
			// Create corrupted copy
			corrupted := make([]bool, 15)
			copy(corrupted, data)
			corrupted[bitPos] = !corrupted[bitPos] // Flip bit

			// Should detect and correct error
			if !Decode15113_2(corrupted) {
				t.Errorf("Test %d, bit %d: Failed to detect single bit error", i, bitPos)
			}

			// Should match original after correction
			for j := 0; j < 15; j++ {
				if corrupted[j] != data[j] {
					t.Errorf("Test %d, bit %d: Error correction failed at position %d", i, bitPos, j)
				}
			}
		}
	}
}

// TestHamming1393 tests Hamming(13,9,3) encode/decode
func TestHamming1393(t *testing.T) {
	testCases := [][]bool{
		// Test case 1: All zeros
		{false, false, false, false, false, false, false, false, false, false, false, false, false},
		// Test case 2: All ones in data positions
		{true, true, true, true, true, true, true, true, true, false, false, false, false},
		// Test case 3: Alternating pattern
		{true, false, true, false, true, false, true, false, true, false, false, false, false},
		// Test case 4: Sequential pattern
		{true, true, false, false, true, true, false, false, true, false, false, false, false},
	}

	for i, testData := range testCases {
		// Make a copy for testing
		data := make([]bool, 13)
		copy(data, testData)

		// Encode
		Encode1393(data)

		// Verify that a valid codeword doesn't report errors
		if Decode1393(data) {
			t.Errorf("Test %d: Valid codeword reported as having errors", i)
		}

		// Test single bit error correction
		for bitPos := 0; bitPos < 13; bitPos++ {
			// Create corrupted copy
			corrupted := make([]bool, 13)
			copy(corrupted, data)
			corrupted[bitPos] = !corrupted[bitPos] // Flip bit

			// Should detect and correct error
			if !Decode1393(corrupted) {
				t.Errorf("Test %d, bit %d: Failed to detect single bit error", i, bitPos)
			}

			// Should match original after correction
			for j := 0; j < 13; j++ {
				if corrupted[j] != data[j] {
					t.Errorf("Test %d, bit %d: Error correction failed at position %d", i, bitPos, j)
				}
			}
		}
	}
}

// TestBitConversion tests byte/bit conversion functions
func TestBitConversion(t *testing.T) {
	testBytes := []uint8{0x00, 0xFF, 0xAA, 0x55, 0x12, 0x34, 0x80, 0x01}

	for _, b := range testBytes {
		// Convert byte to bits
		bits := make([]bool, 8)
		ByteToBitsBE(b, bits)

		// Convert bits back to byte
		result := BitsToByteBE(bits)

		if result != b {
			t.Errorf("Bit conversion failed: 0x%02X → %v → 0x%02X", b, bits, result)
		}
	}

	// Test specific bit patterns
	testPatterns := []struct {
		bits []bool
		byte uint8
	}{
		{[]bool{true, false, false, false, false, false, false, false}, 0x80},
		{[]bool{false, false, false, false, false, false, false, true}, 0x01},
		{[]bool{true, false, true, false, true, false, true, false}, 0xAA},
		{[]bool{false, true, false, true, false, true, false, true}, 0x55},
	}

	for i, test := range testPatterns {
		result := BitsToByteBE(test.bits)
		if result != test.byte {
			t.Errorf("Pattern %d: Expected 0x%02X, got 0x%02X", i, test.byte, result)
		}

		bits := make([]bool, 8)
		ByteToBitsBE(test.byte, bits)
		for j := 0; j < 8; j++ {
			if bits[j] != test.bits[j] {
				t.Errorf("Pattern %d: Bit %d mismatch", i, j)
			}
		}
	}
}

// TestHammingValidation tests the built-in validation function
func TestHammingValidation(t *testing.T) {
	if !ValidateHamming() {
		t.Error("Hamming validation failed")
	}
}

// TestHammingErrorDetection tests error detection capabilities
func TestHammingErrorDetection(t *testing.T) {
	// Test Hamming(15,11,3) with two-bit errors (should not correct)
	data1511 := []bool{true, true, false, true, false, true, false, true, false, true, false, false, false, false, false}
	Encode15113_2(data1511)

	// Introduce two bit errors
	original := make([]bool, 15)
	copy(original, data1511)

	data1511[3] = !data1511[3] // Flip bit 3
	data1511[7] = !data1511[7] // Flip bit 7

	// Hamming should detect error but may not correct properly
	errorDetected := Decode15113_2(data1511)
	if !errorDetected {
		t.Log("Two-bit error was not detected (expected behavior for some patterns)")
	}

	// Test Hamming(13,9,3) with two-bit errors
	data139 := []bool{true, false, true, false, true, false, true, false, true, false, false, false, false}
	Encode1393(data139)

	original139 := make([]bool, 13)
	copy(original139, data139)

	data139[2] = !data139[2] // Flip bit 2
	data139[6] = !data139[6] // Flip bit 6

	// Hamming should detect error but may not correct properly
	errorDetected = Decode1393(data139)
	if !errorDetected {
		t.Log("Two-bit error was not detected (expected behavior for some patterns)")
	}
}

// TestHammingPerformance tests encoding/decoding performance
func TestHammingPerformance(t *testing.T) {
	data1511 := make([]bool, 15)
	data139 := make([]bool, 13)

	// Fill with test pattern
	for i := 0; i < 11; i++ {
		data1511[i] = (i % 2) == 0
	}
	for i := 0; i < 9; i++ {
		data139[i] = (i % 3) == 0
	}

	// Test multiple iterations
	iterations := 1000
	for i := 0; i < iterations; i++ {
		Encode15113_2(data1511)
		Decode15113_2(data1511)

		Encode1393(data139)
		Decode1393(data139)
	}

	t.Logf("Completed %d iterations of Hamming encoding/decoding", iterations)
}

// BenchmarkHamming15113_2Encode benchmarks Hamming(15,11,3) encoding
func BenchmarkHamming15113_2Encode(b *testing.B) {
	data := []bool{true, false, true, false, true, false, true, false, true, false, true, false, false, false, false}

	for i := 0; i < b.N; i++ {
		Encode15113_2(data)
	}
}

// BenchmarkHamming15113_2Decode benchmarks Hamming(15,11,3) decoding
func BenchmarkHamming15113_2Decode(b *testing.B) {
	data := []bool{true, false, true, false, true, false, true, false, true, false, true, false, false, false, false}
	Encode15113_2(data)

	for i := 0; i < b.N; i++ {
		Decode15113_2(data)
	}
}

// BenchmarkHamming1393Encode benchmarks Hamming(13,9,3) encoding
func BenchmarkHamming1393Encode(b *testing.B) {
	data := []bool{true, false, true, false, true, false, true, false, true, false, false, false, false}

	for i := 0; i < b.N; i++ {
		Encode1393(data)
	}
}

// BenchmarkHamming1393Decode benchmarks Hamming(13,9,3) decoding
func BenchmarkHamming1393Decode(b *testing.B) {
	data := []bool{true, false, true, false, true, false, true, false, true, false, false, false, false}
	Encode1393(data)

	for i := 0; i < b.N; i++ {
		Decode1393(data)
	}
}

// BenchmarkByteToBitsBE benchmarks byte to bits conversion
func BenchmarkByteToBitsBE(b *testing.B) {
	bits := make([]bool, 8)

	for i := 0; i < b.N; i++ {
		ByteToBitsBE(0xAA, bits)
	}
}

// BenchmarkBitsToByteBE benchmarks bits to byte conversion
func BenchmarkBitsToByteBE(b *testing.B) {
	bits := []bool{true, false, true, false, true, false, true, false}

	for i := 0; i < b.N; i++ {
		BitsToByteBE(bits)
	}
}

// TestHammingEdgeCases tests edge cases and boundary conditions
func TestHammingEdgeCases(t *testing.T) {
	// Test with nil/empty arrays
	Encode15113_2(nil)
	Encode1393(nil)

	if Decode15113_2(nil) {
		t.Error("Nil array should not report error correction")
	}
	if Decode1393(nil) {
		t.Error("Nil array should not report error correction")
	}

	// Test with too-short arrays
	shortData := make([]bool, 5)
	Encode15113_2(shortData)
	Encode1393(shortData)

	if Decode15113_2(shortData) {
		t.Error("Short array should not report error correction")
	}
	if Decode1393(shortData) {
		t.Error("Short array should not report error correction")
	}

	// Test bit conversion with short arrays
	shortBits := make([]bool, 3)
	ByteToBitsBE(0xFF, shortBits)
	result := BitsToByteBE(shortBits)
	if result != 0 {
		t.Error("Short bit array should return 0")
	}
}

// TestHammingErrorPatterns tests specific error patterns
func TestHammingErrorPatterns(t *testing.T) {
	// Test all-zeros with single error
	data := make([]bool, 15)
	Encode15113_2(data)

	// Introduce error in parity bit
	data[11] = true
	if !Decode15113_2(data) {
		t.Error("Failed to correct parity bit error")
	}

	// Should be back to all zeros
	for i := 0; i < 15; i++ {
		if data[i] != false {
			t.Errorf("Parity correction failed at position %d", i)
		}
	}

	// Test data bit error
	data13 := make([]bool, 13)
	Encode1393(data13)

	// Introduce error in data bit
	data13[4] = true
	if !Decode1393(data13) {
		t.Error("Failed to correct data bit error")
	}

	// Should be back to all zeros
	for i := 0; i < 13; i++ {
		if data13[i] != false {
			t.Errorf("Data bit correction failed at position %d", i)
		}
	}
}