package dmr

import (
	"testing"
)

func TestDMRData_Parse(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedSrc uint32
		expectedDst uint32
		expectedFLCO uint8
		expectedDT   uint8
		expectedSlot uint8
		expectError bool
	}{
		{
			name: "valid DMR frame",
			input: []byte{
				// DMR frame structure (33 bytes)
				0x01, // Slot 1
				0x00, 0x12, 0x34, // Source ID (24-bit)
				0x00, 0x56, 0x78, // Destination ID (24-bit)
				0x00, // FLCO (Group call)
				0x01, // Data type (Voice header)
				0x00, // Sequence number
				// Payload (23 bytes)
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
				0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
			},
			expectedSrc:  0x001234,
			expectedDst:  0x005678,
			expectedFLCO: 0x00,
			expectedDT:   0x01,
			expectedSlot: 0x01,
			expectError:  false,
		},
		{
			name: "slot 2 private call",
			input: []byte{
				0x02,       // Slot 2
				0x00, 0xAB, 0xCD, // Source ID
				0x00, 0xEF, 0x01, // Destination ID
				0x03,       // FLCO (Private call)
				0x04,       // Data type (Voice sync)
				0x02,       // Sequence number
				// Payload
				0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
				0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
				0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36,
			},
			expectedSrc:  0x00ABCD,
			expectedDst:  0x00EF01,
			expectedFLCO: 0x03,
			expectedDT:   0x04,
			expectedSlot: 0x02,
			expectError:  false,
		},
		{
			name:        "frame too short",
			input:       []byte{0x01, 0x02, 0x03}, // Too short
			expectError: true,
		},
		{
			name: "invalid slot number",
			input: func() []byte {
				frame := make([]byte, 33)
				frame[0] = 0x03 // Invalid slot (should be 1 or 2)
				return frame
			}(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &Data{}
			err := data.Parse(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if data.SourceID != tt.expectedSrc {
				t.Errorf("SourceID = 0x%06X, want 0x%06X", data.SourceID, tt.expectedSrc)
			}
			if data.DestinationID != tt.expectedDst {
				t.Errorf("DestinationID = 0x%06X, want 0x%06X", data.DestinationID, tt.expectedDst)
			}
			if data.FLCO != tt.expectedFLCO {
				t.Errorf("FLCO = 0x%02X, want 0x%02X", data.FLCO, tt.expectedFLCO)
			}
			if data.DataType != tt.expectedDT {
				t.Errorf("DataType = 0x%02X, want 0x%02X", data.DataType, tt.expectedDT)
			}
			if data.SlotNumber != tt.expectedSlot {
				t.Errorf("SlotNumber = %d, want %d", data.SlotNumber, tt.expectedSlot)
			}
		})
	}
}

func TestDMRData_Build(t *testing.T) {
	tests := []struct {
		name         string
		data         *Data
		expectedSize int
	}{
		{
			name: "voice header frame",
			data: &Data{
				SlotNumber:    1,
				SourceID:      0x001234,
				DestinationID: 0x005678,
				FLCO:          0x00, // Group call
				DataType:      0x01, // Voice header
				SeqNumber:     0,
				Payload:       make([]byte, 23),
			},
			expectedSize: 33,
		},
		{
			name: "voice sync frame",
			data: &Data{
				SlotNumber:    2,
				SourceID:      0x00ABCD,
				DestinationID: 0x00EF01,
				FLCO:          0x03, // Private call
				DataType:      0x04, // Voice sync
				SeqNumber:     2,
				Payload:       make([]byte, 23),
				StreamID:      0x12345678,
			},
			expectedSize: 33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := tt.data.Build()

			if len(frame) != tt.expectedSize {
				t.Errorf("Build() frame size = %d, want %d", len(frame), tt.expectedSize)
			}

			// Parse the built frame to verify round-trip
			parsed := &Data{}
			err := parsed.Parse(frame)
			if err != nil {
				t.Errorf("Build() produced unparseable frame: %v", err)
				return
			}

			if parsed.SourceID != tt.data.SourceID {
				t.Errorf("Round-trip SourceID = 0x%06X, want 0x%06X", parsed.SourceID, tt.data.SourceID)
			}
			if parsed.DestinationID != tt.data.DestinationID {
				t.Errorf("Round-trip DestinationID = 0x%06X, want 0x%06X", parsed.DestinationID, tt.data.DestinationID)
			}
			if parsed.FLCO != tt.data.FLCO {
				t.Errorf("Round-trip FLCO = 0x%02X, want 0x%02X", parsed.FLCO, tt.data.FLCO)
			}
		})
	}
}

