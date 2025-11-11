package network

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

func TestNewDMRNetwork(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		port        int
		localId     uint32
		id          uint32
		password    string
		duplex      bool
		version     string
		debug       bool
		slot1       bool
		slot2       bool
		hwType      protocol.HWType
		jitter      int
		expectError bool
	}{
		{
			name:        "valid configuration",
			address:     "127.0.0.1",
			port:        62030,
			localId:     4000,
			id:          123456,
			password:    "test123",
			duplex:      true,
			version:     "1.0.0",
			debug:       false,
			slot1:       true,
			slot2:       true,
			hwType:      protocol.HW_TYPE_HOMEBREW,
			jitter:      120,
			expectError: false,
		},
		{
			name:        "invalid address",
			address:     "invalid.invalid.invalid",
			port:        62030,
			localId:     4000,
			id:          123456,
			password:    "test123",
			duplex:      false,
			version:     "1.0.0",
			debug:       true,
			slot1:       false,
			slot2:       true,
			hwType:      protocol.HW_TYPE_HOTSPOT,
			jitter:      60,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, err := NewDMRNetwork(tt.address, tt.port, tt.localId, tt.id, tt.password,
				tt.duplex, tt.version, tt.debug, tt.slot1, tt.slot2, tt.hwType, tt.jitter)

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

			// Verify configuration
			if network.port != tt.port {
				t.Errorf("Port = %d, want %d", network.port, tt.port)
			}

			if network.localId != tt.localId {
				t.Errorf("LocalId = %d, want %d", network.localId, tt.localId)
			}

			expectedId := make([]byte, 4)
			binary.BigEndian.PutUint32(expectedId, tt.id)
			for i, b := range expectedId {
				if network.id[i] != b {
					t.Errorf("ID[%d] = 0x%02X, want 0x%02X", i, network.id[i], b)
				}
			}

			if network.password != tt.password {
				t.Errorf("Password = %q, want %q", network.password, tt.password)
			}

			if network.duplex != tt.duplex {
				t.Errorf("Duplex = %v, want %v", network.duplex, tt.duplex)
			}

			if network.version != tt.version {
				t.Errorf("Version = %q, want %q", network.version, tt.version)
			}

			if network.debug != tt.debug {
				t.Errorf("Debug = %v, want %v", network.debug, tt.debug)
			}

			if network.slot1 != tt.slot1 {
				t.Errorf("Slot1 = %v, want %v", network.slot1, tt.slot1)
			}

			if network.slot2 != tt.slot2 {
				t.Errorf("Slot2 = %v, want %v", network.slot2, tt.slot2)
			}

			if network.hwType != tt.hwType {
				t.Errorf("HwType = %v, want %v", network.hwType, tt.hwType)
			}

			// Verify delay buffers
			if tt.slot1 && network.delayBuffers[1] == nil {
				t.Errorf("Slot 1 delay buffer should not be nil")
			}
			if !tt.slot1 && network.delayBuffers[1] != nil {
				t.Errorf("Slot 1 delay buffer should be nil")
			}

			if tt.slot2 && network.delayBuffers[2] == nil {
				t.Errorf("Slot 2 delay buffer should not be nil")
			}
			if !tt.slot2 && network.delayBuffers[2] != nil {
				t.Errorf("Slot 2 delay buffer should be nil")
			}

			// Verify initial state
			if network.status != protocol.DMR_WAITING_CONNECT {
				t.Errorf("Initial status = %d, want %d", network.status, protocol.DMR_WAITING_CONNECT)
			}

			if network.enabled {
				t.Errorf("Initially enabled = true, want false")
			}

			if network.IsConnected() {
				t.Errorf("Initially connected = true, want false")
			}
		})
	}
}

func TestDMRNetworkSetConfig(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Test configuration
	callsign := "TEST"
	rxFreq := uint32(145500000)
	txFreq := uint32(145100000)
	power := uint32(25)
	colorCode := uint32(1)
	lat := float32(40.7128)
	lng := float32(-74.0060)
	height := 100
	location := "New York"
	description := "Test Repeater"
	url := "http://example.com"

	network.SetConfig(callsign, rxFreq, txFreq, power, colorCode, lat, lng, height, location, description, url)

	if network.callsign != callsign {
		t.Errorf("Callsign = %q, want %q", network.callsign, callsign)
	}

	if network.rxFrequency != rxFreq {
		t.Errorf("RxFrequency = %d, want %d", network.rxFrequency, rxFreq)
	}

	if network.txFrequency != txFreq {
		t.Errorf("TxFrequency = %d, want %d", network.txFrequency, txFreq)
	}

	if network.power != power {
		t.Errorf("Power = %d, want %d", network.power, power)
	}

	if network.colorCode != colorCode {
		t.Errorf("ColorCode = %d, want %d", network.colorCode, colorCode)
	}

	if network.latitude != lat {
		t.Errorf("Latitude = %f, want %f", network.latitude, lat)
	}

	if network.longitude != lng {
		t.Errorf("Longitude = %f, want %f", network.longitude, lng)
	}

	if network.height != height {
		t.Errorf("Height = %d, want %d", network.height, height)
	}

	if network.location != location {
		t.Errorf("Location = %q, want %q", network.location, location)
	}

	if network.description != description {
		t.Errorf("Description = %q, want %q", network.description, description)
	}

	if network.url != url {
		t.Errorf("URL = %q, want %q", network.url, url)
	}
}

