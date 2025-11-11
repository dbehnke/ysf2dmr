package codec

import (
	"fmt"
)

// YSFAMBEExtractor handles YSF AMBE frame extraction and processing
type YSFAMBEExtractor struct {
	// No state needed for extraction
}

// NewYSFAMBEExtractor creates a new YSF AMBE extractor
func NewYSFAMBEExtractor() *YSFAMBEExtractor {
	return &YSFAMBEExtractor{}
}

// ExtractVCHSections extracts 5 VCH sections from YSF payload
// Based on C++ YSFPayload::readVDMode1Data() and processVoice()
func (e *YSFAMBEExtractor) ExtractVCHSections(ysfPayload []byte) ([YSF_VCH_SECTIONS]YSFVCHSection, error) {
	if len(ysfPayload) < YSF_PAYLOAD_LENGTH {
		return [YSF_VCH_SECTIONS]YSFVCHSection{}, fmt.Errorf("YSF payload too short: got %d, need %d",
			len(ysfPayload), YSF_PAYLOAD_LENGTH)
	}

	var vchSections [YSF_VCH_SECTIONS]YSFVCHSection

	for i := 0; i < YSF_VCH_SECTIONS; i++ {
		// Extract VCH section i (104 bits = 13 bytes)
		err := e.extractVCHSection(ysfPayload, i, &vchSections[i])
		if err != nil {
			return [YSF_VCH_SECTIONS]YSFVCHSection{}, fmt.Errorf("failed to extract VCH section %d: %v", i, err)
		}
	}

	return vchSections, nil
}

// extractVCHSection extracts a single VCH section from YSF payload
// Implements the YSF voice processing algorithm from C++
func (e *YSFAMBEExtractor) extractVCHSection(payload []byte, sectionIndex int, vch *YSFVCHSection) error {
	// Clear the VCH section
	for i := range vch.Data {
		vch.Data[i] = 0
	}

	// Step 1: Extract the 312 interleaved bits for this VCH section (104 bits * 3 for triple redundancy)
	tripleBits := make([]bool, YSF_VCH_BITS*3) // 312 bits

	// Calculate the starting bit position for this VCH section in the payload
	// Each VCH section has 312 bits (104 * 3), sections are laid out sequentially
	startBitPos := sectionIndex * YSF_VCH_BITS * 3

	// Extract the interleaved triple bits
	for i := 0; i < YSF_VCH_BITS*3; i++ {
		bitPos := startBitPos + i
		if bitPos >= len(payload)*8 {
			break // Prevent buffer overflow
		}

		bytePos := bitPos / 8
		bitOffset := bitPos % 8

		if bytePos < len(payload) {
			tripleBits[i] = (payload[bytePos] & (1 << (7 - bitOffset))) != 0
		}
	}

	// Step 2: De-interleave the triple bits using the interleave table
	deinterleavedBits := make([]bool, YSF_VCH_BITS*3)
	e.deinterleave(tripleBits, deinterleavedBits)

	// Step 3: Remove whitening/scrambling
	dewhitenedBits := make([]bool, YSF_VCH_BITS*3)
	e.removeWhitening(deinterleavedBits, dewhitenedBits, sectionIndex)

	// Step 4: Perform majority voting on triple bits to get 104 voice bits
	voiceBits := make([]bool, YSF_VCH_BITS)
	for i := 0; i < YSF_VCH_BITS; i++ {
		// Each voice bit is represented by 3 consecutive bits - use majority vote
		bit1 := dewhitenedBits[i*3]
		bit2 := dewhitenedBits[i*3+1]
		bit3 := dewhitenedBits[i*3+2]

		// Majority vote: if 2 or more bits are true, the result is true
		voiceBits[i] = (bit1 && bit2) || (bit1 && bit3) || (bit2 && bit3)
	}

	// Step 5: Pack the 104 voice bits into 13 bytes
	for i := 0; i < YSF_VCH_BITS; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8) // MSB first

		if voiceBits[i] {
			vch.Data[byteIndex] |= (1 << bitIndex)
		}
	}

	return nil
}

// deinterleave removes the interleaving from YSF voice data
// Based on C++ INTERLEAVE_TABLE_26_4
func (e *YSFAMBEExtractor) deinterleave(src, dest []bool) {
	// The interleave table defines how bits are rearranged
	// We need to reverse this process to get the original bit order

	if len(src) != len(dest) || len(src) < YSF_VCH_BITS*3 {
		return
	}

	// Clear destination
	for i := range dest {
		dest[i] = false
	}

	// Apply reverse interleaving
	// The interleave table shows where each bit goes - we reverse it
	for i := 0; i < YSF_VCH_BITS*3 && i < len(INTERLEAVE_TABLE_26_4); i++ {
		srcIndex := INTERLEAVE_TABLE_26_4[i]
		if int(srcIndex) < len(src) {
			dest[i] = src[srcIndex]
		}
	}
}

