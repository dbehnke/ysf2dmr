package codec

import (
	"testing"
)

func TestAMBEConverter_YSFToDMR(t *testing.T) {
	tests := []struct {
		name         string
		ysfFrames    [][]byte // YSF AMBE frames (162 bits each, packed as bytes)
		expectedDMR  int      // Expected number of DMR frames output
		expectError  bool
	}{
		{
			name: "single YSF frame conversion",
			ysfFrames: [][]byte{
				make([]byte, 90), // Full YSF payload (90 bytes)
			},
			expectedDMR: 2, // YSF frame should produce 2 DMR frames
			expectError: false,
		},
		{
			name: "multiple YSF frames",
			ysfFrames: [][]byte{
				make([]byte, 90),
				make([]byte, 90),
				make([]byte, 90),
			},
			expectedDMR: 6, // 3 YSF frames * 2 DMR frames each
			expectError: false,
		},
		{
			name: "empty input",
			ysfFrames: [][]byte{},
			expectedDMR: 0,
			expectError: false,
		},
		{
			name: "invalid YSF frame size",
			ysfFrames: [][]byte{
				make([]byte, 10), // Too short
			},
			expectedDMR: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewAMBEConverter()

			var dmrFrames [][]byte
			var err error

			for _, ysfFrame := range tt.ysfFrames {
				frames, convErr := converter.YSFToDMR(ysfFrame)
				if convErr != nil {
					err = convErr
					break
				}
				dmrFrames = append(dmrFrames, frames...)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("YSFToDMR() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("YSFToDMR() unexpected error: %v", err)
				return
			}

			if len(dmrFrames) != tt.expectedDMR {
				t.Errorf("YSFToDMR() produced %d DMR frames, want %d", len(dmrFrames), tt.expectedDMR)
			}

			// Verify each DMR frame is the correct size (108 bits = 14 bytes rounded up)
			for i, frame := range dmrFrames {
				if len(frame) != 14 {
					t.Errorf("DMR frame %d has size %d, want 14", i, len(frame))
				}
			}
		})
	}
}

func TestAMBEConverter_DMRToYSF(t *testing.T) {
	tests := []struct {
		name        string
		dmrFrames   [][]byte // DMR AMBE frames (108 bits each)
		expectedYSF int      // Expected number of YSF frames
		expectError bool
	}{
		{
			name: "two DMR frames to one YSF",
			dmrFrames: [][]byte{
				make([]byte, 14), // 108 bits = 14 bytes (rounded up)
				make([]byte, 14),
			},
			expectedYSF: 1, // 2 DMR frames produce 1 YSF frame
			expectError: false,
		},
		{
			name: "four DMR frames to two YSF",
			dmrFrames: [][]byte{
				make([]byte, 14),
				make([]byte, 14),
				make([]byte, 14),
				make([]byte, 14),
			},
			expectedYSF: 2,
			expectError: false,
		},
		{
			name: "odd number of DMR frames",
			dmrFrames: [][]byte{
				make([]byte, 14),
				make([]byte, 14),
				make([]byte, 14), // Odd number - should buffer the last one
			},
			expectedYSF: 1, // Only first pair converts
			expectError: false,
		},
		{
			name: "empty input",
			dmrFrames: [][]byte{},
			expectedYSF: 0,
			expectError: false,
		},
		{
			name: "invalid DMR frame size",
			dmrFrames: [][]byte{
				make([]byte, 8), // Too short
				make([]byte, 14), // Valid second frame
			},
			expectedYSF: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewAMBEConverter()

			var ysfFrames [][]byte
			var err error

			for i := 0; i < len(tt.dmrFrames); i += 2 {
				if i+1 < len(tt.dmrFrames) {
					frame, convErr := converter.DMRToYSF(tt.dmrFrames[i], tt.dmrFrames[i+1])
					if convErr != nil {
						err = convErr
						break
					}
					if frame != nil {
						ysfFrames = append(ysfFrames, frame)
					}
				}
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("DMRToYSF() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("DMRToYSF() unexpected error: %v", err)
				return
			}

			if len(ysfFrames) != tt.expectedYSF {
				t.Errorf("DMRToYSF() produced %d YSF frames, want %d", len(ysfFrames), tt.expectedYSF)
			}

			// Verify each YSF frame is the correct size (162 bits = 21 bytes rounded up)
			for i, frame := range ysfFrames {
				if len(frame) != 21 {
					t.Errorf("YSF frame %d has size %d, want 21", i, len(frame))
				}
			}
		})
	}
}

