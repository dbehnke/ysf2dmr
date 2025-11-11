package correction

import (
	"testing"
)

func TestGolay2087_Encode(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "all zeros",
			input: []byte{0x00, 0x00, 0x00},
		},
		{
			name:  "alternating pattern",
			input: []byte{0xAA, 0xAA, 0xAA},
		},
		{
			name:  "ascending pattern",
			input: []byte{0x12, 0x34, 0x56},
		},
		{
			name:  "YSF sync pattern",
			input: []byte{0xD4, 0x71, 0xC9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]byte, len(tt.input))
			copy(data, tt.input)

			// Store original for comparison
			original := make([]byte, len(tt.input))
			copy(original, tt.input)

			// Encode the data (should add parity bits)
			Golay2087Encode(data)

			// The data should be modified by encoding
			// We can't predict the exact output without implementing the algorithm,
			// but we can test that decoding works

			// Test that decoding returns no errors for clean data
			errors := Golay2087Decode(data)
			if errors != 0 {
				t.Errorf("Golay2087Decode() reported %d errors for clean encoded data", errors)
			}
		})
	}
}

func TestGolay2087_Decode(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		corruptBit  int // bit position to corrupt (-1 for no corruption)
		expectFix   bool
		maxErrors   uint8
	}{
		{
			name:       "no corruption",
			input:      []byte{0x00, 0x00, 0x00},
			corruptBit: -1,
			expectFix:  true,
			maxErrors:  0,
		},
		{
			name:       "single bit error",
			input:      []byte{0x12, 0x34, 0x56},
			corruptBit: 7, // corrupt bit 7 of first byte
			expectFix:  true,
			maxErrors:  1,
		},
		{
			name:       "bit error in second byte",
			input:      []byte{0xAA, 0xBB, 0xCC},
			corruptBit: 15, // corrupt bit 7 of second byte (8+7)
			expectFix:  true,
			maxErrors:  1,
		},
		{
			name:       "bit error in third byte",
			input:      []byte{0xFF, 0x00, 0x55},
			corruptBit: 20, // corrupt bit 4 of third byte (16+4)
			expectFix:  true,
			maxErrors:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]byte, len(tt.input))
			copy(data, tt.input)

			// Encode first to create valid codeword
			Golay2087Encode(data)

			// Store the encoded version for comparison
			encoded := make([]byte, len(data))
			copy(encoded, data)

			// Corrupt a bit if specified
			if tt.corruptBit >= 0 && tt.corruptBit < len(data)*8 {
				byteIdx := tt.corruptBit / 8
				bitIdx := tt.corruptBit % 8
				data[byteIdx] ^= (1 << uint(bitIdx))
			}

			// Decode and check error count
			errors := Golay2087Decode(data)

			if tt.expectFix {
				if errors > tt.maxErrors {
					t.Errorf("Golay2087Decode() reported %d errors, expected <= %d", errors, tt.maxErrors)
				}
				// If we expect a fix and corrupted a bit, verify correction
				if tt.corruptBit >= 0 && errors <= tt.maxErrors {
					// The data should be corrected back to the encoded version
					for i, b := range data {
						if b != encoded[i] {
							t.Errorf("Golay2087Decode() failed to correct byte %d: got 0x%02X, want 0x%02X", i, b, encoded[i])
						}
					}
				}
			}
		})
	}
}

func TestGolay24128_Encode(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "all zeros",
			input: []byte{0x00, 0x00, 0x00},
		},
		{
			name:  "test pattern 1",
			input: []byte{0x5A, 0xA5, 0x5A},
		},
		{
			name:  "test pattern 2",
			input: []byte{0xFF, 0x00, 0xFF},
		},
		{
			name:  "YSF FICH data",
			input: []byte{0x71, 0xC9, 0x63},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]byte, len(tt.input))
			copy(data, tt.input)

			// Store original for comparison
			original := make([]byte, len(tt.input))
			copy(original, tt.input)

			// Encode the data (should add parity bits)
			Golay24128Encode(data)

			// Test that decoding works on clean data
			errors := Golay24128Decode(data)
			if errors != 0 {
				t.Errorf("Golay24128Decode() reported %d errors for clean encoded data", errors)
			}
		})
	}
}

