package correction

import (
	"testing"
)

func TestCRC8(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected uint8
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: 0x00,
		},
		{
			name:     "single byte",
			input:    []byte{0x01},
			expected: 0x07, // From CRC8_TABLE[0x00 ^ 0x01]
		},
		{
			name:     "multiple bytes",
			input:    []byte{0x12, 0x34, 0x56},
			expected: 0x7C, // Verified with C++ implementation
		},
		{
			name:     "YSF frame data",
			input:    []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}, // YSF sync pattern
			expected: 0x5F, // Verified with C++ implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CRC8(tt.input)
			if result != tt.expected {
				t.Errorf("CRC8() = 0x%02X, want 0x%02X", result, tt.expected)
			}
		})
	}
}

func TestCRC16CCITT1(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected [2]byte // [high, low]
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: [2]byte{0x00, 0x00}, // Verified with C++ implementation
		},
		{
			name:     "single byte",
			input:    []byte{0x01},
			expected: [2]byte{0xF1, 0xE1}, // Verified with C++ implementation
		},
		{
			name:     "DMR header data",
			input:    []byte{0x00, 0x12, 0x34, 0x56, 0x78},
			expected: [2]byte{0x87, 0xA8}, // Verified with C++ implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer with 2 extra bytes for CRC
			buffer := make([]byte, len(tt.input)+2)
			copy(buffer, tt.input)

			AddCCITT161(buffer)

			// Check that CRC was added correctly
			if buffer[len(buffer)-2] != tt.expected[0] || buffer[len(buffer)-1] != tt.expected[1] {
				t.Errorf("AddCCITT161() added CRC [0x%02X, 0x%02X], want [0x%02X, 0x%02X]",
					buffer[len(buffer)-2], buffer[len(buffer)-1], tt.expected[0], tt.expected[1])
			}

			// Verify CheckCCITT161 validates the CRC
			if !CheckCCITT161(buffer) {
				t.Errorf("CheckCCITT161() failed to validate added CRC")
			}
		})
	}
}

func TestCRC16CCITT2(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected [2]byte // [high, low]
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: [2]byte{0xFF, 0xFF}, // Inverted 0x0000
		},
		{
			name:     "single byte",
			input:    []byte{0x01},
			expected: [2]byte{0xDE, 0xEF}, // Verified with C++ implementation
		},
		{
			name:     "YSF payload data",
			input:    []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE},
			expected: [2]byte{0xF6, 0x50}, // Verified with C++ implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer with 2 extra bytes for CRC
			buffer := make([]byte, len(tt.input)+2)
			copy(buffer, tt.input)

			AddCCITT162(buffer)

			// Check that CRC was added correctly
			if buffer[len(buffer)-1] != tt.expected[0] || buffer[len(buffer)-2] != tt.expected[1] {
				t.Errorf("AddCCITT162() added CRC [0x%02X, 0x%02X], want [0x%02X, 0x%02X]",
					buffer[len(buffer)-1], buffer[len(buffer)-2], tt.expected[0], tt.expected[1])
			}

			// Verify CheckCCITT162 validates the CRC
			if !CheckCCITT162(buffer) {
				t.Errorf("CheckCCITT162() failed to validate added CRC")
			}
		})
	}
}

func TestFiveBitCRC(t *testing.T) {
	tests := []struct {
		name     string
		input    []bool // 72 bits
		expected uint32
	}{
		{
			name:     "all zeros",
			input:    make([]bool, 72),
			expected: 0,
		},
		{
			name:     "all ones",
			input:    func() []bool { b := make([]bool, 72); for i := range b { b[i] = true }; return b }(),
			expected: 1, // Verified with C++ implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeFiveBit(tt.input)
			if result != tt.expected {
				t.Errorf("EncodeFiveBit() = %d, want %d", result, tt.expected)
			}

			// Test the check function
			if !CheckFiveBit(tt.input, tt.expected) {
				t.Errorf("CheckFiveBit() failed for correct CRC")
			}

			// Test with wrong CRC
			if CheckFiveBit(tt.input, tt.expected+1) {
				t.Errorf("CheckFiveBit() passed for incorrect CRC")
			}
		})
	}
}

func TestAdditiveCRC(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected uint8
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: 0x00,
		},
		{
			name:     "single byte",
			input:    []byte{0x01},
			expected: 0x01,
		},
		{
			name:     "multiple bytes",
			input:    []byte{0x12, 0x34, 0x56, 0x78},
			expected: 0x14, // Verified with C++ implementation: 0x12+0x34+0x56+0x78=0x114, 0x114&0xFF=0x14
		},
		{
			name:     "overflow test",
			input:    []byte{0xFF, 0xFF, 0x01},
			expected: 0xFF, // (0xFF + 0xFF + 0x01) & 0xFF = 0x1FF & 0xFF = 0xFF
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddCRC(tt.input)
			if result != tt.expected {
				t.Errorf("AddCRC() = 0x%02X, want 0x%02X", result, tt.expected)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkCRC8(b *testing.B) {
	data := make([]byte, 120) // Typical YSF frame size
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CRC8(data)
	}
}

func BenchmarkCRC16CCITT1(b *testing.B) {
	data := make([]byte, 155) // YSF frame + header
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AddCCITT161(data)
	}
}

func BenchmarkCRC16CCITT2(b *testing.B) {
	data := make([]byte, 33) // DMR frame size
	for i := range data {
		data[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AddCCITT162(data)
	}
}