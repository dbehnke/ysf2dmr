package ysf

import (
	"fmt"
	"strings"
)

// YSF frame constants
const (
	YSF_FRAME_LENGTH      = 155 // Total YSF frame length
	YSF_HEADER_LENGTH     = 35  // YSF header length
	YSF_PAYLOAD_LENGTH    = 120 // YSF payload length
	YSF_SYNC_LENGTH       = 5   // YSF sync pattern length
	YSF_FICH_LENGTH       = 25  // YSF FICH length
	YSF_MAGIC             = "YSFD"
	CALLSIGN_LENGTH       = 10  // YSF callsign field length
)

// YSF sync pattern
var YSF_SYNC = []byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}

// Frame Information CHannel (FICH) structure
type FICH struct {
	FI uint8 // Frame indicator (0=header, 1=communications, 2=terminator)
	DT uint8 // Data type (0=VD mode 1, 1=data, 2=VD mode 2, 3=voice FR)
	CM uint8 // Call mode (0=group, 1=group2, 3=individual)
	CS uint8 // Calling standards
	FN uint8 // Frame number (0-5)
	FT uint8 // Frame type (0=last, 1=not last)
	MR uint8 // Message route (0=direct, 1=not busy, 2=busy)
	BN uint8 // Block number
	BT uint8 // Block type
	SQL uint8 // Squelch
	VOIPIndicator uint8 // VOIP indicator
	DestinationID uint16 // Destination ID
	SourceID      uint16 // Source ID
}

// YSF Frame structure
type Frame struct {
	SourceCallsign string // Source callsign (up to 10 chars)
	DestCallsign   string // Destination callsign (up to 10 chars)
	FICH           FICH   // Frame Information CHannel
	Payload        []byte // Frame payload (90 bytes after FICH)
	RawData        []byte // Complete raw frame data
}

// Parse parses a YSF frame from raw bytes
func (f *Frame) Parse(data []byte) error {
	if len(data) < YSF_FRAME_LENGTH {
		return fmt.Errorf("YSF frame too short: got %d bytes, need %d", len(data), YSF_FRAME_LENGTH)
	}

	f.RawData = make([]byte, len(data))
	copy(f.RawData, data)

	// Check magic number
	if string(data[0:4]) != YSF_MAGIC {
		return fmt.Errorf("invalid YSF magic number: got %q, want %q", string(data[0:4]), YSF_MAGIC)
	}

	// Extract callsigns
	f.SourceCallsign = extractCallsign(data[4:14])
	f.DestCallsign = extractCallsign(data[14:24])

	// Check for YSF sync pattern at offset 35
	if !bytesEqual(data[35:40], YSF_SYNC) {
		return fmt.Errorf("invalid YSF sync pattern")
	}

	// Parse FICH (Frame Information CHannel) at offset 40
	err := f.FICH.Decode(data[40:65])
	if err != nil {
		return fmt.Errorf("failed to decode FICH: %v", err)
	}

	// Extract payload (90 bytes after FICH)
	f.Payload = make([]byte, 90)
	copy(f.Payload, data[65:155])

	return nil
}

// Build constructs a YSF frame from the structure
func (f *Frame) Build() []byte {
	frame := make([]byte, YSF_FRAME_LENGTH)

	// Magic number
	copy(frame[0:4], []byte(YSF_MAGIC))

	// Callsigns (padded to 10 bytes each)
	copy(frame[4:14], padCallsign(f.SourceCallsign))
	copy(frame[14:24], padCallsign(f.DestCallsign))

	// Reserved/other header fields (set to zero)
	// Bytes 24-34 are used for other purposes in some implementations

	// YSF sync pattern at offset 35
	copy(frame[35:40], YSF_SYNC)

	// FICH at offset 40
	fichData := f.FICH.Encode()
	copy(frame[40:65], fichData)

	// Payload at offset 65
	if len(f.Payload) >= 90 {
		copy(frame[65:155], f.Payload[0:90])
	} else {
		copy(frame[65:65+len(f.Payload)], f.Payload)
	}

	return frame
}

// IsHeader returns true if this is a header frame
func (f *Frame) IsHeader() bool {
	return f.FICH.FI == 0
}

// IsCommunications returns true if this is a communications frame
func (f *Frame) IsCommunications() bool {
	return f.FICH.FI == 1
}

// IsTerminator returns true if this is a terminator frame
func (f *Frame) IsTerminator() bool {
	return f.FICH.FI == 2
}

// IsVoice returns true if this frame contains voice data
func (f *Frame) IsVoice() bool {
	return f.FICH.DT == 0 || f.FICH.DT == 2 || f.FICH.DT == 3
}

