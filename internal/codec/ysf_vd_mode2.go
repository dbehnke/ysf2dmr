package codec

// YSFVDMode2 implements YSF Voice/Data Mode 2 encoding/decoding for callsigns
// This matches the C++ CYSFPayload class functionality for YSF callsign protection
//
// Mode 2 parameters:
// - Handles 10-byte callsign data (YSF_CALLSIGN_LENGTH)
// - Uses WHITENING_DATA for scrambling protection
// - CCITT162 CRC for error detection (2 bytes)
// - Rate 1/2 convolutional encoding (100 info bits → 200 encoded bits)
// - 5×20 interleaving pattern (INTERLEAVE_TABLE_5_20)
// - Stores in YSF DCH payload structure (5×5 bytes = 25 bytes)

// Constants from C++ implementation
const (
	YSF_VD_MODE2_CALLSIGN_LENGTH = 10  // Callsign data length
	YSF_VD_MODE2_DATA_LENGTH     = 12  // Total data length (callsign + CRC)
	YSF_VD_MODE2_ENCODED_LENGTH  = 25  // Encoded data length in bytes
	YSF_VD_MODE2_INFO_BITS       = 96  // Information bits (12 bytes * 8 - 4 padding)
	YSF_VD_MODE2_ENCODED_BITS    = 200 // Encoded bits (rate 1/2)
	YSF_VD_MODE2_INTERLEAVE_SIZE = 100 // Interleave table size
)

// YSF frame structure constants
const (
	YSF_SYNC_LENGTH_BYTES = 5  // YSF sync pattern length
	YSF_FICH_LENGTH_BYTES = 25 // YSF FICH length
	YSF_DCH_SECTION_SIZE  = 18 // DCH section size
	YSF_DCH_SECTIONS      = 5  // Number of DCH sections
)

// INTERLEAVE_TABLE_5_20 for VD Mode 2 (100 entries for 5×20 pattern)
var YSF_VD_MODE2_INTERLEAVE_TABLE = [YSF_VD_MODE2_INTERLEAVE_SIZE]uint32{
	0, 40, 80, 120, 160,
	2, 42, 82, 122, 162,
	4, 44, 84, 124, 164,
	6, 46, 86, 126, 166,
	8, 48, 88, 128, 168,
	10, 50, 90, 130, 170,
	12, 52, 92, 132, 172,
	14, 54, 94, 134, 174,
	16, 56, 96, 136, 176,
	18, 58, 98, 138, 178,
	20, 60, 100, 140, 180,
	22, 62, 102, 142, 182,
	24, 64, 104, 144, 184,
	26, 66, 106, 146, 186,
	28, 68, 108, 148, 188,
	30, 70, 110, 150, 190,
	32, 72, 112, 152, 192,
	34, 74, 114, 154, 194,
	36, 76, 116, 156, 196,
	38, 78, 118, 158, 198,
}

// YSF_VD_MODE2_WHITENING_DATA matches C++ WHITENING_DATA
var YSF_VD_MODE2_WHITENING_DATA = [20]uint8{
	0x93, 0xD7, 0x51, 0x21, 0x9C, 0x2F, 0x6C, 0xD0, 0xEF, 0x0F,
	0xF8, 0x3D, 0xF1, 0x73, 0x20, 0x94, 0xED, 0x1E, 0x7C, 0xD8,
}

// YSF_VD_MODE2_BIT_MASK_TABLE for bit manipulation
var YSF_VD_MODE2_BIT_MASK_TABLE = [8]uint8{0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01}

// YSFVDMode2 represents a VD Mode 2 encoder/decoder
type YSFVDMode2 struct {
	conv *YSFConvolution // Convolutional encoder/decoder
}

// NewYSFVDMode2 creates a new YSF VD Mode 2 encoder/decoder
func NewYSFVDMode2() *YSFVDMode2 {
	return &YSFVDMode2{
		conv: NewYSFConvolution(),
	}
}

// writeBit writes a bit to a byte array at the specified bit position
func (vd *YSFVDMode2) writeBit(data []uint8, pos uint32, bit bool) {
	bytePos := pos >> 3
	bitPos := pos & 7

	if bytePos < uint32(len(data)) {
		if bit {
			data[bytePos] |= YSF_VD_MODE2_BIT_MASK_TABLE[bitPos]
		} else {
			data[bytePos] &= ^YSF_VD_MODE2_BIT_MASK_TABLE[bitPos]
		}
	}
}

// readBit reads a bit from a byte array at the specified bit position
func (vd *YSFVDMode2) readBit(data []uint8, pos uint32) bool {
	bytePos := pos >> 3
	bitPos := pos & 7

	if bytePos < uint32(len(data)) {
		return (data[bytePos] & YSF_VD_MODE2_BIT_MASK_TABLE[bitPos]) != 0
	}
	return false
}

