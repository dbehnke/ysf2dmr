package codec

import (
	"fmt"
)

// ModeConv handles conversion between YSF and DMR AMBE formats
// This implementation matches the C++ YSF2DMR ModeConv class behavior
type ModeConv struct {
	// Frame counters (matching C++ m_ysfN, m_dmrN)
	ysfFrameCount uint32
	dmrFrameCount uint32

	// Ring buffers for frame management (matching C++ CRingBuffer)
	ysfBuffer *RingBuffer
	dmrBuffer *RingBuffer

	debugEnabled bool
}

// FrameData represents a frame with its tag
type FrameData struct {
	Tag  uint8
	Data []uint8
}

// AMBEVoiceParameters represents the AMBE voice parameters
type AMBEVoiceParameters struct {
	A uint32 // 24-bit fundamental frequency and voicing decision
	B uint32 // 23-bit spectral coefficients
	C uint32 // 25-bit additional voice parameters
}

// NewModeConv creates a new mode converter instance
func NewModeConv() *ModeConv {
	// Buffer sizes based on typical usage patterns
	// YSF frames are larger and less frequent, DMR frames are smaller and more frequent
	const YSF_BUFFER_SIZE = 2000 // ~120 bytes per YSF frame, ~16 frames
	const DMR_BUFFER_SIZE = 1000 // ~9 bytes per DMR frame, ~110 frames

	return &ModeConv{
		ysfFrameCount: 0,
		dmrFrameCount: 0,
		ysfBuffer:     NewRingBuffer(YSF_BUFFER_SIZE, "YSF"),
		dmrBuffer:     NewRingBuffer(DMR_BUFFER_SIZE, "DMR"),
		debugEnabled:  false,
	}
}

// SetDebug enables or disables debug logging
func (m *ModeConv) SetDebug(enabled bool) {
	m.debugEnabled = enabled
}

// PutDMR processes incoming DMR data and converts to YSF format
// This matches the C++ putDMR() function
func (m *ModeConv) PutDMR(dmrBytes []uint8) error {
	if len(dmrBytes) < 9 {
		return fmt.Errorf("DMR frame too short: %d bytes, need 9", len(dmrBytes))
	}

	m.logDebug("Processing DMR frame, size: %d bytes", len(dmrBytes))

	// Extract AMBE voice parameters using the C++ algorithm
	// Process 3 sets of voice parameters per DMR frame (like C++)
	for frame := 0; frame < 3; frame++ {
		params, err := m.extractDMRAMBE(dmrBytes, frame)
		if err != nil {
			return fmt.Errorf("failed to extract DMR AMBE frame %d: %v", frame, err)
		}

		// Convert AMBE parameters to YSF format
		err = m.putAMBE2YSF(params)
		if err != nil {
			return fmt.Errorf("failed to convert AMBE to YSF: %v", err)
		}
	}

	m.dmrFrameCount++
	return nil
}

// PutYSF processes incoming YSF data and converts to DMR format
// This matches the C++ putYSF() function
func (m *ModeConv) PutYSF(ysfBytes []uint8) error {
	if len(ysfBytes) < 120 {
		return fmt.Errorf("YSF frame too short: %d bytes, need 120", len(ysfBytes))
	}

	m.logDebug("Processing YSF frame, size: %d bytes", len(ysfBytes))

	// Skip sync and FICH, process payload starting at offset 40
	data := ysfBytes[40:] // Skip YSF_SYNC_LENGTH_BYTES + YSF_FICH_LENGTH_BYTES

	offset := 0

	// Process 5 VCH sections (matching C++ implementation)
	for j := 0; j < 5; j++ {
		params, err := m.extractYSFAMBE(data, offset)
		if err != nil {
			return fmt.Errorf("failed to extract YSF AMBE section %d: %v", j, err)
		}

		// Convert AMBE parameters to DMR format
		err = m.putAMBE2DMR(params)
		if err != nil {
			return fmt.Errorf("failed to convert AMBE to DMR: %v", err)
		}

		offset += 144 // Move to next VCH section
	}

	m.ysfFrameCount++
	return nil
}

