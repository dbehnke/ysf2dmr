package dmr

import (
	"fmt"
)

// DMR frame constants
const (
	DMR_FRAME_LENGTH = 33 // DMR frame length in bytes
	DMR_PAYLOAD_LENGTH = 23 // DMR payload length

	// FLCO (Forward Link Class Operation) values
	FLCO_GROUP_CALL    = 0x00
	FLCO_UNIT_TO_UNIT  = 0x03

	// Data types
	DATA_TYPE_VOICE_HEADER     = 0x01
	DATA_TYPE_VOICE_FRAME_A    = 0x02
	DATA_TYPE_VOICE_FRAME_B    = 0x03
	DATA_TYPE_VOICE_SYNC       = 0x04
	DATA_TYPE_VOICE_FRAME_C    = 0x05
	DATA_TYPE_VOICE_FRAME_D    = 0x06
	DATA_TYPE_VOICE_FRAME_E    = 0x07
	DATA_TYPE_VOICE_FRAME_F    = 0x08
	DATA_TYPE_VOICE_TERMINATOR = 0x09
	DATA_TYPE_DATA_HEADER      = 0x0A
	DATA_TYPE_DATA_FRAME       = 0x0B
	DATA_TYPE_DATA_TERMINATOR  = 0x0C

	// Color code range
	COLOR_CODE_MIN = 0
	COLOR_CODE_MAX = 15
)

// SyncType represents the type of DMR sync pattern
type SyncType int

const (
	SYNC_NONE SyncType = iota
	SYNC_VOICE
	SYNC_DATA
)

// DMR sync patterns
var (
	DMR_VOICE_SYNC = []byte{0x75, 0x5F, 0xD7, 0xDF, 0x75, 0xF7}
	DMR_DATA_SYNC  = []byte{0xDF, 0xF5, 0x7D, 0x75, 0xDF, 0x5D}
)

// Data represents a DMR data packet
type Data struct {
	SlotNumber    uint8  // Slot number (1 or 2)
	SourceID      uint32 // Source radio ID (24-bit)
	DestinationID uint32 // Destination ID (24-bit)
	FLCO          uint8  // Forward Link Class Operation
	DataType      uint8  // Data type (voice, data, etc.)
	SeqNumber     uint8  // Sequence number (0-5 for voice)
	Payload       []byte // Payload data (23 bytes)
	StreamID      uint32 // Stream ID for reassembly
	BER           uint8  // Bit Error Rate
	RSSI          uint8  // Received Signal Strength Indicator
}

// LinkControl represents DMR Link Control information
type LinkControl struct {
	FLCO          uint8  // Forward Link Class Operation
	SourceID      uint32 // Source radio ID (24-bit)
	DestinationID uint32 // Destination ID (24-bit)
	FID           uint8  // Feature ID
	Options       uint8  // Options byte
}

// EmbeddedData represents DMR embedded signaling data
type EmbeddedData struct {
	LCSS uint8    // Link Control Start Stop
	EMB  uint8    // Embedded signaling
	Data []byte   // Embedded data (8 bytes)
}

// SlotType represents DMR slot type information
type SlotType struct {
	DataType  uint8 // Data type
	ColorCode uint8 // Color code (0-15)
}

// Parse parses a DMR frame from raw bytes
func (d *Data) Parse(data []byte) error {
	if len(data) < DMR_FRAME_LENGTH {
		return fmt.Errorf("DMR frame too short: got %d bytes, need %d", len(data), DMR_FRAME_LENGTH)
	}

	// Byte 0: Slot number
	d.SlotNumber = data[0]
	if d.SlotNumber != 1 && d.SlotNumber != 2 {
		return fmt.Errorf("invalid DMR slot number: %d", d.SlotNumber)
	}

	// Bytes 1-3: Source ID (24-bit, big-endian)
	d.SourceID = (uint32(data[1]) << 16) | (uint32(data[2]) << 8) | uint32(data[3])

	// Bytes 4-6: Destination ID (24-bit, big-endian)
	d.DestinationID = (uint32(data[4]) << 16) | (uint32(data[5]) << 8) | uint32(data[6])

	// Byte 7: FLCO
	d.FLCO = data[7]

	// Byte 8: Data type
	d.DataType = data[8]

	// Byte 9: Sequence number
	d.SeqNumber = data[9]

	// Bytes 10-32: Payload (23 bytes)
	d.Payload = make([]byte, DMR_PAYLOAD_LENGTH)
	copy(d.Payload, data[10:DMR_FRAME_LENGTH])

	return nil
}