// EncodeCallsign encodes a callsign string for VD Mode 2 transmission
// Input: callsign string (up to 10 characters)
// Output: 25-byte encoded data ready for YSF payload
// Equivalent to C++ CYSFPayload::writeVDMode2Data()
func (vd *YSFVDMode2) EncodeCallsign(callsign string) [YSF_VD_MODE2_ENCODED_LENGTH]uint8 {
	var result [YSF_VD_MODE2_ENCODED_LENGTH]uint8

	// Prepare callsign data (pad to 10 bytes)
	var callsignData [YSF_VD_MODE2_CALLSIGN_LENGTH]uint8
	for i := 0; i < YSF_VD_MODE2_CALLSIGN_LENGTH; i++ {
		if i < len(callsign) {
			callsignData[i] = callsign[i]
		} else {
			callsignData[i] = ' ' // Pad with spaces
		}
	}

	// Create data with callsign + CRC space
	var data [YSF_VD_MODE2_DATA_LENGTH + 1]uint8 // +1 for padding

	// Copy callsign data
	copy(data[:YSF_VD_MODE2_CALLSIGN_LENGTH], callsignData[:])

	// Apply whitening to first 10 bytes (C++ lines 346-347)
	for i := 0; i < YSF_VD_MODE2_CALLSIGN_LENGTH; i++ {
		data[i] ^= YSF_VD_MODE2_WHITENING_DATA[i]
	}

	// Add CCITT162 CRC (C++ line 349)
	AddCCITT162(data[:], YSF_VD_MODE2_DATA_LENGTH)
	data[YSF_VD_MODE2_DATA_LENGTH] = 0x00 // Clear padding byte

	// Convolutional encode 100 bits (C++ lines 352-355)
	var convolved [25]uint8
	vd.conv.Encode(data[:], convolved[:], YSF_VD_MODE2_INTERLEAVE_SIZE)

	// Interleave the encoded data (C++ lines 357-372)
	var bytes [25]uint8
	j := uint32(0)
	for i := uint32(0); i < YSF_VD_MODE2_INTERLEAVE_SIZE; i++ {
		n := YSF_VD_MODE2_INTERLEAVE_TABLE[i]

		// Read two bits from convolved data
		s0 := vd.readBit(convolved[:], j)
		j++
		s1 := vd.readBit(convolved[:], j)
		j++

		// Write interleaved bits
		vd.writeBit(bytes[:], n, s0)
		vd.writeBit(bytes[:], n+1, s1)
	}

	// Copy result (C++ lines 374-379)
	copy(result[:], bytes[:])

	return result
}

// DecodeCallsign decodes a VD Mode 2 encoded callsign
// Input: 25-byte encoded data from YSF payload
// Output: decoded callsign string and success flag
// Equivalent to C++ CYSFPayload::readVDMode2Data()
func (vd *YSFVDMode2) DecodeCallsign(encoded [YSF_VD_MODE2_ENCODED_LENGTH]uint8) (string, bool) {
	// Extract data from payload structure (C++ lines 437-443)
	var dch [25]uint8
	copy(dch[:], encoded[:])

	// Start convolutional decoder (C++ lines 445-446)
	vd.conv.Start()

	// De-interleave and decode (C++ lines 448-456)
	for i := uint32(0); i < YSF_VD_MODE2_INTERLEAVE_SIZE; i++ {
		n := YSF_VD_MODE2_INTERLEAVE_TABLE[i]

		// Read interleaved bits
		s0 := uint8(0)
		if vd.readBit(dch[:], n) {
			s0 = 1
		}

		s1 := uint8(0)
		if vd.readBit(dch[:], n+1) {
			s1 = 1
		}

		// Decode symbols
		vd.conv.Decode(s0, s1)
	}

	// Chainback to get decoded data (C++ line 459)
	var output [13]uint8
	vd.conv.Chainback(output[:], YSF_VD_MODE2_INFO_BITS)

	// Check CRC (C++ line 461)
	if !CheckCCITT162(output[:], YSF_VD_MODE2_DATA_LENGTH) {
		return "", false
	}

	// Remove whitening from first 10 bytes (C++ lines 463-464)
	for i := 0; i < YSF_VD_MODE2_CALLSIGN_LENGTH; i++ {
		output[i] ^= YSF_VD_MODE2_WHITENING_DATA[i]
	}

	// Extract callsign (C++ line 468)
	callsign := string(output[:YSF_VD_MODE2_CALLSIGN_LENGTH])

	// Trim trailing spaces
	for len(callsign) > 0 && callsign[len(callsign)-1] == ' ' {
		callsign = callsign[:len(callsign)-1]
	}

	return callsign, true
}