func TestAMBEConverter_ExtractYSFAudio(t *testing.T) {
	tests := []struct {
		name        string
		ysfPayload  []byte
		expectError bool
		expectFrames int
	}{
		{
			name: "valid YSF payload with 3 AMBE frames",
			ysfPayload: func() []byte {
				// YSF payload contains 3 AMBE frames of 54 bits each = 162 bits = 21 bytes (rounded up)
				payload := make([]byte, 90) // YSF payload is 90 bytes after FICH
				// Fill with test pattern
				for i := range payload {
					payload[i] = byte(i % 256)
				}
				return payload
			}(),
			expectError: false,
			expectFrames: 3,
		},
		{
			name: "empty payload",
			ysfPayload: []byte{},
			expectError: true,
			expectFrames: 0,
		},
		{
			name: "payload too short",
			ysfPayload: make([]byte, 20), // Too short for 3 AMBE frames
			expectError: true,
			expectFrames: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewAMBEConverter()

			frames, err := converter.ExtractYSFAudio(tt.ysfPayload)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExtractYSFAudio() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractYSFAudio() unexpected error: %v", err)
				return
			}

			if len(frames) != tt.expectFrames {
				t.Errorf("ExtractYSFAudio() extracted %d frames, want %d", len(frames), tt.expectFrames)
			}

			// Verify each frame is the expected size (54 bits = 7 bytes rounded up)
			for i, frame := range frames {
				if len(frame) != 7 {
					t.Errorf("AMBE frame %d has size %d, want 7", i, len(frame))
				}
			}
		})
	}
}

func TestAMBEConverter_ExtractDMRAudio(t *testing.T) {
	tests := []struct {
		name        string
		dmrPayload  []byte
		expectError bool
		expectFrame bool
	}{
		{
			name: "valid DMR payload with AMBE",
			dmrPayload: func() []byte {
				// DMR payload contains 1 AMBE frame of 108 bits = 14 bytes (rounded up)
				payload := make([]byte, 23) // DMR payload is 23 bytes
				// Fill with test pattern
				for i := range payload {
					payload[i] = byte(i % 256)
				}
				return payload
			}(),
			expectError: false,
			expectFrame: true,
		},
		{
			name: "empty payload",
			dmrPayload: []byte{},
			expectError: true,
			expectFrame: false,
		},
		{
			name: "payload too short",
			dmrPayload: make([]byte, 10), // Too short for AMBE frame
			expectError: true,
			expectFrame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewAMBEConverter()

			frame, err := converter.ExtractDMRAudio(tt.dmrPayload)

			if tt.expectError {
				if err == nil {
					t.Errorf("ExtractDMRAudio() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractDMRAudio() unexpected error: %v", err)
				return
			}

			if tt.expectFrame && frame == nil {
				t.Errorf("ExtractDMRAudio() expected frame but got nil")
			}
			if !tt.expectFrame && frame != nil {
				t.Errorf("ExtractDMRAudio() expected nil but got frame")
			}

			if frame != nil && len(frame) != 14 {
				t.Errorf("DMR AMBE frame has size %d, want 14", len(frame))
			}
		})
	}
}