// Build constructs a DMR frame from the structure
func (d *Data) Build() []byte {
	frame := make([]byte, DMR_FRAME_LENGTH)

	// Byte 0: Slot number
	frame[0] = d.SlotNumber

	// Bytes 1-3: Source ID (24-bit, big-endian)
	frame[1] = uint8((d.SourceID >> 16) & 0xFF)
	frame[2] = uint8((d.SourceID >> 8) & 0xFF)
	frame[3] = uint8(d.SourceID & 0xFF)

	// Bytes 4-6: Destination ID (24-bit, big-endian)
	frame[4] = uint8((d.DestinationID >> 16) & 0xFF)
	frame[5] = uint8((d.DestinationID >> 8) & 0xFF)
	frame[6] = uint8(d.DestinationID & 0xFF)

	// Byte 7: FLCO
	frame[7] = d.FLCO

	// Byte 8: Data type
	frame[8] = d.DataType

	// Byte 9: Sequence number
	frame[9] = d.SeqNumber

	// Bytes 10-32: Payload (23 bytes)
	if len(d.Payload) >= DMR_PAYLOAD_LENGTH {
		copy(frame[10:], d.Payload[0:DMR_PAYLOAD_LENGTH])
	} else {
		copy(frame[10:10+len(d.Payload)], d.Payload)
	}

	return frame
}

// IsVoice returns true if this frame contains voice data
func (d *Data) IsVoice() bool {
	return d.DataType >= DATA_TYPE_VOICE_HEADER && d.DataType <= DATA_TYPE_VOICE_TERMINATOR
}

// IsData returns true if this frame contains data
func (d *Data) IsData() bool {
	return d.DataType >= DATA_TYPE_DATA_HEADER && d.DataType <= DATA_TYPE_DATA_TERMINATOR
}

// IsHeader returns true if this is a header frame
func (d *Data) IsHeader() bool {
	return d.DataType == DATA_TYPE_VOICE_HEADER || d.DataType == DATA_TYPE_DATA_HEADER
}

// IsTerminator returns true if this is a terminator frame
func (d *Data) IsTerminator() bool {
	return d.DataType == DATA_TYPE_VOICE_TERMINATOR || d.DataType == DATA_TYPE_DATA_TERMINATOR
}

// IsSync returns true if this is a sync frame
func (d *Data) IsSync() bool {
	return d.DataType == DATA_TYPE_VOICE_SYNC
}

// IsGroupCall returns true if this is a group call
func (d *Data) IsGroupCall() bool {
	return d.FLCO == FLCO_GROUP_CALL
}

// IsPrivateCall returns true if this is a private call
func (d *Data) IsPrivateCall() bool {
	return d.FLCO == FLCO_UNIT_TO_UNIT
}

// GetFrameName returns a human-readable name for the frame type
func (d *Data) GetFrameName() string {
	switch d.DataType {
	case DATA_TYPE_VOICE_HEADER:
		return "Voice Header"
	case DATA_TYPE_VOICE_FRAME_A:
		return "Voice Frame A"
	case DATA_TYPE_VOICE_FRAME_B:
		return "Voice Frame B"
	case DATA_TYPE_VOICE_SYNC:
		return "Voice Sync"
	case DATA_TYPE_VOICE_FRAME_C:
		return "Voice Frame C"
	case DATA_TYPE_VOICE_FRAME_D:
		return "Voice Frame D"
	case DATA_TYPE_VOICE_FRAME_E:
		return "Voice Frame E"
	case DATA_TYPE_VOICE_FRAME_F:
		return "Voice Frame F"
	case DATA_TYPE_VOICE_TERMINATOR:
		return "Voice Terminator"
	case DATA_TYPE_DATA_HEADER:
		return "Data Header"
	case DATA_TYPE_DATA_FRAME:
		return "Data Frame"
	case DATA_TYPE_DATA_TERMINATOR:
		return "Data Terminator"
	default:
		return fmt.Sprintf("Unknown (0x%02X)", d.DataType)
	}
}

