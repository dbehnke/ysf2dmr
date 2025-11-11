package network

import (
	"net"
	"testing"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

func TestNewYSFNetworkClient(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		port        int
		callsign    string
		debug       bool
		expectError bool
	}{
		{
			name:        "valid IP address",
			address:     "127.0.0.1",
			port:        14580,
			callsign:    "TEST",
			debug:       false,
			expectError: false,
		},
		{
			name:        "localhost hostname",
			address:     "localhost",
			port:        14580,
			callsign:    "LONGCALL",
			debug:       true,
			expectError: false,
		},
		{
			name:        "invalid address",
			address:     "invalid.invalid.invalid",
			port:        14580,
			callsign:    "TEST",
			debug:       false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := NewYSFNetworkClient(tt.address, tt.port, tt.callsign, tt.debug)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if network == nil {
				t.Fatalf("Expected non-nil network")
			}

			// Verify callsign padding
			expectedCallsign := padCallsign(tt.callsign)
			if network.callsign != expectedCallsign {
				t.Errorf("Callsign = %q, want %q", network.callsign, expectedCallsign)
			}

			// Verify port and debug settings
			if network.port != tt.port {
				t.Errorf("Port = %d, want %d", network.port, tt.port)
			}

			if network.debug != tt.debug {
				t.Errorf("Debug = %v, want %v", network.debug, tt.debug)
			}
		})
	}
}

func TestNewYSFNetworkServer(t *testing.T) {
	network := NewYSFNetworkServer(14580, "SERVER", true)

	if network == nil {
		t.Fatalf("Expected non-nil network")
	}

	expectedCallsign := "SERVER    " // Padded to 10 bytes
	if network.callsign != expectedCallsign {
		t.Errorf("Callsign = %q, want %q", network.callsign, expectedCallsign)
	}

	if network.port != 0 { // Server mode starts with no destination
		t.Errorf("Port = %d, want 0", network.port)
	}

	if !network.debug {
		t.Errorf("Debug = false, want true")
	}
}

func TestCallsignPadding(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short callsign",
			input:    "TEST",
			expected: "TEST      ",
		},
		{
			name:     "exact length",
			input:    "1234567890",
			expected: "1234567890",
		},
		{
			name:     "too long",
			input:    "VERYLONGCALLSIGN",
			expected: "VERYLONGCA", // Truncated to 10 chars
		},
		{
			name:     "empty",
			input:    "",
			expected: "          ",
		},
		{
			name:     "single char",
			input:    "A",
			expected: "A         ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padCallsign(tt.input)
			if result != tt.expected {
				t.Errorf("padCallsign(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			if len(result) != protocol.YSF_CALLSIGN_LENGTH {
				t.Errorf("padCallsign(%q) length = %d, want %d", tt.input, len(result), protocol.YSF_CALLSIGN_LENGTH)
			}
		})
	}
}

func TestGetCallsign(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	callsign := network.GetCallsign()
	expected := "TEST"

	if callsign != expected {
		t.Errorf("GetCallsign() = %q, want %q", callsign, expected)
	}
}

func TestMessageInitialization(t *testing.T) {
	network := NewYSFNetworkServer(14580, "MYCALL", false)

	// Test poll message
	expectedPoll := make([]byte, protocol.YSF_POLL_MESSAGE_LENGTH)
	copy(expectedPoll[0:], "YSFP")
	copy(expectedPoll[4:], "MYCALL    ") // Padded callsign

	if len(network.pollMsg) != len(expectedPoll) {
		t.Errorf("Poll message length = %d, want %d", len(network.pollMsg), len(expectedPoll))
	}

	for i, b := range expectedPoll {
		if network.pollMsg[i] != b {
			t.Errorf("Poll message byte %d = 0x%02X, want 0x%02X", i, network.pollMsg[i], b)
		}
	}

	// Test unlink message
	expectedUnlink := make([]byte, protocol.YSF_UNLINK_MESSAGE_LENGTH)
	copy(expectedUnlink[0:], "YSFU")
	copy(expectedUnlink[4:], "MYCALL    ") // Padded callsign

	if len(network.unlinkMsg) != len(expectedUnlink) {
		t.Errorf("Unlink message length = %d, want %d", len(network.unlinkMsg), len(expectedUnlink))
	}

	for i, b := range expectedUnlink {
		if network.unlinkMsg[i] != b {
			t.Errorf("Unlink message byte %d = 0x%02X, want 0x%02X", i, network.unlinkMsg[i], b)
		}
	}
}

