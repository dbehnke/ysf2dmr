package protocol

// YSF protocol constants equivalent to YSFDefines.h

const (
	// Frame and network constants
	YSF_CALLSIGN_LENGTH = 10  // Callsign length in bytes (space-padded)
	YSF_FRAME_LENGTH    = 155 // Complete YSF frame length
	YSF_DATA_LENGTH     = 120 // YSF payload data length

	// Buffer constants
	BUFFER_LENGTH        = 200  // Maximum packet size for network operations
	RING_BUFFER_LENGTH   = 1000 // Internal ring buffer size

	// Frame type identifiers
	YSF_DT_VD_MODE1      = 0x00 // Voice/Data Mode 1
	YSF_DT_DATA_FR_MODE  = 0x01 // Data Frame Mode
	YSF_DT_VD_MODE2      = 0x02 // Voice/Data Mode 2
	YSF_DT_VOICE_FR_MODE = 0x03 // Voice Frame Mode
	YSF_DT_TERMINATOR    = 0x08 // Terminator Frame

	// Frame information
	YSF_FI_HEADER        = 0x00 // Header frame
	YSF_FI_COMMUNICATIONS = 0x01 // Communications frame
	YSF_FI_TERMINATOR    = 0x02 // Terminator frame
	YSF_FI_TEST          = 0x03 // Test frame

	// Sync patterns
	YSF_SYNC_LENGTH = 5
)

// YSF sync patterns
var (
	YSF_SYNC_BYTES = [YSF_SYNC_LENGTH]byte{0xD4, 0x71, 0xC9, 0x63, 0x4D}
)

// Message type constants for YSF network protocol
const (
	YSF_POLL_MESSAGE_LENGTH   = 14 // "YSFP" + 10-byte callsign
	YSF_UNLINK_MESSAGE_LENGTH = 14 // "YSFU" + 10-byte callsign
)

// YSF frame offsets
const (
	YSF_SYNC_OFFSET       = 0  // Sync pattern offset
	YSF_LENGTH_OFFSET     = 5  // Frame length offset
	YSF_TAG_OFFSET        = 6  // Frame tag offset
	YSF_SRC_OFFSET        = 14 // Source callsign offset
	YSF_DEST_OFFSET       = 24 // Destination callsign offset
	YSF_DOWNLINK_OFFSET   = 34 // Downlink callsign offset
	YSF_UPLINK_OFFSET     = 44 // Uplink callsign offset
	YSF_REM1_OFFSET       = 54 // Remote 1 callsign offset
	YSF_REM2_OFFSET       = 64 // Remote 2 callsign offset
	YSF_REM3_OFFSET       = 74 // Remote 3 callsign offset
	YSF_REM4_OFFSET       = 84 // Remote 4 callsign offset
	YSF_FCH_OFFSET        = 35 // Frame control header offset
	YSF_PAYLOAD_OFFSET    = 35 // Payload data offset
)