// String returns a human-readable representation of the DMR data
func (d *Data) String() string {
	callType := "Group"
	if d.IsPrivateCall() {
		callType = "Private"
	}

	return fmt.Sprintf("DMR{Slot=%d, %s, %s Call, Src=%d, Dst=%d, Seq=%d}",
		d.SlotNumber, d.GetFrameName(), callType, d.SourceID, d.DestinationID, d.SeqNumber)
}

// Encode encodes the Link Control information into 9 bytes
func (lc *LinkControl) Encode() []byte {
	data := make([]byte, 9)

	// Byte 0: FLCO and reserved bits
	data[0] = (lc.FLCO << 2) // FLCO in upper 6 bits

	// Bytes 1-3: Destination ID (24-bit, big-endian)
	data[1] = uint8((lc.DestinationID >> 16) & 0xFF)
	data[2] = uint8((lc.DestinationID >> 8) & 0xFF)
	data[3] = uint8(lc.DestinationID & 0xFF)

	// Bytes 4-6: Source ID (24-bit, big-endian)
	data[4] = uint8((lc.SourceID >> 16) & 0xFF)
	data[5] = uint8((lc.SourceID >> 8) & 0xFF)
	data[6] = uint8(lc.SourceID & 0xFF)

	// Byte 7: Feature ID
	data[7] = lc.FID

	// Byte 8: Options
	data[8] = lc.Options

	return data
}

// Decode decodes 9 bytes into the Link Control structure
func (lc *LinkControl) Decode(data []byte) error {
	if len(data) < 9 {
		return fmt.Errorf("Link Control data too short: got %d bytes, need 9", len(data))
	}

	// Byte 0: FLCO and reserved bits
	lc.FLCO = (data[0] >> 2) & 0x3F

	// Bytes 1-3: Destination ID (24-bit, big-endian)
	lc.DestinationID = (uint32(data[1]) << 16) | (uint32(data[2]) << 8) | uint32(data[3])

	// Bytes 4-6: Source ID (24-bit, big-endian)
	lc.SourceID = (uint32(data[4]) << 16) | (uint32(data[5]) << 8) | uint32(data[6])

	// Byte 7: Feature ID
	lc.FID = data[7]

	// Byte 8: Options
	lc.Options = data[8]

	return nil
}

// Parse parses embedded data from 8 bytes
func (emb *EmbeddedData) Parse(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("embedded data too short: got %d bytes, need 8", len(data))
	}

	// Extract LCSS and EMB from first byte
	emb.LCSS = (data[0] >> 1) & 0x03
	emb.EMB = data[0] & 0x0F

	// Copy embedded data
	emb.Data = make([]byte, 8)
	copy(emb.Data, data)

	return nil
}

// Encode encodes the slot type into bytes with error correction
func (st *SlotType) Encode() []byte {
	// Simplified encoding - real implementation would use Golay codes
	data := make([]byte, 1)
	data[0] = (st.ColorCode << 4) | (st.DataType & 0x0F)
	return data
}

// Decode decodes bytes into the slot type structure
func (st *SlotType) Decode(data []byte) error {
	if len(data) < 1 {
		return fmt.Errorf("slot type data too short")
	}

	// Simplified decoding - real implementation would use Golay codes
	st.ColorCode = (data[0] >> 4) & 0x0F
	st.DataType = data[0] & 0x0F

	// Validate color code
	if st.ColorCode > COLOR_CODE_MAX {
		return fmt.Errorf("invalid color code: %d", st.ColorCode)
	}

	return nil
}

// DetectSync detects the sync pattern in the data
func DetectSync(data []byte) SyncType {
	if len(data) < 6 {
		return SYNC_NONE
	}

	// Check for voice sync
	if bytesEqual(data[0:6], DMR_VOICE_SYNC) {
		return SYNC_VOICE
	}

	// Check for data sync
	if bytesEqual(data[0:6], DMR_DATA_SYNC) {
		return SYNC_DATA
	}

	return SYNC_NONE
}

// bytesEqual compares two byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}