func TestGolay24128_Decode(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		corruptBits []int // bit positions to corrupt
		expectFix   bool
		maxErrors   uint8
	}{
		{
			name:       "no corruption",
			input:      []byte{0x00, 0x00, 0x00},
			corruptBits: []int{},
			expectFix:  true,
			maxErrors:  0,
		},
		{
			name:       "single bit error",
			input:      []byte{0x12, 0x34, 0x56},
			corruptBits: []int{3},
			expectFix:  true,
			maxErrors:  1,
		},
		{
			name:       "double bit error",
			input:      []byte{0xAA, 0x55, 0xAA},
			corruptBits: []int{5, 13},
			expectFix:  true,
			maxErrors:  2,
		},
		{
			name:       "triple bit error",
			input:      []byte{0xFF, 0x00, 0xFF},
			corruptBits: []int{2, 10, 18},
			expectFix:  true,
			maxErrors:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]byte, len(tt.input))
			copy(data, tt.input)

			// Encode first to create valid codeword
			Golay24128Encode(data)

			// Store the encoded version for comparison
			encoded := make([]byte, len(data))
			copy(encoded, data)

			// Corrupt specified bits
			for _, bitPos := range tt.corruptBits {
				if bitPos >= 0 && bitPos < len(data)*8 {
					byteIdx := bitPos / 8
					bitIdx := bitPos % 8
					data[byteIdx] ^= (1 << uint(bitIdx))
				}
			}

			// Decode and check error count
			errors := Golay24128Decode(data)

			if tt.expectFix {
				if errors > tt.maxErrors {
					t.Errorf("Golay24128Decode() reported %d errors, expected <= %d", errors, tt.maxErrors)
				}
				// If we expect a fix and corrupted bits, verify correction
				if len(tt.corruptBits) > 0 && errors <= tt.maxErrors {
					// The data should be corrected back to the encoded version
					for i, b := range data {
						if b != encoded[i] {
							t.Errorf("Golay24128Decode() failed to correct byte %d: got 0x%02X, want 0x%02X", i, b, encoded[i])
						}
					}
				}
			}
		})
	}
}

// Test error detection beyond correction capability
func TestGolayUncorrectableErrors(t *testing.T) {
	data := []byte{0x12, 0x34, 0x56}

	// Encode first
	Golay24128Encode(data)

	// Corrupt too many bits for Golay (24,12,8) to correct (more than 3 errors)
	data[0] ^= 0x0F // corrupt 4 bits in first byte

	// Should detect errors but may not correct them all
	errors := Golay24128Decode(data)
	// We expect errors to be reported, exact number depends on implementation
	if errors == 0 {
		t.Errorf("Golay24128Decode() should have detected errors for 4-bit corruption")
	}
}

// Test Golay syndrome calculation (internal function testing if exposed)
func TestGolaySyndrome(t *testing.T) {
	// Test that syndrome is 0 for valid codewords
	data := []byte{0x00, 0x00, 0x00}
	Golay2087Encode(data)

	// Valid codeword should have syndrome 0
	errors := Golay2087Decode(data)
	if errors != 0 {
		t.Errorf("Valid codeword should have 0 errors, got %d", errors)
	}
}

// Benchmark tests for performance
func BenchmarkGolay2087_Encode(b *testing.B) {
	data := []byte{0x12, 0x34, 0x56}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := make([]byte, 3)
		copy(testData, data)
		Golay2087Encode(testData)
	}
}

func BenchmarkGolay2087_Decode(b *testing.B) {
	data := []byte{0x12, 0x34, 0x56}
	Golay2087Encode(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := make([]byte, 3)
		copy(testData, data)
		Golay2087Decode(testData)
	}
}

func BenchmarkGolay24128_Encode(b *testing.B) {
	data := []byte{0x12, 0x34, 0x56}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := make([]byte, 3)
		copy(testData, data)
		Golay24128Encode(testData)
	}
}

func BenchmarkGolay24128_Decode(b *testing.B) {
	data := []byte{0x12, 0x34, 0x56}
	Golay24128Encode(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := make([]byte, 3)
		copy(testData, data)
		Golay24128Decode(testData)
	}
}