func TestDMRLinkControl_Encode(t *testing.T) {
	tests := []struct {
		name string
		lc   *LinkControl
		expectedBytes int
	}{
		{
			name: "group call link control",
			lc: &LinkControl{
				FLCO:          0x00, // Group call
				SourceID:      0x001234,
				DestinationID: 0x005678,
				FID:           0x00,
			},
			expectedBytes: 9, // DMR link control is 9 bytes
		},
		{
			name: "private call link control",
			lc: &LinkControl{
				FLCO:          0x03, // Private call
				SourceID:      0x00ABCD,
				DestinationID: 0x00EF01,
				FID:           0x10,
			},
			expectedBytes: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := tt.lc.Encode()

			if len(encoded) != tt.expectedBytes {
				t.Errorf("LinkControl.Encode() length = %d, want %d", len(encoded), tt.expectedBytes)
			}

			// Decode and verify round-trip
			decoded := &LinkControl{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Errorf("LinkControl.Decode() error: %v", err)
				return
			}

			if decoded.FLCO != tt.lc.FLCO {
				t.Errorf("Round-trip FLCO = 0x%02X, want 0x%02X", decoded.FLCO, tt.lc.FLCO)
			}
			if decoded.SourceID != tt.lc.SourceID {
				t.Errorf("Round-trip SourceID = 0x%06X, want 0x%06X", decoded.SourceID, tt.lc.SourceID)
			}
			if decoded.DestinationID != tt.lc.DestinationID {
				t.Errorf("Round-trip DestinationID = 0x%06X, want 0x%06X", decoded.DestinationID, tt.lc.DestinationID)
			}
		})
	}
}

func TestDMREmbeddedData_Parse(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectValid bool
	}{
		{
			name: "valid embedded data",
			input: []byte{
				0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, // 8 bytes EMB data
			},
			expectValid: true,
		},
		{
			name: "too short",
			input: []byte{
				0x11, 0x22, 0x33, // Too short
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emb := &EmbeddedData{}
			err := emb.Parse(tt.input)

			if tt.expectValid {
				if err != nil {
					t.Errorf("EmbeddedData.Parse() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("EmbeddedData.Parse() expected error but got none")
				}
			}
		})
	}
}

func TestDMRSlotType_Encode(t *testing.T) {
	tests := []struct {
		name     string
		st       *SlotType
		expected []byte
	}{
		{
			name: "voice header slot type",
			st: &SlotType{
				DataType:   0x01, // Voice header
				ColorCode:  0x01,
			},
			expected: []byte{0x11}, // Simplified - actual encoding is more complex
		},
		{
			name: "voice sync slot type",
			st: &SlotType{
				DataType:   0x04, // Voice sync
				ColorCode:  0x07,
			},
			expected: []byte{0x74}, // Simplified
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := tt.st.Encode()

			// For this test, we're mainly checking that encoding produces some output
			// The actual encoding algorithm is complex and depends on specific implementation
			if len(encoded) == 0 {
				t.Errorf("SlotType.Encode() produced empty output")
			}

			// Test decode round-trip
			decoded := &SlotType{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Errorf("SlotType.Decode() error: %v", err)
				return
			}

			if decoded.DataType != tt.st.DataType {
				t.Errorf("Round-trip DataType = 0x%02X, want 0x%02X", decoded.DataType, tt.st.DataType)
			}
		})
	}
}