// extractDMRAMBE extracts AMBE voice parameters from DMR frame
// Implements the complex bit extraction algorithm from C++ putDMR()
func (m *ModeConv) extractDMRAMBE(dmrBytes []uint8, frameIndex int) (*AMBEVoiceParameters, error) {
	params := &AMBEVoiceParameters{}

	// Extract A parameters (24 bits) using DMR_A_TABLE
	var a uint32 = 0
	mask := uint32(0x800000) // Start with MSB

	for i := 0; i < 24; i++ {
		aPos := DMR_A_TABLE[i]

		// Calculate positions for the 3 AMBE frames
		a1Pos := aPos
		a2Pos := a1Pos + 72
		if a2Pos >= 108 {
			a2Pos += 48
		}
		a3Pos := a1Pos + 192

		// Choose position based on frame index
		var bitPos uint32
		switch frameIndex {
		case 0:
			bitPos = a1Pos
		case 1:
			bitPos = a2Pos
		case 2:
			bitPos = a3Pos
		default:
			return nil, fmt.Errorf("invalid frame index: %d", frameIndex)
		}

		if m.readBit(dmrBytes, bitPos) {
			a |= mask
		}
		mask >>= 1
	}

	// Extract B parameters (23 bits) using DMR_B_TABLE
	var b uint32 = 0
	mask = 0x400000 // 23-bit mask

	for i := 0; i < 23; i++ {
		bPos := DMR_B_TABLE[i]

		// Calculate positions for the 3 AMBE frames
		b1Pos := bPos
		b2Pos := b1Pos + 72
		if b2Pos >= 108 {
			b2Pos += 48
		}
		b3Pos := b1Pos + 192

		// Choose position based on frame index
		var bitPos uint32
		switch frameIndex {
		case 0:
			bitPos = b1Pos
		case 1:
			bitPos = b2Pos
		case 2:
			bitPos = b3Pos
		default:
			return nil, fmt.Errorf("invalid frame index: %d", frameIndex)
		}

		if m.readBit(dmrBytes, bitPos) {
			b |= mask
		}
		mask >>= 1
	}

	// Extract C parameters (25 bits) using DMR_C_TABLE
	var c uint32 = 0
	mask = 0x1000000 // 25-bit mask

	for i := 0; i < 25; i++ {
		cPos := DMR_C_TABLE[i]

		// Calculate positions for the 3 AMBE frames
		c1Pos := cPos
		c2Pos := c1Pos + 72
		if c2Pos >= 108 {
			c2Pos += 48
		}
		c3Pos := c1Pos + 192

		// Choose position based on frame index
		var bitPos uint32
		switch frameIndex {
		case 0:
			bitPos = c1Pos
		case 1:
			bitPos = c2Pos
		case 2:
			bitPos = c3Pos
		default:
			return nil, fmt.Errorf("invalid frame index: %d", frameIndex)
		}

		if m.readBit(dmrBytes, bitPos) {
			c |= mask
		}
		mask >>= 1
	}

	// Decode using Golay error correction (matching C++ behavior)
	params.A = Decode24128(a) // Extract corrected 12-bit data
	params.B = Decode23127(b << 1) // Shift back and decode 11-bit data
	params.C = c // dat_c is used directly

	m.logDebug("Extracted DMR AMBE frame %d: A=0x%03X, B=0x%03X, C=0x%07X", frameIndex, params.A, params.B, params.C)

	return params, nil
}

// extractYSFAMBE extracts AMBE voice parameters from YSF VCH section
// Implements the algorithm from C++ putYSF()
func (m *ModeConv) extractYSFAMBE(data []uint8, offset int) (*AMBEVoiceParameters, error) {
	params := &AMBEVoiceParameters{}

	vch := make([]uint8, 13) // 104 bits = 13 bytes

	// Deinterleave using INTERLEAVE_TABLE_26_4
	for i := 0; i < 104; i++ {
		n := INTERLEAVE_TABLE_26_4[i]
		if m.readBit(data, uint32(offset)+n) {
			m.writeBit(vch, uint32(i), true)
		}
	}

	// "Un-whiten" (descramble) with WHITENING_DATA
	for i := 0; i < 13; i++ {
		vch[i] ^= WHITENING_DATA[i]
	}

	// Extract dat_a (12 bits) from triple redundancy encoding
	var datA uint32 = 0
	for i := 0; i < 12; i++ {
		datA <<= 1
		if m.readBit(vch, uint32(3*i+1)) {
			datA |= 0x01
		}
	}

	// Extract dat_b (12 bits) from triple redundancy encoding
	var datB uint32 = 0
	for i := 0; i < 12; i++ {
		datB <<= 1
		if m.readBit(vch, uint32(3*(i+12)+1)) {
			datB |= 0x01
		}
	}

	// Extract dat_c (25 bits) from VCH
	var datC uint32 = 0

	// First 3 bits from triple redundancy
	for i := 0; i < 3; i++ {
		datC <<= 1
		if m.readBit(vch, uint32(3*(i+24)+1)) {
			datC |= 0x01
		}
	}

	// Remaining 22 bits from direct encoding
	for i := 0; i < 22; i++ {
		datC <<= 1
		if m.readBit(vch, uint32(i+81)) {
			datC |= 0x01
		}
	}

	params.A = datA
	params.B = datB
	params.C = datC

	m.logDebug("Extracted YSF AMBE: A=0x%03X, B=0x%03X, C=0x%07X", datA, datB, datC)

	return params, nil
}

