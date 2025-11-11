package ysf

import (
	"testing"
)

func TestYSFFrame_ParseHeader(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		expectedFICH FICH
		expectedErr  bool
	}{
		{
			name: "valid YSF header frame",
			input: []byte{
				// YSF header (35 bytes) + payload (120 bytes) = 155 bytes total
				'Y', 'S', 'F', 'D',                          // Magic number
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 10 bytes callsign
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 10 bytes callsign
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 11 bytes remaining header
				// FICH + payload (120 bytes) - using YSF sync + FICH + payload structure
				0xD4, 0x71, 0xC9, 0x63, 0x4D, // YSF sync pattern (5 bytes)
				// FICH (25 bytes) - Frame Information CHannel
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00,
				// Payload (90 bytes remaining)
			},
			expectedFICH: FICH{
				FI: 0, // Header frame
				DT: 0, // Voice/Data mode 1
				CM: 0, // Group call
				CS: 0, // Calling standards
				FN: 0, // Frame number
				FT: 0, // Frame type
				MR: 0, // Message route
			},
			expectedErr: false,
		},
		{
			name: "invalid magic number",
			input: []byte{
				'X', 'S', 'F', 'D', // Invalid magic
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			expectedErr: true,
		},
		{
			name:        "frame too short",
			input:       []byte{0x00, 0x01, 0x02}, // Too short
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pad input to minimum frame size if needed for parsing test
			if len(tt.input) < 155 {
				padded := make([]byte, 155)
				copy(padded, tt.input)
				tt.input = padded
			}

			frame := &Frame{}
			err := frame.Parse(tt.input)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			// Verify FICH fields if parsing succeeded
			if frame.FICH.FI != tt.expectedFICH.FI {
				t.Errorf("FICH.FI = %d, want %d", frame.FICH.FI, tt.expectedFICH.FI)
			}
			if frame.FICH.DT != tt.expectedFICH.DT {
				t.Errorf("FICH.DT = %d, want %d", frame.FICH.DT, tt.expectedFICH.DT)
			}
			if frame.FICH.CM != tt.expectedFICH.CM {
				t.Errorf("FICH.CM = %d, want %d", frame.FICH.CM, tt.expectedFICH.CM)
			}
		})
	}
}

func TestYSFFrame_ParseCommunications(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectFICH  FICH
		expectValid bool
	}{
		{
			name: "communications frame with voice data",
			input: func() []byte {
				frame := make([]byte, 155)
				copy(frame[:4], []byte{'Y', 'S', 'F', 'D'})
				// Fill in callsign
				copy(frame[4:14], []byte("G4KLX     "))
				copy(frame[14:24], []byte("G4KLX     "))
				// Set YSF sync pattern at offset 35
				copy(frame[35:40], []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D})
				// FICH bytes (25 bytes starting at offset 40)
				frame[40] = 0x40 // FI=1 (communications), other fields 0
				return frame
			}(),
			expectFICH: FICH{
				FI: 1, // Communications frame
				DT: 0, // Voice/Data mode 1
				CM: 0, // Group call
				FN: 0, // Frame number 0
			},
			expectValid: true,
		},
		{
			name: "terminator frame",
			input: func() []byte {
				frame := make([]byte, 155)
				copy(frame[:4], []byte{'Y', 'S', 'F', 'D'})
				copy(frame[4:14], []byte("VK3DRS    "))
				copy(frame[14:24], []byte("VK3DRS    "))
				copy(frame[35:40], []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D})
				frame[40] = 0x80 // FI=2 (terminator)
				return frame
			}(),
			expectFICH: FICH{
				FI: 2, // Terminator frame
				DT: 0,
				CM: 0,
				FN: 0,
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &Frame{}
			err := frame.Parse(tt.input)

			if !tt.expectValid {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if frame.FICH.FI != tt.expectFICH.FI {
				t.Errorf("FICH.FI = %d, want %d", frame.FICH.FI, tt.expectFICH.FI)
			}
		})
	}
}

func TestYSFFrame_ExtractCallsign(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		expectedSrc  string
		expectedDest string
	}{
		{
			name: "valid callsigns",
			input: func() []byte {
				frame := make([]byte, 155)
				copy(frame[:4], []byte{'Y', 'S', 'F', 'D'})
				copy(frame[4:14], []byte("G4KLX     "))  // Source
				copy(frame[14:24], []byte("VK3DRS    ")) // Dest
				copy(frame[35:40], []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}) // YSF sync
				return frame
			}(),
			expectedSrc:  "G4KLX",
			expectedDest: "VK3DRS",
		},
		{
			name: "short callsigns",
			input: func() []byte {
				frame := make([]byte, 155)
				copy(frame[:4], []byte{'Y', 'S', 'F', 'D'})
				copy(frame[4:14], []byte("VK3A      "))
				copy(frame[14:24], []byte("G0ABC     "))
				copy(frame[35:40], []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}) // YSF sync
				return frame
			}(),
			expectedSrc:  "VK3A",
			expectedDest: "G0ABC",
		},
		{
			name: "empty callsigns",
			input: func() []byte {
				frame := make([]byte, 155)
				copy(frame[:4], []byte{'Y', 'S', 'F', 'D'})
				// Leave callsign fields as zeros/spaces
				copy(frame[35:40], []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}) // YSF sync
				return frame
			}(),
			expectedSrc:  "",
			expectedDest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &Frame{}
			err := frame.Parse(tt.input)
			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if frame.SourceCallsign != tt.expectedSrc {
				t.Errorf("SourceCallsign = %q, want %q", frame.SourceCallsign, tt.expectedSrc)
			}
			if frame.DestCallsign != tt.expectedDest {
				t.Errorf("DestCallsign = %q, want %q", frame.DestCallsign, tt.expectedDest)
			}
		})
	}
}