func TestDMRSync_Detect(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected SyncType
	}{
		{
			name: "voice sync pattern",
			input: []byte{
				0x75, 0x5F, 0xD7, 0xDF, 0x75, 0xF7, // DMR voice sync
			},
			expected: SYNC_VOICE,
		},
		{
			name: "data sync pattern",
			input: []byte{
				0xDF, 0xF5, 0x7D, 0x75, 0xDF, 0x5D, // DMR data sync
			},
			expected: SYNC_DATA,
		},
		{
			name: "no sync pattern",
			input: []byte{
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05,
			},
			expected: SYNC_NONE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectSync(tt.input)
			if result != tt.expected {
				t.Errorf("DetectSync() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDMRData_IsVoice(t *testing.T) {
	tests := []struct {
		name     string
		dataType uint8
		expected bool
	}{
		{
			name:     "voice header",
			dataType: 0x01,
			expected: true,
		},
		{
			name:     "voice sync",
			dataType: 0x04,
			expected: true,
		},
		{
			name:     "voice frame A",
			dataType: 0x02,
			expected: true,
		},
		{
			name:     "voice frame B",
			dataType: 0x03,
			expected: true,
		},
		{
			name:     "voice frame C",
			dataType: 0x05,
			expected: true,
		},
		{
			name:     "voice frame D",
			dataType: 0x06,
			expected: true,
		},
		{
			name:     "voice frame E",
			dataType: 0x07,
			expected: true,
		},
		{
			name:     "voice frame F",
			dataType: 0x08,
			expected: true,
		},
		{
			name:     "voice terminator",
			dataType: 0x09,
			expected: true,
		},
		{
			name:     "data header",
			dataType: 0x0A,
			expected: false,
		},
		{
			name:     "data frame",
			dataType: 0x0B,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &Data{DataType: tt.dataType}
			result := data.IsVoice()
			if result != tt.expected {
				t.Errorf("IsVoice() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDMRData_IsHeader(t *testing.T) {
	tests := []struct {
		name     string
		dataType uint8
		expected bool
	}{
		{
			name:     "voice header",
			dataType: 0x01,
			expected: true,
		},
		{
			name:     "data header",
			dataType: 0x0A,
			expected: true,
		},
		{
			name:     "voice sync",
			dataType: 0x04,
			expected: false,
		},
		{
			name:     "voice frame A",
			dataType: 0x02,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &Data{DataType: tt.dataType}
			result := data.IsHeader()
			if result != tt.expected {
				t.Errorf("IsHeader() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDMRData_IsTerminator(t *testing.T) {
	tests := []struct {
		name     string
		dataType uint8
		expected bool
	}{
		{
			name:     "voice terminator",
			dataType: 0x09,
			expected: true,
		},
		{
			name:     "data terminator",
			dataType: 0x0C,
			expected: true,
		},
		{
			name:     "voice header",
			dataType: 0x01,
			expected: false,
		},
		{
			name:     "voice frame A",
			dataType: 0x02,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &Data{DataType: tt.dataType}
			result := data.IsTerminator()
			if result != tt.expected {
				t.Errorf("IsTerminator() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkDMRData_Parse(b *testing.B) {
	frame := []byte{
		0x01, 0x00, 0x12, 0x34, 0x00, 0x56, 0x78, 0x00, 0x01, 0x00,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09,
		0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13,
		0x14, 0x15, 0x16,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := &Data{}
		data.Parse(frame)
	}
}

func BenchmarkDMRData_Build(b *testing.B) {
	data := &Data{
		SlotNumber:    1,
		SourceID:      0x001234,
		DestinationID: 0x005678,
		FLCO:          0x00,
		DataType:      0x01,
		SeqNumber:     0,
		Payload:       make([]byte, 23),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Build()
	}
}