func TestSetDestination(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	// Initially no destination
	if network.port != 0 {
		t.Errorf("Initial port = %d, want 0", network.port)
	}

	// Set destination
	ip := net.ParseIP("192.168.1.100")
	port := 42000

	network.SetDestination(ip, port)

	if !network.address.Equal(ip) {
		t.Errorf("Address = %s, want %s", network.address.String(), ip.String())
	}

	if network.port != port {
		t.Errorf("Port = %d, want %d", network.port, port)
	}

	// Clear destination
	network.ClearDestination()

	if network.address != nil {
		t.Errorf("Address = %s, want nil", network.address.String())
	}

	if network.port != 0 {
		t.Errorf("Port = %d, want 0", network.port)
	}
}

func TestSetDestinationByString(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	err := network.SetDestinationByString("127.0.0.1", 42000)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedIP := net.ParseIP("127.0.0.1")
	if !network.address.Equal(expectedIP) {
		t.Errorf("Address = %s, want %s", network.address.String(), expectedIP.String())
	}

	if network.port != 42000 {
		t.Errorf("Port = %d, want 42000", network.port)
	}
}

func TestWriteValidation(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	// Test with no destination - should return nil without error
	data := make([]byte, protocol.YSF_FRAME_LENGTH)
	err := network.Write(data)
	if err != nil {
		t.Errorf("Write with no destination should not error, got: %v", err)
	}

	// Set destination
	network.SetDestinationByString("127.0.0.1", 42000)

	// Test with wrong frame length
	wrongData := make([]byte, 100) // Wrong size
	err = network.Write(wrongData)
	if err == nil {
		t.Errorf("Write with wrong frame length should error")
	}

	// Note: We can't actually test successful write without opening the socket
	// and having a real network endpoint, but the validation logic is tested
}

func TestPollAndUnlinkNoDestination(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	// Test poll with no destination - should return nil without error
	err := network.WritePoll()
	if err != nil {
		t.Errorf("WritePoll with no destination should not error, got: %v", err)
	}

	// Test unlink with no destination - should return nil without error
	err = network.WriteUnlink()
	if err != nil {
		t.Errorf("WriteUnlink with no destination should not error, got: %v", err)
	}
}

func TestReadEmptyBuffer(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	data := make([]byte, 200)
	length := network.Read(data)

	if length != 0 {
		t.Errorf("Read from empty buffer should return 0, got %d", length)
	}

	if network.HasData() {
		t.Errorf("HasData() should return false for empty buffer")
	}
}

func TestStringRepresentation(t *testing.T) {
	// Test server mode
	server := NewYSFNetworkServer(14580, "SERVER", false)
	serverStr := server.String()
	expectedServer := "YSFNetwork[SERVER]: server mode"
	if serverStr != expectedServer {
		t.Errorf("Server String() = %q, want %q", serverStr, expectedServer)
	}

	// Test client mode
	client, err := NewYSFNetworkClient("127.0.0.1", 42000, "CLIENT", false)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	clientStr := client.String()
	expectedClient := "YSFNetwork[CLIENT]: client mode -> 127.0.0.1:42000"
	if clientStr != expectedClient {
		t.Errorf("Client String() = %q, want %q", clientStr, expectedClient)
	}
}

// Integration test to verify ring buffer interaction
func TestRingBufferIntegration(t *testing.T) {
	network := NewYSFNetworkServer(14580, "TEST", false)

	// Manually add some test data to the ring buffer
	testData := []byte("Hello, YSF!")
	if !network.buffer.AddLength(testData) {
		t.Fatalf("Failed to add test data to ring buffer")
	}

	// Verify HasData returns true
	if !network.HasData() {
		t.Errorf("HasData() should return true after adding data")
	}

	// Read the data back
	readBuffer := make([]byte, 100)
	length := network.Read(readBuffer)

	if length != len(testData) {
		t.Errorf("Read length = %d, want %d", length, len(testData))
	}

	// Verify the data matches
	for i := 0; i < length; i++ {
		if readBuffer[i] != testData[i] {
			t.Errorf("Read data[%d] = 0x%02X, want 0x%02X", i, readBuffer[i], testData[i])
		}
	}

	// Verify buffer is now empty
	if network.HasData() {
		t.Errorf("HasData() should return false after reading all data")
	}
}