func TestAMBEConverter_RoundTrip(t *testing.T) {
	// Test round-trip conversion: YSF -> DMR -> YSF
	converter := NewAMBEConverter()

	// Create test YSF frame
	originalYSF := make([]byte, 90) // Full YSF payload
	for i := range originalYSF {
		originalYSF[i] = byte(i % 256)
	}

	// Convert YSF to DMR
	dmrFrames, err := converter.YSFToDMR(originalYSF)
	if err != nil {
		t.Fatalf("YSF to DMR conversion failed: %v", err)
	}

	if len(dmrFrames) != 2 {
		t.Fatalf("Expected 2 DMR frames, got %d", len(dmrFrames))
	}

	// Convert DMR back to YSF
	convertedYSF, err := converter.DMRToYSF(dmrFrames[0], dmrFrames[1])
	if err != nil {
		t.Fatalf("DMR to YSF conversion failed: %v", err)
	}

	if convertedYSF == nil {
		t.Fatalf("DMR to YSF conversion returned nil")
	}

	// The converted YSF contains the extracted AMBE frames (21 bytes)
	// not the full payload (90 bytes), which is correct
	expectedSize := 21 // 3 AMBE frames * 7 bytes each = 21 bytes
	if len(convertedYSF) != expectedSize {
		t.Errorf("Round-trip YSF frame size = %d, want %d", len(convertedYSF), expectedSize)
	}

	// Note: Due to codec conversions, we don't expect bit-perfect round-trip,
	// but the frame should be the same size and roughly similar
}

func TestAMBEFrame_Validate(t *testing.T) {
	tests := []struct {
		name    string
		frame   []byte
		format  AMBEFormat
		isValid bool
	}{
		{
			name:    "valid YSF AMBE frame",
			frame:   make([]byte, 7), // 54 bits = 7 bytes rounded up
			format:  AMBE_YSF,
			isValid: true,
		},
		{
			name:    "valid DMR AMBE frame",
			frame:   make([]byte, 14), // 108 bits = 14 bytes rounded up
			format:  AMBE_DMR,
			isValid: true,
		},
		{
			name:    "invalid YSF frame - too short",
			frame:   make([]byte, 5),
			format:  AMBE_YSF,
			isValid: false,
		},
		{
			name:    "invalid DMR frame - too short",
			frame:   make([]byte, 10),
			format:  AMBE_DMR,
			isValid: false,
		},
		{
			name:    "invalid YSF frame - too long",
			frame:   make([]byte, 10),
			format:  AMBE_YSF,
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := ValidateAMBEFrame(tt.frame, tt.format)
			if isValid != tt.isValid {
				t.Errorf("ValidateAMBEFrame() = %v, want %v", isValid, tt.isValid)
			}
		})
	}
}

func TestAMBEConverter_BufferManagement(t *testing.T) {
	converter := NewAMBEConverter()

	// Test that converter handles buffering correctly
	dmrFrame1 := make([]byte, 14)
	for i := range dmrFrame1 {
		dmrFrame1[i] = 0xAA
	}

	// First frame should be buffered, no output yet
	result, err := converter.DMRToYSF(dmrFrame1, nil)
	if err != nil {
		t.Errorf("DMRToYSF() with single frame failed: %v", err)
	}
	if result != nil {
		t.Errorf("DMRToYSF() with single frame should return nil, got frame")
	}

	// Second frame should complete the pair and produce output
	dmrFrame2 := make([]byte, 14)
	for i := range dmrFrame2 {
		dmrFrame2[i] = 0x55
	}

	result, err = converter.DMRToYSF(dmrFrame1, dmrFrame2)
	if err != nil {
		t.Errorf("DMRToYSF() with frame pair failed: %v", err)
	}
	if result == nil {
		t.Errorf("DMRToYSF() with frame pair should return YSF frame, got nil")
	}
}

// Benchmark tests for performance - critical for real-time operation
func BenchmarkAMBEConverter_YSFToDMR(b *testing.B) {
	converter := NewAMBEConverter()
	ysfFrame := make([]byte, 21)
	for i := range ysfFrame {
		ysfFrame[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.YSFToDMR(ysfFrame)
	}
}

func BenchmarkAMBEConverter_DMRToYSF(b *testing.B) {
	converter := NewAMBEConverter()
	dmrFrame1 := make([]byte, 14)
	dmrFrame2 := make([]byte, 14)
	for i := range dmrFrame1 {
		dmrFrame1[i] = byte(i % 256)
		dmrFrame2[i] = byte((i + 128) % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.DMRToYSF(dmrFrame1, dmrFrame2)
	}
}

func BenchmarkAMBEConverter_ExtractYSFAudio(b *testing.B) {
	converter := NewAMBEConverter()
	payload := make([]byte, 90)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.ExtractYSFAudio(payload)
	}
}