// putAMBE2YSF converts AMBE voice parameters to YSF VCH format
// Implements the complex algorithm from C++ putAMBE2YSF()
func (m *ModeConv) putAMBE2YSF(params *AMBEVoiceParameters) error {
	// Split dat_a into 12-bit components
	datA := params.A >> 12 // Upper 12 bits

	// Apply PRNG scrambling to dat_b (matching C++ algorithm)
	datB := params.B
	datB ^= (PRNG_TABLE[datA] >> 1)

	vch := make([]uint8, 13) // 104 bits = 13 bytes

	// Pack VCH with triple redundancy for dat_a (12 bits -> 36 bits)
	for i := 0; i < 12; i++ {
		bit := (datA << (20 + i)) & 0x80000000
		bitValue := bit != 0
		m.writeBit(vch, uint32(3*i+0), bitValue)
		m.writeBit(vch, uint32(3*i+1), bitValue)
		m.writeBit(vch, uint32(3*i+2), bitValue)
	}

	// Pack dat_b with triple redundancy (12 bits -> 36 bits)
	for i := 0; i < 12; i++ {
		bit := (datB << (20 + i)) & 0x80000000
		bitValue := bit != 0
		m.writeBit(vch, uint32(3*(i+12)+0), bitValue)
		m.writeBit(vch, uint32(3*(i+12)+1), bitValue)
		m.writeBit(vch, uint32(3*(i+12)+2), bitValue)
	}

	// Pack dat_c (25 bits) - first 3 bits with triple redundancy, rest direct
	datC := params.C

	// First 3 bits with triple redundancy
	for i := 0; i < 3; i++ {
		bit := (datC << (7 + i)) & 0x80000000
		bitValue := bit != 0
		m.writeBit(vch, uint32(3*(i+24)+0), bitValue)
		m.writeBit(vch, uint32(3*(i+24)+1), bitValue)
		m.writeBit(vch, uint32(3*(i+24)+2), bitValue)
	}

	// Remaining 22 bits direct
	for i := 0; i < 22; i++ {
		bit := (datC << (10 + i)) & 0x80000000
		bitValue := bit != 0
		m.writeBit(vch, uint32(i+81), bitValue)
	}

	// Apply whitening (scrambling) with WHITENING_DATA
	for i := 0; i < 13; i++ {
		vch[i] ^= WHITENING_DATA[i]
	}

	// Interleave using INTERLEAVE_TABLE_26_4
	ysfFrame := make([]uint8, 120) // Full YSF frame
	for i := 0; i < 104; i++ {
		n := INTERLEAVE_TABLE_26_4[i]
		if m.readBit(vch, uint32(i)) {
			m.writeBit(ysfFrame, n, true)
		}
	}

	// Add tag and frame to YSF buffer (matching C++ m_YSF.addData(&TAG_DATA, 1U))
	m.ysfBuffer.AddSingleByte(TAG_DATA)
	m.ysfBuffer.AddData(ysfFrame)

	m.logDebug("Converted AMBE to YSF, added to buffer")

	return nil
}

// putAMBE2DMR converts AMBE voice parameters to DMR format
// Implements the algorithm from C++ putAMBE2DMR()
func (m *ModeConv) putAMBE2DMR(params *AMBEVoiceParameters) error {
	dmrFrame := make([]uint8, 9) // DMR voice frame is 9 bytes

	// Apply Golay error correction to dat_a (matching C++ CGolay24128::encode24128)
	a := Encode24128(params.A)

	// Apply PRNG scrambling and Golay to dat_b (matching C++ algorithm)
	p := PRNG_TABLE[params.A] >> 1
	b := Encode23127(params.B) >> 1
	b ^= p

	c := params.C // dat_c is used directly

	// Distribute bits using DMR A/B/C tables

	// Write A bits (24 bits)
	mask := uint32(0x800000)
	for i := 0; i < 24; i++ {
		aPos := DMR_A_TABLE[i]
		bitValue := (a & mask) != 0
		m.writeBit(dmrFrame, aPos, bitValue)
		mask >>= 1
	}

	// Write B bits (23 bits)
	mask = 0x400000
	for i := 0; i < 23; i++ {
		bPos := DMR_B_TABLE[i]
		bitValue := (b & mask) != 0
		m.writeBit(dmrFrame, bPos, bitValue)
		mask >>= 1
	}

	// Write C bits (25 bits)
	mask = 0x1000000
	for i := 0; i < 25; i++ {
		cPos := DMR_C_TABLE[i]
		bitValue := (c & mask) != 0
		m.writeBit(dmrFrame, cPos, bitValue)
		mask >>= 1
	}

	// Add tag and frame to DMR buffer (matching C++ m_DMR.addData(&TAG_DATA, 1U))
	m.dmrBuffer.AddSingleByte(TAG_DATA)
	m.dmrBuffer.AddData(dmrFrame)

	m.dmrFrameCount++
	m.logDebug("Converted AMBE to DMR frame %d, added to buffer", m.dmrFrameCount)

	return nil
}

