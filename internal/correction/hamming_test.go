package correction

import (
	"testing"
)

func TestHamming15113_1(t *testing.T) {
	tests := []struct {
		name    string
		input   []bool // 11 data bits + 4 parity bits = 15 bits total
		corrupt int    // bit position to corrupt (-1 for no corruption)
		want    bool   // expected decode success
	}{
		{
			name:    "all zeros uncorrupted",
			input:   make([]bool, 15),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "all zeros with single bit error",
			input:   make([]bool, 15),
			corrupt: 5, // corrupt bit 5
			want:    true,
		},
		{
			name:    "data pattern uncorrupted",
			input:   []bool{true, false, true, false, true, false, true, false, true, false, true, false, false, false, false},
			corrupt: -1,
			want:    true,
		},
		{
			name:    "data pattern with error in data bit",
			input:   []bool{true, false, true, false, true, false, true, false, true, false, true, false, false, false, false},
			corrupt: 3, // corrupt data bit
			want:    true,
		},
		{
			name:    "data pattern with error in parity bit",
			input:   []bool{true, false, true, false, true, false, true, false, true, false, true, false, false, false, false},
			corrupt: 12, // corrupt parity bit
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]bool, len(tt.input))
			copy(data, tt.input)

			// Encode the data (adds parity bits)
			original := make([]bool, len(data))
			copy(original, data)
			Encode15113_1(data)

			// Verify encoding worked by decoding uncorrupted data
			testData := make([]bool, len(data))
			copy(testData, data)
			if !Decode15113_1(testData) {
				t.Errorf("Decode15113_1() failed on clean encoded data")
				return
			}

			// Corrupt a bit if specified
			if tt.corrupt >= 0 && tt.corrupt < len(data) {
				data[tt.corrupt] = !data[tt.corrupt]
			}

			// Test decoding
			result := Decode15113_1(data)
			if result != tt.want {
				t.Errorf("Decode15113_1() = %v, want %v", result, tt.want)
			}

			// If decode was successful and we corrupted a bit, verify correction
			if result && tt.corrupt >= 0 {
				// The data should be corrected back to the original
				for i := 0; i < 11; i++ { // Only check data bits
					if data[i] != testData[i] {
						t.Errorf("Decode15113_1() failed to correct bit %d: got %v, want %v", i, data[i], testData[i])
					}
				}
			}
		})
	}
}