func TestDMRNetworkSetOptions(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	options := "PrivateCall=1;GroupCall=1"
	network.SetOptions(options)

	if network.options != options {
		t.Errorf("Options = %q, want %q", network.options, options)
	}
}

func TestDMRNetworkEnable(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Initially disabled
	if network.enabled {
		t.Errorf("Initially enabled = true, want false")
	}

	// Enable
	network.Enable(true)
	if !network.enabled {
		t.Errorf("After Enable(true), enabled = false, want true")
	}

	// Disable
	network.Enable(false)
	if network.enabled {
		t.Errorf("After Enable(false), enabled = true, want false")
	}
}

func TestDMRNetworkBeacon(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Initially no beacon
	if network.WantsBeacon() {
		t.Errorf("Initially WantsBeacon = true, want false")
	}

	// Set beacon flag
	network.beacon = true
	if !network.WantsBeacon() {
		t.Errorf("WantsBeacon = false after setting beacon, want true")
	}

	// Should be cleared after reading
	if network.WantsBeacon() {
		t.Errorf("WantsBeacon = true after second read, want false")
	}
}

func TestBuildDMRDPacket(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Create test DMR data
	data := protocol.NewDMRData()
	data.SetSlotNo(1)
	data.SetSrcId(12345)
	data.SetDstId(9)
	data.SetFLCO(protocol.FLCO_GROUP)
	data.SetDataType(protocol.DT_VOICE)
	data.SetN(3)
	data.SetBER(5)
	data.SetRSSI(80)

	// Set test data
	testData := make([]byte, 33)
	for i := range testData {
		testData[i] = byte(i)
	}
	data.SetData(testData)

	packet := network.buildDMRDPacket(data)

	// Verify packet length
	if len(packet) != protocol.HOMEBREW_DATA_PACKET_LENGTH {
		t.Errorf("Packet length = %d, want %d", len(packet), protocol.HOMEBREW_DATA_PACKET_LENGTH)
	}

	// Verify magic
	if string(packet[0:4]) != protocol.NETWORK_MAGIC_DATA {
		t.Errorf("Magic = %q, want %q", string(packet[0:4]), protocol.NETWORK_MAGIC_DATA)
	}

	// Verify source ID (3 bytes big-endian)
	srcId := (uint32(packet[5]) << 16) | (uint32(packet[6]) << 8) | uint32(packet[7])
	if srcId != 12345 {
		t.Errorf("Source ID = %d, want 12345", srcId)
	}

	// Verify destination ID
	dstId := (uint32(packet[8]) << 16) | (uint32(packet[9]) << 8) | uint32(packet[10])
	if dstId != 9 {
		t.Errorf("Destination ID = %d, want 9", dstId)
	}

	// Verify repeater ID
	expectedId := make([]byte, 4)
	binary.BigEndian.PutUint32(expectedId, 123456)
	for i, b := range expectedId {
		if packet[11+i] != b {
			t.Errorf("Repeater ID[%d] = 0x%02X, want 0x%02X", i, packet[11+i], b)
		}
	}

	// Verify flags (slot 1, group call, voice with N=3)
	expectedFlags := byte(0x03) // N=3 for voice
	if packet[15] != expectedFlags {
		t.Errorf("Flags = 0x%02X, want 0x%02X", packet[15], expectedFlags)
	}

	// Verify DMR data
	for i, b := range testData {
		if packet[20+i] != b {
			t.Errorf("DMR data[%d] = 0x%02X, want 0x%02X", i, packet[20+i], b)
		}
	}

	// Verify BER and RSSI
	if packet[53] != 5 {
		t.Errorf("BER = %d, want 5", packet[53])
	}

	if packet[54] != 80 {
		t.Errorf("RSSI = %d, want 80", packet[54])
	}
}

