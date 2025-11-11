package wiresx

import (
	"testing"
)

func TestWiresX_ProcessDXRequest(t *testing.T) {
	tests := []struct {
		name           string
		command        []byte
		expectedStatus Status
		expectedReply  bool
	}{
		{
			name:           "valid DX request",
			command:        []byte{0x01, 0x5D, 0x71, 0x5F, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x4A}, // DX_REQ with proper framing and length
			expectedStatus: StatusDX,
			expectedReply:  true,
		},
		{
			name:           "invalid command",
			command:        []byte{0x01, 0x5D, 0xFF, 0x5F, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00},
			expectedStatus: StatusFail,
			expectedReply:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wx := NewWiresX("G4KLX", "", nil, "", false)
			wx.SetInfo("Test Node", 145800000, 145200000, 9)

			status := wx.Process(tt.command, []byte("G4KLX     "), 1, 1, 1, 1)

			if status != tt.expectedStatus {
				t.Errorf("Process() status = %v, want %v", status, tt.expectedStatus)
			}
		})
	}
}

func TestWiresX_ProcessConnectRequest(t *testing.T) {
	tests := []struct {
		name           string
		command        []byte
		expectedStatus Status
		expectedDstID  uint32
	}{
		{
			name:           "valid connect to TG 9",
			command:        []byte{0x01, 0x5D, 0x23, 0x5F, '0', '0', '0', '0', '0', '9', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x7D}, // CONN_REQ to TG 9
			expectedStatus: StatusConnect,
			expectedDstID:  9,
		},
		{
			name:           "valid connect to TG 91",
			command:        []byte{0x01, 0x5D, 0x23, 0x5F, '0', '0', '0', '0', '9', '1', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x18},
			expectedStatus: StatusConnect,
			expectedDstID:  91,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wx := NewWiresX("G4KLX", "", nil, "", false)
			wx.SetInfo("Test Node", 145800000, 145200000, 0)

			status := wx.Process(tt.command, []byte("G4KLX     "), 1, 1, 1, 1)

			if status != tt.expectedStatus {
				t.Errorf("Process() status = %v, want %v", status, tt.expectedStatus)
			}

			if wx.GetDstID() != tt.expectedDstID {
				t.Errorf("GetDstID() = %d, want %d", wx.GetDstID(), tt.expectedDstID)
			}
		})
	}
}

func TestWiresX_ProcessDisconnectRequest(t *testing.T) {
	wx := NewWiresX("G4KLX", "", nil, "", false)
	wx.SetInfo("Test Node", 145800000, 145200000, 91)

	command := []byte{0x01, 0x5D, 0x2A, 0x5F, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x2B} // DISC_REQ
	status := wx.Process(command, []byte("G4KLX     "), 1, 1, 1, 1)

	if status != StatusDisconnect {
		t.Errorf("Process() status = %v, want %v", status, StatusDisconnect)
	}
}

func TestWiresX_ProcessAllRequest(t *testing.T) {
	tests := []struct {
		name           string
		command        []byte
		expectedStatus Status
	}{
		{
			name:           "ALL request for page 0",
			command:        []byte{0x01, 0x5D, 0x66, 0x5F, '0', '1', '0', '0', '0', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x2F},
			expectedStatus: StatusAll,
		},
		{
			name:           "SEARCH request",
			command:        []byte{0x01, 0x5D, 0x66, 0x5F, '1', '1', '0', '0', '0', 'T', 'E', 'S', 'T', ' ', 'S', 'E', 'A', 'R', 0x03, 0x5E}, // Truncated search term
			expectedStatus: StatusAll, // Search is handled as ALL with different parameters
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wx := NewWiresX("G4KLX", "", nil, "", false)
			wx.SetInfo("Test Node", 145800000, 145200000, 0)

			status := wx.Process(tt.command, []byte("G4KLX     "), 1, 1, 1, 1)

			if status != tt.expectedStatus {
				t.Errorf("Process() status = %v, want %v", status, tt.expectedStatus)
			}
		})
	}
}

func TestTalkGroupRegistry_LoadFromFile(t *testing.T) {
	// Test loading talk group registry from file
	testData := `# Test TG file
9;0;LOCAL;Local talk group
91;0;WORLDWIDE;Worldwide reflector
4000;0;UNLINK;Unlink command
9990;0;PARROT;Parrot mode
123456;0;TEST TG;Test talk group with long ID`

	registry := NewTalkGroupRegistry(false)
	err := registry.LoadFromString(testData)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	tests := []struct {
		id       uint32
		expected *TalkGroup
	}{
		{
			id: 9,
			expected: &TalkGroup{
				ID:   "0000009",
				Opt:  "0",
				Name: "LOCAL           ",
				Desc: "Local talk gro",
			},
		},
		{
			id: 91,
			expected: &TalkGroup{
				ID:   "0000091",
				Opt:  "0",
				Name: "WORLDWIDE       ",
				Desc: "Worldwide refl",
			},
		},
	}

	for _, tt := range tests {
		t.Run("find TG "+tt.expected.ID, func(t *testing.T) {
			tg := registry.FindByID(tt.id)
			if tg == nil {
				t.Fatalf("FindByID(%d) returned nil", tt.id)
			}

			if tg.ID != tt.expected.ID {
				t.Errorf("TalkGroup.ID = %q, want %q", tg.ID, tt.expected.ID)
			}
			if tg.Name != tt.expected.Name {
				t.Errorf("TalkGroup.Name = %q, want %q", tg.Name, tt.expected.Name)
			}
		})
	}
}