// ExtractFromYSFPayload extracts VD Mode 2 data from a YSF frame payload
// Input: complete YSF frame data
// Output: 25-byte VD Mode 2 data ready for decoding
func (vd *YSFVDMode2) ExtractFromYSFPayload(ysfFrame []uint8) ([YSF_VD_MODE2_ENCODED_LENGTH]uint8, bool) {
	var result [YSF_VD_MODE2_ENCODED_LENGTH]uint8

	if len(ysfFrame) < YSF_SYNC_LENGTH_BYTES+YSF_FICH_LENGTH_BYTES+(YSF_DCH_SECTIONS*YSF_DCH_SECTION_SIZE) {
		return result, false
	}

	// Skip sync and FICH to get to payload
	payloadStart := YSF_SYNC_LENGTH_BYTES + YSF_FICH_LENGTH_BYTES

	// Extract 5 bytes from each of 5 DCH sections (C++ lines 438-443)
	for i := 0; i < YSF_DCH_SECTIONS; i++ {
		sectionStart := payloadStart + (i * YSF_DCH_SECTION_SIZE)
		copy(result[i*5:(i+1)*5], ysfFrame[sectionStart:sectionStart+5])
	}

	return result, true
}

// InsertIntoYSFPayload inserts VD Mode 2 data into a YSF frame payload
// Input: YSF frame data and 25-byte VD Mode 2 encoded data
// Output: success flag
func (vd *YSFVDMode2) InsertIntoYSFPayload(ysfFrame []uint8, encoded [YSF_VD_MODE2_ENCODED_LENGTH]uint8) bool {
	if len(ysfFrame) < YSF_SYNC_LENGTH_BYTES+YSF_FICH_LENGTH_BYTES+(YSF_DCH_SECTIONS*YSF_DCH_SECTION_SIZE) {
		return false
	}

	// Skip sync and FICH to get to payload
	payloadStart := YSF_SYNC_LENGTH_BYTES + YSF_FICH_LENGTH_BYTES

	// Insert 5 bytes into each of 5 DCH sections (C++ lines 374-379)
	for i := 0; i < YSF_DCH_SECTIONS; i++ {
		sectionStart := payloadStart + (i * YSF_DCH_SECTION_SIZE)
		copy(ysfFrame[sectionStart:sectionStart+5], encoded[i*5:(i+1)*5])
	}

	return true
}

// ValidateCallsign checks if a callsign string is valid for VD Mode 2
func (vd *YSFVDMode2) ValidateCallsign(callsign string) bool {
	if len(callsign) == 0 || len(callsign) > YSF_VD_MODE2_CALLSIGN_LENGTH {
		return false
	}

	// Check for valid characters (alphanumeric and space)
	for _, char := range callsign {
		if !((char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == ' ') {
			return false
		}
	}

	return true
}

// GetDataLength returns the data length for VD Mode 2
func (vd *YSFVDMode2) GetDataLength() int {
	return YSF_VD_MODE2_CALLSIGN_LENGTH
}

// GetEncodedLength returns the encoded data length for VD Mode 2
func (vd *YSFVDMode2) GetEncodedLength() int {
	return YSF_VD_MODE2_ENCODED_LENGTH
}

// TestRoundTrip tests round-trip encoding and decoding
func (vd *YSFVDMode2) TestRoundTrip(callsign string) bool {
	if !vd.ValidateCallsign(callsign) {
		return false
	}

	// Encode
	encoded := vd.EncodeCallsign(callsign)

	// Decode
	decoded, ok := vd.DecodeCallsign(encoded)
	if !ok {
		return false
	}

	// Compare (handle padding)
	originalPadded := callsign
	for len(originalPadded) < YSF_VD_MODE2_CALLSIGN_LENGTH {
		originalPadded += " "
	}

	decodedPadded := decoded
	for len(decodedPadded) < YSF_VD_MODE2_CALLSIGN_LENGTH {
		decodedPadded += " "
	}

	return originalPadded[:YSF_VD_MODE2_CALLSIGN_LENGTH] == decodedPadded[:YSF_VD_MODE2_CALLSIGN_LENGTH]
}

// GetInterleavePattern returns the interleave pattern for debugging
func (vd *YSFVDMode2) GetInterleavePattern() [YSF_VD_MODE2_INTERLEAVE_SIZE]uint32 {
	return YSF_VD_MODE2_INTERLEAVE_TABLE
}

// GetWhiteningPattern returns the whitening pattern for debugging
func (vd *YSFVDMode2) GetWhiteningPattern() [20]uint8 {
	return YSF_VD_MODE2_WHITENING_DATA
}