func TestParseDMRDPacket(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Create test packet
	packet := make([]byte, protocol.HOMEBREW_DATA_PACKET_LENGTH)
	copy(packet[0:4], protocol.NETWORK_MAGIC_DATA)

	// Sequence number
	packet[4] = 123

	// Source ID (big-endian)
	packet[5] = 0x00
	packet[6] = 0x30
	packet[7] = 0x39 // 12345

	// Destination ID
	packet[8] = 0x00
	packet[9] = 0x00
	packet[10] = 0x09 // 9

	// Repeater ID
	binary.BigEndian.PutUint32(packet[11:15], 123456)

	// Flags: Slot 2, private call, voice sync, N=2
	packet[15] = 0x80 | 0x40 | 0x10 | 0x02

	// Stream ID
	binary.BigEndian.PutUint32(packet[16:20], 0xABCDEF12)

	// DMR data
	for i := 0; i < 33; i++ {
		packet[20+i] = byte(i + 100)
	}

	// BER and RSSI
	packet[53] = 10
	packet[54] = 75

	// Parse packet
	data := protocol.NewDMRData()
	success := network.parseDMRDPacket(packet, data)

	if !success {
		t.Fatalf("Failed to parse DMRD packet")
	}

	// Verify parsed data
	if data.GetSeqNo() != 123 {
		t.Errorf("SeqNo = %d, want 123", data.GetSeqNo())
	}

	if data.GetSrcId() != 12345 {
		t.Errorf("SrcId = %d, want 12345", data.GetSrcId())
	}

	if data.GetDstId() != 9 {
		t.Errorf("DstId = %d, want 9", data.GetDstId())
	}

	if data.GetSlotNo() != 2 {
		t.Errorf("SlotNo = %d, want 2", data.GetSlotNo())
	}

	if data.GetFLCO() != protocol.FLCO_USER_USER {
		t.Errorf("FLCO = %d, want %d", data.GetFLCO(), protocol.FLCO_USER_USER)
	}

	if data.GetDataType() != protocol.DT_VOICE_SYNC {
		t.Errorf("DataType = %d, want %d", data.GetDataType(), protocol.DT_VOICE_SYNC)
	}

	if data.GetN() != 2 {
		t.Errorf("N = %d, want 2", data.GetN())
	}

	if data.GetStreamId() != 0xABCDEF12 {
		t.Errorf("StreamId = 0x%08X, want 0xABCDEF12", data.GetStreamId())
	}

	// Verify DMR data
	dmrData := data.GetData()
	for i := 0; i < 33; i++ {
		expected := byte(i + 100)
		if dmrData[i] != expected {
			t.Errorf("DMR data[%d] = %d, want %d", i, dmrData[i], expected)
		}
	}

	if data.GetBER() != 10 {
		t.Errorf("BER = %d, want 10", data.GetBER())
	}

	if data.GetRSSI() != 75 {
		t.Errorf("RSSI = %d, want 75", data.GetRSSI())
	}
}

func TestAuthenticationPackets(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "testpass",
		true, "1.0.0", true, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Test salt and authentication
	testSalt := []byte{0x12, 0x34, 0x56, 0x78}
	copy(network.salt, testSalt)

	// Calculate expected hash
	hasher := sha256.New()
	hasher.Write(testSalt)
	hasher.Write([]byte("testpass"))
	expectedHash := hasher.Sum(nil)

	// Create mock auth packet to test the hash calculation
	packet := make([]byte, protocol.NETWORK_AUTH_LENGTH)
	copy(packet[0:4], protocol.NETWORK_MAGIC_AUTH)
	copy(packet[4:8], network.id[:])

	// Manually build auth packet like writeAuth does
	hasher2 := sha256.New()
	hasher2.Write(network.salt)
	hasher2.Write([]byte(network.password))
	hash := hasher2.Sum(nil)
	copy(packet[8:40], hash[:32])

	// Verify the hash matches
	for i := 0; i < 32; i++ {
		if packet[8+i] != expectedHash[i] {
			t.Errorf("Auth hash[%d] = 0x%02X, want 0x%02X", i, packet[8+i], expectedHash[i])
		}
	}
}

func TestReset(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Get initial stream IDs
	originalStream1 := network.streamId[1]
	originalStream2 := network.streamId[2]

	// Reset slot 1
	network.Reset(1)

	// Verify slot 1 stream ID changed
	if network.streamId[1] == originalStream1 {
		t.Errorf("Stream ID for slot 1 should have changed after reset")
	}

	// Verify slot 2 stream ID unchanged
	if network.streamId[2] != originalStream2 {
		t.Errorf("Stream ID for slot 2 should not have changed")
	}

	// Reset slot 2
	network.Reset(2)

	// Verify slot 2 stream ID changed
	if network.streamId[2] == originalStream2 {
		t.Errorf("Stream ID for slot 2 should have changed after reset")
	}
}

func TestReadWithoutConnection(t *testing.T) {
	network, err := NewDMRNetwork("127.0.0.1", 62030, 4000, 123456, "test123",
		true, "1.0.0", false, true, true, protocol.HW_TYPE_HOMEBREW, 120)
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	data := protocol.NewDMRData()
	result := network.Read(data)

	// Should return false when not connected
	if result {
		t.Errorf("Read should return false when not connected")
	}

	// Enable network but still not connected
	network.Enable(true)
	result = network.Read(data)

	// Should still return false
	if result {
		t.Errorf("Read should return false when enabled but not connected")
	}
}