// removeWhitening removes the whitening/scrambling from YSF voice data
// Based on C++ WHITENING_DATA pattern
func (e *YSFAMBEExtractor) removeWhitening(src, dest []bool, sectionIndex int) {
	if len(src) != len(dest) || len(src) < YSF_VCH_BITS*3 {
		return
	}

	// Copy input to output first
	copy(dest, src)

	// Apply whitening removal using the whitening pattern
	// The whitening pattern is applied cyclically
	for i := 0; i < len(src); i++ {
		// Calculate whitening byte and bit position
		whiteningByteIndex := (i + sectionIndex*YSF_VCH_BITS*3/8) % WHITENING_DATA_SIZE
		whiteningBitIndex := i % 8

		// Get the whitening bit
		whiteningBit := (WHITENING_DATA[whiteningByteIndex] & (1 << (7 - whiteningBitIndex))) != 0

		// XOR with whitening bit to remove scrambling
		dest[i] = src[i] != whiteningBit // XOR operation
	}
}

// ConvertVCHToAMBE converts a YSF VCH section to AMBE voice parameters
// This is a simplified conversion - full implementation would use Golay decoding
func (e *YSFAMBEExtractor) ConvertVCHToAMBE(vch *YSFVCHSection) (AMBEVoiceParams, error) {
	// Extract voice parameters from the 104-bit VCH data
	// This is a simplified mapping - real implementation would:
	// 1. Apply Golay error correction
	// 2. Extract fundamental frequency (F0)
	// 3. Extract voicing decision bits
	// 4. Extract spectral coefficients
	// 5. Extract gain parameters

	params := AMBEVoiceParams{}

	// Extract fundamental frequency and voicing (first 24 bits → A parameter)
	params.A = 0
	for i := 0; i < DMR_VOICE_BITS_A && i < YSF_VCH_BITS; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)

		if byteIndex < len(vch.Data) {
			if (vch.Data[byteIndex] & (1 << bitIndex)) != 0 {
				params.A |= (1 << (DMR_VOICE_BITS_A - 1 - i))
			}
		}
	}

	// Extract spectral coefficients (next 23 bits → B parameter)
	params.B = 0
	startBit := DMR_VOICE_BITS_A
	for i := 0; i < DMR_VOICE_BITS_B && startBit+i < YSF_VCH_BITS; i++ {
		bitPos := startBit + i
		byteIndex := bitPos / 8
		bitIndex := 7 - (bitPos % 8)

		if byteIndex < len(vch.Data) {
			if (vch.Data[byteIndex] & (1 << bitIndex)) != 0 {
				params.B |= (1 << (DMR_VOICE_BITS_B - 1 - i))
			}
		}
	}

	// Extract additional voice parameters (remaining bits → C parameter)
	params.C = 0
	startBit = DMR_VOICE_BITS_A + DMR_VOICE_BITS_B
	for i := 0; i < DMR_VOICE_BITS_C && startBit+i < YSF_VCH_BITS; i++ {
		bitPos := startBit + i
		byteIndex := bitPos / 8
		bitIndex := 7 - (bitPos % 8)

		if byteIndex < len(vch.Data) {
			if (vch.Data[byteIndex] & (1 << bitIndex)) != 0 {
				params.C |= (1 << (DMR_VOICE_BITS_C - 1 - i))
			}
		}
	}

	return params, nil
}

// ValidateVCHSection performs basic validation on a VCH section
func (e *YSFAMBEExtractor) ValidateVCHSection(vch *YSFVCHSection) bool {
	// Basic validation - check for all zeros or all ones (likely errors)
	allZeros := true
	allOnes := true

	for _, b := range vch.Data {
		if b != 0x00 {
			allZeros = false
		}
		if b != 0xFF {
			allOnes = false
		}
	}

	// Invalid if all zeros or all ones (indicates no valid voice data)
	return !allZeros && !allOnes
}

// GetVCHBitError calculates a simple bit error estimate for a VCH section
func (e *YSFAMBEExtractor) GetVCHBitError(vch *YSFVCHSection) float32 {
	// This is a simplified error estimation
	// Real implementation would use the triple-bit redundancy to calculate BER

	// Count transitions as a rough error indicator
	transitions := 0
	totalBits := len(vch.Data) * 8

	for i := 0; i < len(vch.Data); i++ {
		for j := 0; j < 7; j++ {
			bit1 := (vch.Data[i] & (1 << (7 - j))) != 0
			bit2 := (vch.Data[i] & (1 << (6 - j))) != 0
			if bit1 != bit2 {
				transitions++
			}
		}
	}

	// Return transition density as error estimate
	return float32(transitions) / float32(totalBits-1)
}