func TestTalkGroupRegistry_Search(t *testing.T) {
	testData := `9;0;LOCAL;Local talk group
91;0;WORLDWIDE;Worldwide reflector
4000;0;UNLINK;Unlink command
50;0;TEST GROUP;Test group
51;0;TEST ANOTHER;Another test group`

	registry := NewTalkGroupRegistry(false)
	err := registry.LoadFromString(testData)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	tests := []struct {
		name        string
		searchTerm  string
		expectedLen int
	}{
		{
			name:        "search for TEST",
			searchTerm:  "TEST",
			expectedLen: 2,
		},
		{
			name:        "search for LOCAL",
			searchTerm:  "LOCAL",
			expectedLen: 1,
		},
		{
			name:        "search for nonexistent",
			searchTerm:  "NONEXISTENT",
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.Search(tt.searchTerm)
			if len(results) != tt.expectedLen {
				t.Errorf("Search(%q) returned %d results, want %d", tt.searchTerm, len(results), tt.expectedLen)
			}
		})
	}
}

func TestWiresX_ResponseGeneration(t *testing.T) {
	wx := NewWiresX("G4KLX", "RPT", nil, "", false)
	wx.SetInfo("Test Repeater", 145800000, 145200000, 91)

	// Test DX response generation
	t.Run("DX response", func(t *testing.T) {
		response := wx.createDXResponse()
		if len(response) == 0 {
			t.Error("createDXResponse() returned empty response")
		}

		// Verify DX response contains expected elements
		if len(response) < 129 {
			t.Errorf("DX response length = %d, want >= 129", len(response))
		}

		// Check for end marker and CRC
		if response[127] != 0x03 {
			t.Errorf("DX response end marker = 0x%02X, want 0x03", response[127])
		}
	})

	// Test Connect response generation
	t.Run("Connect response", func(t *testing.T) {
		response := wx.createConnectResponse(91)
		if len(response) == 0 {
			t.Error("createConnectResponse() returned empty response")
		}

		if len(response) < 91 {
			t.Errorf("Connect response length = %d, want >= 91", len(response))
		}
	})
}

func TestWiresX_RepeaterID(t *testing.T) {
	wx := NewWiresX("G4KLX", "", nil, "", false)
	wx.SetInfo("Test Node", 145800000, 145200000, 0)

	id := wx.GetRepeaterID()
	if len(id) != 5 {
		t.Errorf("GetRepeaterID() length = %d, want 5", len(id))
	}

	// ID should be numeric
	for _, char := range id {
		if char < '0' || char > '9' {
			t.Errorf("GetRepeaterID() contains non-numeric character: %c", char)
		}
	}
}

func TestWiresX_Timer(t *testing.T) {
	wx := NewWiresX("G4KLX", "", nil, "", false)
	wx.SetInfo("Test Node", 145800000, 145200000, 0)

	// Simulate DX request
	command := []byte{0x01, 0x5D, 0x71, 0x5F, 0x00, 0x03, 0x00}
	status := wx.Process(command, []byte("G4KLX     "), 1, 1, 1, 1)

	if status != StatusDX {
		t.Fatalf("Process() status = %v, want %v", status, StatusDX)
	}

	// Clock should trigger response after timer expires
	wx.Clock(1100) // Timer is set to 1000ms

	// Response should have been generated and buffered
	// This would require checking the output buffer/network write
}

// Benchmark tests for performance
func BenchmarkWiresX_ProcessDX(b *testing.B) {
	wx := NewWiresX("G4KLX", "", nil, "", false)
	wx.SetInfo("Test Node", 145800000, 145200000, 0)
	command := []byte{0x01, 0x5D, 0x71, 0x5F, 0x00, 0x03, 0x00}
	source := []byte("G4KLX     ")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wx.Process(command, source, 1, 1, 1, 1)
	}
}

func BenchmarkTalkGroupRegistry_Search(b *testing.B) {
	testData := `9;0;LOCAL;Local talk group
91;0;WORLDWIDE;Worldwide reflector
4000;0;UNLINK;Unlink command`

	registry := NewTalkGroupRegistry(false)
	registry.LoadFromString(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Search("LOCAL")
	}
}