func TestYSFFrame_BuildFrame(t *testing.T) {
	tests := []struct {
		name         string
		sourceCall   string
		destCall     string
		fich         FICH
		payload      []byte
		expectedSize int
	}{
		{
			name:       "header frame",
			sourceCall: "G4KLX",
			destCall:   "VK3DRS",
			fich: FICH{
				FI: 0, // Header
				DT: 0, // Voice/Data mode 1
				CM: 0, // Group call
			},
			payload:      make([]byte, 90), // YSF payload after FICH
			expectedSize: 155,
		},
		{
			name:       "communications frame",
			sourceCall: "VK3A",
			destCall:   "G0ABC",
			fich: FICH{
				FI: 1, // Communications
				DT: 0,
				CM: 0,
				FN: 3, // Frame number 3
			},
			payload:      make([]byte, 90),
			expectedSize: 155,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := &Frame{
				SourceCallsign: tt.sourceCall,
				DestCallsign:   tt.destCall,
				FICH:           tt.fich,
				Payload:        tt.payload,
			}

			data := frame.Build()

			if len(data) != tt.expectedSize {
				t.Errorf("Build() frame size = %d, want %d", len(data), tt.expectedSize)
			}

			// Verify magic number
			if string(data[:4]) != "YSFD" {
				t.Errorf("Build() magic = %q, want %q", string(data[:4]), "YSFD")
			}

			// Verify sync pattern at offset 35
			expectedSync := []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}
			if !bytesEqual(data[35:40], expectedSync) {
				t.Errorf("Build() sync pattern incorrect")
			}

			// Parse the built frame to verify round-trip
			parsedFrame := &Frame{}
			err := parsedFrame.Parse(data)
			if err != nil {
				t.Errorf("Build() produced unparseable frame: %v", err)
				return
			}

			if parsedFrame.SourceCallsign != tt.sourceCall {
				t.Errorf("Round-trip source callsign = %q, want %q", parsedFrame.SourceCallsign, tt.sourceCall)
			}
			if parsedFrame.FICH.FI != tt.fich.FI {
				t.Errorf("Round-trip FICH.FI = %d, want %d", parsedFrame.FICH.FI, tt.fich.FI)
			}
		})
	}
}

func TestFICH_Encode(t *testing.T) {
	tests := []struct {
		name string
		fich FICH
	}{
		{
			name: "header frame",
			fich: FICH{
				FI: 0, DT: 0, CM: 0, CS: 0, FN: 0, FT: 0, MR: 0,
			},
		},
		{
			name: "communications frame",
			fich: FICH{
				FI: 1, DT: 1, CM: 1, CS: 1, FN: 5, FT: 1, MR: 1,
			},
		},
		{
			name: "terminator frame",
			fich: FICH{
				FI: 2, DT: 2, CM: 3, CS: 3, FN: 7, FT: 1, MR: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := tt.fich.Encode()

			if len(encoded) != 25 {
				t.Errorf("FICH.Encode() length = %d, want 25", len(encoded))
			}

			// Decode and verify round-trip
			decoded := &FICH{}
			err := decoded.Decode(encoded)
			if err != nil {
				t.Errorf("FICH.Decode() error: %v", err)
				return
			}

			if decoded.FI != tt.fich.FI {
				t.Errorf("Round-trip FI = %d, want %d", decoded.FI, tt.fich.FI)
			}
			if decoded.DT != tt.fich.DT {
				t.Errorf("Round-trip DT = %d, want %d", decoded.DT, tt.fich.DT)
			}
			if decoded.CM != tt.fich.CM {
				t.Errorf("Round-trip CM = %d, want %d", decoded.CM, tt.fich.CM)
			}
		})
	}
}

func TestFICH_Decode(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectError bool
		expectFICH  FICH
	}{
		{
			name:        "all zeros",
			input:       make([]byte, 25),
			expectError: false,
			expectFICH: FICH{
				FI: 0, DT: 0, CM: 0, CS: 0, FN: 0, FT: 0, MR: 0,
			},
		},
		{
			name:        "too short",
			input:       make([]byte, 10),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fich := &FICH{}
			err := fich.Decode(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("FICH.Decode() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FICH.Decode() unexpected error: %v", err)
				return
			}

			if fich.FI != tt.expectFICH.FI {
				t.Errorf("FICH.FI = %d, want %d", fich.FI, tt.expectFICH.FI)
			}
		})
	}
}

// bytesEqual function is now in frame.go

// Benchmark tests
func BenchmarkYSFFrame_Parse(b *testing.B) {
	frame := make([]byte, 155)
	copy(frame[:4], []byte{'Y', 'S', 'F', 'D'})
	copy(frame[4:14], []byte("G4KLX     "))
	copy(frame[35:40], []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := &Frame{}
		f.Parse(frame)
	}
}

func BenchmarkYSFFrame_Build(b *testing.B) {
	frame := &Frame{
		SourceCallsign: "G4KLX",
		DestCallsign:   "VK3DRS",
		FICH: FICH{
			FI: 1, DT: 0, CM: 0, FN: 3,
		},
		Payload: make([]byte, 90),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame.Build()
	}
}