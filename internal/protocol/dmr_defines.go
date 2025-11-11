package protocol

// DMR protocol constants equivalent to DMRDefines.h

const (
	// Frame and packet constants
	DMR_FRAME_LENGTH_BYTES         = 33  // DMR frame payload length
	DMR_SLOT_TIME                 = 60  // DMR slot time in milliseconds
	HOMEBREW_DATA_PACKET_LENGTH   = 55  // Total DMRD packet size

	// Network packet lengths
	NETWORK_LOGIN_LENGTH          = 8   // RPTL packet
	NETWORK_AUTH_LENGTH           = 40  // RPTK packet
	NETWORK_CONFIG_LENGTH         = 302 // RPTC packet
	NETWORK_PING_LENGTH           = 11  // RPTPING packet
	NETWORK_CLOSE_LENGTH          = 9   // RPTCL packet
	NETWORK_PONG_LENGTH           = 7   // MSTPONG packet
	NETWORK_ACK_LENGTH            = 6   // RPTACK packet (minimum)
	NETWORK_NAK_LENGTH            = 7   // MSTNAK packet
	NETWORK_BEACON_LENGTH         = 8   // RPTSBKN packet
	NETWORK_POSITION_LENGTH       = 18  // DMRG packet
	NETWORK_TALKERALIAS_LENGTH    = 19  // DMRA packet

	// Timer constants
	DMR_RETRY_TIMEOUT             = 10000 // 10 seconds in milliseconds
	DMR_CONNECTION_TIMEOUT        = 60000 // 60 seconds in milliseconds

	// Salt and authentication
	DMR_SALT_LENGTH               = 4   // Server salt length
	DMR_AUTH_HASH_LENGTH          = 32  // SHA256 hash length
)

// DMR Network Status - equivalent to C++ STATUS enum
type DMRNetworkStatus int

const (
	DMR_WAITING_CONNECT DMRNetworkStatus = iota // Initial state, waiting to connect
	DMR_WAITING_LOGIN                          // Sent login, waiting for RPTACK
	DMR_WAITING_AUTHORISATION                  // Sent auth, waiting for RPTACK
	DMR_WAITING_CONFIG                         // Sent config, waiting for RPTACK
	DMR_WAITING_OPTIONS                        // Sent options, waiting for RPTACK
	DMR_RUNNING                                // Connected and authenticated
)

// FLCO Types (Frame Level Call Options)
const (
	FLCO_GROUP               = 0x00
	FLCO_USER_USER          = 0x03
	FLCO_TALKER_ALIAS_HEADER = 0x04
	FLCO_TALKER_ALIAS_BLOCK1 = 0x05
	FLCO_TALKER_ALIAS_BLOCK2 = 0x06
	FLCO_TALKER_ALIAS_BLOCK3 = 0x07
	FLCO_GPS_INFO           = 0x08
)

// Data Types
const (
	DT_VOICE                = 0xF1
	DT_VOICE_SYNC          = 0xF0
	DT_VOICE_LC_HEADER     = 0x01
	DT_TERMINATOR_WITH_LC  = 0x02
	DT_DATA_HEADER         = 0x06
	DT_RATE_12_DATA        = 0x07
	DT_RATE_34_DATA        = 0x08
	DT_IDLE                = 0x09
	DT_RATE_1_DATA         = 0x0A
)

// Delay Buffer Status
type DelayBufferStatus int

const (
	BS_NO_DATA DelayBufferStatus = iota
	BS_DATA
	BS_MISSING
)

// Network packet magic strings
const (
	NETWORK_MAGIC_LOGIN    = "RPTL"     // Login packet
	NETWORK_MAGIC_AUTH     = "RPTK"     // Authorization packet
	NETWORK_MAGIC_CONFIG   = "RPTC"     // Configuration packet
	NETWORK_MAGIC_OPTIONS  = "RPTO"     // Options packet
	NETWORK_MAGIC_PING     = "RPTPING"  // Ping packet
	NETWORK_MAGIC_CLOSE    = "RPTCL"    // Close packet
	NETWORK_MAGIC_DATA     = "DMRD"     // Data packet
	NETWORK_MAGIC_POSITION = "DMRG"     // Position packet
	NETWORK_MAGIC_TALKERALIAS = "DMRA"  // Talker alias packet

	// Server response packets
	NETWORK_MAGIC_ACK      = "RPTACK"   // Acknowledgement
	NETWORK_MAGIC_NAK      = "MSTNAK"   // Negative acknowledgement
	NETWORK_MAGIC_PONG     = "MSTPONG"  // Ping response
	NETWORK_MAGIC_CLOSE_MASTER = "MSTCL" // Master closing
	NETWORK_MAGIC_BEACON   = "RPTSBKN"  // Beacon request
)

// Slot numbers
const (
	DMR_SLOT_1 = 1
	DMR_SLOT_2 = 2
)

// Frame flag bit positions (byte 15 in DMRD packet)
const (
	DMR_SLOT_FLAG_BIT     = 7  // 0=Slot1, 1=Slot2
	DMR_CALL_TYPE_BIT     = 6  // 0=Group, 1=Private
	DMR_DATA_SYNC_BIT     = 5  // Data sync flag
	DMR_VOICE_SYNC_BIT    = 4  // Voice sync flag
)

// Hardware types
type HWType int

const (
	HW_TYPE_UNKNOWN HWType = iota
	HW_TYPE_HOMEBREW
	HW_TYPE_REPEATER
	HW_TYPE_HOTSPOT
)

func (h HWType) String() string {
	switch h {
	case HW_TYPE_HOMEBREW:
		return "HOMEBREW"
	case HW_TYPE_REPEATER:
		return "REPEATER"
	case HW_TYPE_HOTSPOT:
		return "HOTSPOT"
	default:
		return "UNKNOWN"
	}
}