// Helper functions for bit manipulation

// readBit reads a bit at the specified position from byte array
func (m *ModeConv) readBit(data []uint8, bitPos uint32) bool {
	bytePos := bitPos / 8
	bitOffset := bitPos % 8

	if bytePos >= uint32(len(data)) {
		return false
	}

	return (data[bytePos] & (0x80 >> bitOffset)) != 0
}

// writeBit writes a bit at the specified position in byte array
func (m *ModeConv) writeBit(data []uint8, bitPos uint32, value bool) {
	bytePos := bitPos / 8
	bitOffset := bitPos % 8

	if bytePos >= uint32(len(data)) {
		return
	}

	if value {
		data[bytePos] |= (0x80 >> bitOffset)
	} else {
		data[bytePos] &= ^(0x80 >> bitOffset)
	}
}

// GetYSF retrieves converted YSF frames from buffer
// Returns the frame data and tag
func (m *ModeConv) GetYSF() ([]uint8, uint8, bool) {
	if !m.ysfBuffer.HasData() {
		return nil, 0, false
	}

	// Get tag first (1 byte)
	tag, success := m.ysfBuffer.GetData(1)
	if !success {
		return nil, 0, false
	}

	// Get frame data (120 bytes for YSF)
	frame, success := m.ysfBuffer.GetData(120)
	if !success {
		return nil, tag[0], false
	}

	return frame, tag[0], true
}

// GetDMR retrieves converted DMR frames from buffer
// Returns the frame data and tag
func (m *ModeConv) GetDMR() ([]uint8, uint8, bool) {
	if !m.dmrBuffer.HasData() {
		return nil, 0, false
	}

	// Get tag first (1 byte)
	tag, success := m.dmrBuffer.GetData(1)
	if !success {
		return nil, 0, false
	}

	// Get frame data (9 bytes for DMR)
	frame, success := m.dmrBuffer.GetData(9)
	if !success {
		return nil, tag[0], false
	}

	return frame, tag[0], true
}

// Reset clears all buffers and counters
func (m *ModeConv) Reset() {
	m.ysfFrameCount = 0
	m.dmrFrameCount = 0
	m.ysfBuffer.Clear()
	m.dmrBuffer.Clear()
}

// GetStats returns conversion statistics
func (m *ModeConv) GetStats() (ysfCount, dmrCount uint32) {
	return m.ysfFrameCount, m.dmrFrameCount
}

// HasYSFData checks if there's YSF data available
func (m *ModeConv) HasYSFData() bool {
	return m.ysfBuffer.HasData()
}

// HasDMRData checks if there's DMR data available
func (m *ModeConv) HasDMRData() bool {
	return m.dmrBuffer.HasData()
}

// GetYSFDataSize returns the amount of YSF data available
func (m *ModeConv) GetYSFDataSize() uint32 {
	return m.ysfBuffer.DataSize()
}

// GetDMRDataSize returns the amount of DMR data available
func (m *ModeConv) GetDMRDataSize() uint32 {
	return m.dmrBuffer.DataSize()
}

// GetYSFFreeSpace returns free space in YSF buffer
func (m *ModeConv) GetYSFFreeSpace() uint32 {
	return m.ysfBuffer.FreeSpace()
}

// GetDMRFreeSpace returns free space in DMR buffer
func (m *ModeConv) GetDMRFreeSpace() uint32 {
	return m.dmrBuffer.FreeSpace()
}

// logDebug logs debug messages if debug is enabled
func (m *ModeConv) logDebug(format string, args ...interface{}) {
	if m.debugEnabled {
		fmt.Printf("ModeConv: "+format+"\n", args...)
	}
}