// IsData returns true if this frame contains data
func (f *Frame) IsData() bool {
	return f.FICH.DT == 1
}

// IsGroupCall returns true if this is a group call
func (f *Frame) IsGroupCall() bool {
	return f.FICH.CM == 0 || f.FICH.CM == 1
}

// IsIndividualCall returns true if this is an individual call
func (f *Frame) IsIndividualCall() bool {
	return f.FICH.CM == 3
}

// Encode encodes the FICH structure into 25 bytes
func (fich *FICH) Encode() []byte {
	data := make([]byte, YSF_FICH_LENGTH)

	// Pack FICH fields into bytes
	// First byte: FI (2 bits) | DT (2 bits) | CM (2 bits) | CS (2 bits)
	data[0] = (fich.FI << 6) | (fich.DT << 4) | (fich.CM << 2) | fich.CS

	// Second byte: FN (3 bits) | FT (1 bit) | MR (2 bits) | reserved (2 bits)
	data[1] = (fich.FN << 5) | (fich.FT << 4) | (fich.MR << 2)

	// Remaining fields
	data[2] = fich.BN
	data[3] = fich.BT
	data[4] = fich.SQL
	data[5] = fich.VOIPIndicator

	// Destination ID (16-bit, big-endian)
	data[6] = uint8(fich.DestinationID >> 8)
	data[7] = uint8(fich.DestinationID & 0xFF)

	// Source ID (16-bit, big-endian)
	data[8] = uint8(fich.SourceID >> 8)
	data[9] = uint8(fich.SourceID & 0xFF)

	// Remaining bytes are typically used for error correction
	// For now, leave them as zeros

	return data
}

// Decode decodes 25 bytes into the FICH structure
func (fich *FICH) Decode(data []byte) error {
	if len(data) < YSF_FICH_LENGTH {
		return fmt.Errorf("FICH data too short: got %d bytes, need %d", len(data), YSF_FICH_LENGTH)
	}

	// Unpack FICH fields from bytes
	// First byte: FI (2 bits) | DT (2 bits) | CM (2 bits) | CS (2 bits)
	fich.FI = (data[0] >> 6) & 0x03
	fich.DT = (data[0] >> 4) & 0x03
	fich.CM = (data[0] >> 2) & 0x03
	fich.CS = data[0] & 0x03

	// Second byte: FN (3 bits) | FT (1 bit) | MR (2 bits) | reserved (2 bits)
	fich.FN = (data[1] >> 5) & 0x07
	fich.FT = (data[1] >> 4) & 0x01
	fich.MR = (data[1] >> 2) & 0x03

	// Remaining fields
	fich.BN = data[2]
	fich.BT = data[3]
	fich.SQL = data[4]
	fich.VOIPIndicator = data[5]

	// Destination ID (16-bit, big-endian)
	fich.DestinationID = (uint16(data[6]) << 8) | uint16(data[7])

	// Source ID (16-bit, big-endian)
	fich.SourceID = (uint16(data[8]) << 8) | uint16(data[9])

	return nil
}

// String returns a human-readable representation of the FICH
func (fich *FICH) String() string {
	frameTypes := []string{"Header", "Communications", "Terminator"}
	dataTypes := []string{"VD Mode 1", "Data", "VD Mode 2", "Voice FR"}
	callModes := []string{"Group", "Group2", "Reserved", "Individual"}

	frameType := "Unknown"
	if fich.FI < uint8(len(frameTypes)) {
		frameType = frameTypes[fich.FI]
	}

	dataType := "Unknown"
	if fich.DT < uint8(len(dataTypes)) {
		dataType = dataTypes[fich.DT]
	}

	callMode := "Unknown"
	if fich.CM < uint8(len(callModes)) {
		callMode = callModes[fich.CM]
	}

	return fmt.Sprintf("FICH{Type=%s, Data=%s, Call=%s, FN=%d, SrcID=%d, DstID=%d}",
		frameType, dataType, callMode, fich.FN, fich.SourceID, fich.DestinationID)
}

// extractCallsign extracts a callsign from a 10-byte field, removing padding
func extractCallsign(data []byte) string {
	if len(data) < CALLSIGN_LENGTH {
		return ""
	}

	// Convert to string and trim spaces and null bytes
	callsign := string(data)
	callsign = strings.TrimSpace(callsign)
	callsign = strings.Trim(callsign, "\x00")

	return callsign
}

// padCallsign pads a callsign to 10 bytes with spaces
func padCallsign(callsign string) []byte {
	data := make([]byte, CALLSIGN_LENGTH)

	// Copy callsign data
	copy(data, []byte(callsign))

	// Pad with spaces
	for i := len(callsign); i < CALLSIGN_LENGTH; i++ {
		data[i] = ' '
	}

	return data
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