func TestHamming15113_2(t *testing.T) {
	tests := []struct {
		name    string
		input   []bool // 11 data bits + 4 parity bits = 15 bits total
		corrupt int    // bit position to corrupt (-1 for no corruption)
		want    bool   // expected decode success
	}{
		{
			name:    "all zeros uncorrupted",
			input:   make([]bool, 15),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "all ones data bits",
			input:   func() []bool { b := make([]bool, 15); for i := 0; i < 11; i++ { b[i] = true }; return b }(),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "pattern with single error",
			input:   []bool{false, true, false, true, false, true, false, true, false, true, false, false, false, false, false},
			corrupt: 7,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]bool, len(tt.input))
			copy(data, tt.input)

			// Encode the data (adds parity bits)
			original := make([]bool, len(data))
			copy(original, data)
			Encode15113_2(data)

			// Verify encoding worked by decoding uncorrupted data
			testData := make([]bool, len(data))
			copy(testData, data)
			if !Decode15113_2(testData) {
				t.Errorf("Decode15113_2() failed on clean encoded data")
				return
			}

			// Corrupt a bit if specified
			if tt.corrupt >= 0 && tt.corrupt < len(data) {
				data[tt.corrupt] = !data[tt.corrupt]
			}

			// Test decoding
			result := Decode15113_2(data)
			if result != tt.want {
				t.Errorf("Decode15113_2() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestHamming1393(t *testing.T) {
	tests := []struct {
		name    string
		input   []bool // 9 data bits + 4 parity bits = 13 bits total
		corrupt int    // bit position to corrupt (-1 for no corruption)
		want    bool   // expected decode success
	}{
		{
			name:    "all zeros uncorrupted",
			input:   make([]bool, 13),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "data pattern",
			input:   []bool{true, true, false, false, true, true, false, false, true, false, false, false, false},
			corrupt: -1,
			want:    true,
		},
		{
			name:    "data pattern with error",
			input:   []bool{true, true, false, false, true, true, false, false, true, false, false, false, false},
			corrupt: 4,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]bool, len(tt.input))
			copy(data, tt.input)

			// Encode the data
			Encode1393(data)

			// Verify encoding worked
			testData := make([]bool, len(data))
			copy(testData, data)
			if !Decode1393(testData) {
				t.Errorf("Decode1393() failed on clean encoded data")
				return
			}

			// Corrupt a bit if specified
			if tt.corrupt >= 0 && tt.corrupt < len(data) {
				data[tt.corrupt] = !data[tt.corrupt]
			}

			// Test decoding
			result := Decode1393(data)
			if result != tt.want {
				t.Errorf("Decode1393() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestHamming1063(t *testing.T) {
	tests := []struct {
		name    string
		input   []bool // 6 data bits + 4 parity bits = 10 bits total
		corrupt int    // bit position to corrupt (-1 for no corruption)
		want    bool   // expected decode success
	}{
		{
			name:    "all zeros uncorrupted",
			input:   make([]bool, 10),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "all data bits set",
			input:   []bool{true, true, true, true, true, true, false, false, false, false},
			corrupt: -1,
			want:    true,
		},
		{
			name:    "pattern with error in data",
			input:   []bool{true, false, true, false, true, false, false, false, false, false},
			corrupt: 2,
			want:    true,
		},
		{
			name:    "pattern with error in parity",
			input:   []bool{true, false, true, false, true, false, false, false, false, false},
			corrupt: 8,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]bool, len(tt.input))
			copy(data, tt.input)

			// Encode the data
			Encode1063(data)

			// Verify encoding worked
			testData := make([]bool, len(data))
			copy(testData, data)
			if !Decode1063(testData) {
				t.Errorf("Decode1063() failed on clean encoded data")
				return
			}

			// Corrupt a bit if specified
			if tt.corrupt >= 0 && tt.corrupt < len(data) {
				data[tt.corrupt] = !data[tt.corrupt]
			}

			// Test decoding
			result := Decode1063(data)
			if result != tt.want {
				t.Errorf("Decode1063() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestHamming16114(t *testing.T) {
	tests := []struct {
		name    string
		input   []bool // 11 data bits + 5 parity bits = 16 bits total
		corrupt int    // bit position to corrupt (-1 for no corruption)
		want    bool   // expected decode success
	}{
		{
			name:    "all zeros uncorrupted",
			input:   make([]bool, 16),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "mixed data pattern",
			input:   []bool{true, false, true, true, false, false, true, false, true, false, true, false, false, false, false, false},
			corrupt: -1,
			want:    true,
		},
		{
			name:    "pattern with single error",
			input:   []bool{true, false, true, true, false, false, true, false, true, false, true, false, false, false, false, false},
			corrupt: 6,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]bool, len(tt.input))
			copy(data, tt.input)

			// Encode the data
			Encode16114(data)

			// Verify encoding worked
			testData := make([]bool, len(data))
			copy(testData, data)
			if !Decode16114(testData) {
				t.Errorf("Decode16114() failed on clean encoded data")
				return
			}

			// Corrupt a bit if specified
			if tt.corrupt >= 0 && tt.corrupt < len(data) {
				data[tt.corrupt] = !data[tt.corrupt]
			}

			// Test decoding
			result := Decode16114(data)
			if result != tt.want {
				t.Errorf("Decode16114() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestHamming17123(t *testing.T) {
	tests := []struct {
		name    string
		input   []bool // 12 data bits + 5 parity bits = 17 bits total
		corrupt int    // bit position to corrupt (-1 for no corruption)
		want    bool   // expected decode success
	}{
		{
			name:    "all zeros uncorrupted",
			input:   make([]bool, 17),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "alternating pattern",
			input:   func() []bool { b := make([]bool, 17); for i := 0; i < 12; i += 2 { b[i] = true }; return b }(),
			corrupt: -1,
			want:    true,
		},
		{
			name:    "pattern with error in middle",
			input:   func() []bool { b := make([]bool, 17); for i := 0; i < 12; i += 2 { b[i] = true }; return b }(),
			corrupt: 10,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			data := make([]bool, len(tt.input))
			copy(data, tt.input)

			// Encode the data
			Encode17123(data)

			// Verify encoding worked
			testData := make([]bool, len(data))
			copy(testData, data)
			if !Decode17123(testData) {
				t.Errorf("Decode17123() failed on clean encoded data")
				return
			}

			// Corrupt a bit if specified
			if tt.corrupt >= 0 && tt.corrupt < len(data) {
				data[tt.corrupt] = !data[tt.corrupt]
			}

			// Test decoding
			result := Decode17123(data)
			if result != tt.want {
				t.Errorf("Decode17123() = %v, want %v", result, tt.want)
			}
		})
	}
}

// Test multiple bit errors (should fail)
func TestHammingMultipleBitErrors(t *testing.T) {
	data := make([]bool, 15)
	for i := 0; i < 11; i += 2 {
		data[i] = true
	}

	// Encode the data
	Encode15113_1(data)

	// Corrupt two bits (should not be correctable)
	data[0] = !data[0]
	data[1] = !data[1]

	// This might or might not succeed depending on the specific error pattern
	// But we test to ensure it doesn't panic
	_ = Decode15113_1(data)
}

// Benchmark tests
func BenchmarkHamming15113_1_Encode(b *testing.B) {
	data := make([]bool, 15)
	for i := 0; i < 11; i++ {
		data[i] = i%2 == 0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := make([]bool, 15)
		copy(testData, data)
		Encode15113_1(testData)
	}
}

func BenchmarkHamming15113_1_Decode(b *testing.B) {
	data := make([]bool, 15)
	for i := 0; i < 11; i++ {
		data[i] = i%2 == 0
	}
	Encode15113_1(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData := make([]bool, 15)
		copy(testData, data)
		Decode15113_1(testData)
	}
}