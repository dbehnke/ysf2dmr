package codec

import (
	"fmt"
)

// AMBE format types
type AMBEFormat int

const (
	AMBE_YSF AMBEFormat = iota // YSF AMBE format (54 bits per frame)
	AMBE_DMR                   // DMR AMBE format (108 bits per frame)
)

// AMBE frame sizes in bits
const (
	YSF_AMBE_FRAME_BITS = 54  // YSF AMBE frame size in bits
	DMR_AMBE_FRAME_BITS = 108 // DMR AMBE frame size in bits

	// Packed sizes in bytes (rounded up)
	YSF_AMBE_FRAME_BYTES = 7  // (54 + 7) / 8 = 7 bytes
	DMR_AMBE_FRAME_BYTES = 14 // (108 + 7) / 8 = 14 bytes

	// YSF contains 3 AMBE frames per payload
	YSF_AMBE_FRAMES_PER_PAYLOAD = 3
	YSF_TOTAL_AMBE_BITS = YSF_AMBE_FRAME_BITS * YSF_AMBE_FRAMES_PER_PAYLOAD // 162 bits
)

// AMBEConverter handles conversion between YSF and DMR AMBE formats
type AMBEConverter struct {
	dmrBuffer [][]byte // Buffer for DMR frames awaiting pairing
}

// NewAMBEConverter creates a new AMBE converter
func NewAMBEConverter() *AMBEConverter {
	return &AMBEConverter{
		dmrBuffer: make([][]byte, 0),
	}
}

// YSFToDMR converts a YSF AMBE frame (162 bits) to 2 DMR AMBE frames (108 bits each)
func (c *AMBEConverter) YSFToDMR(ysfFrame []byte) ([][]byte, error) {
	if len(ysfFrame) < YSF_AMBE_FRAME_BYTES*3 {
		return nil, fmt.Errorf("YSF frame too short: got %d bytes, need %d", len(ysfFrame), YSF_AMBE_FRAME_BYTES*3)
	}

	// Extract 3 YSF AMBE frames (54 bits each = 162 bits total)
	ysfFrames, err := c.ExtractYSFAudio(ysfFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to extract YSF audio: %v", err)
	}

	if len(ysfFrames) != 3 {
		return nil, fmt.Errorf("expected 3 YSF AMBE frames, got %d", len(ysfFrames))
	}

	// Convert each YSF frame to DMR format
	// YSF: 3 frames of 54 bits each = 162 bits
	// DMR: 2 frames of 108 bits each = 216 bits (with padding)
	dmrFrames := make([][]byte, 2)

	// First DMR frame: Combine first 1.5 YSF frames
	dmrFrames[0] = make([]byte, DMR_AMBE_FRAME_BYTES)
	c.convertYSFToDMRFrame(ysfFrames[0], ysfFrames[1], dmrFrames[0], true)

	// Second DMR frame: Combine remaining 1.5 YSF frames
	dmrFrames[1] = make([]byte, DMR_AMBE_FRAME_BYTES)
	c.convertYSFToDMRFrame(ysfFrames[1], ysfFrames[2], dmrFrames[1], false)

	return dmrFrames, nil
}

// DMRToYSF converts 2 DMR AMBE frames (108 bits each) to 1 YSF AMBE frame (162 bits)
func (c *AMBEConverter) DMRToYSF(dmrFrame1, dmrFrame2 []byte) ([]byte, error) {
	if dmrFrame1 != nil && len(dmrFrame1) < DMR_AMBE_FRAME_BYTES {
		return nil, fmt.Errorf("DMR frame 1 too short: got %d bytes, need %d", len(dmrFrame1), DMR_AMBE_FRAME_BYTES)
	}
	if dmrFrame2 != nil && len(dmrFrame2) < DMR_AMBE_FRAME_BYTES {
		return nil, fmt.Errorf("DMR frame 2 too short: got %d bytes, need %d", len(dmrFrame2), DMR_AMBE_FRAME_BYTES)
	}

	// If we only have one frame, buffer it for later
	if dmrFrame2 == nil {
		if dmrFrame1 != nil {
			buffered := make([]byte, len(dmrFrame1))
			copy(buffered, dmrFrame1)
			c.dmrBuffer = append(c.dmrBuffer, buffered)
		}
		return nil, nil // Wait for pair
	}

	// We have a pair, convert to YSF
	ysfFrame := make([]byte, YSF_AMBE_FRAME_BYTES*3) // 3 YSF frames

	// Convert DMR frames back to YSF format
	// This is the reverse of YSFToDMR conversion
	err := c.convertDMRToYSFFrames(dmrFrame1, dmrFrame2, ysfFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to convert DMR to YSF: %v", err)
	}

	return ysfFrame, nil
}

// ExtractYSFAudio extracts 3 AMBE frames from YSF payload
func (c *AMBEConverter) ExtractYSFAudio(ysfPayload []byte) ([][]byte, error) {
	if len(ysfPayload) < 90 {
		return nil, fmt.Errorf("YSF payload too short: got %d bytes, need at least 90", len(ysfPayload))
	}

	// YSF payload structure:
	// - 3 AMBE frames of 54 bits each
	// - Total of 162 bits = 21 bytes (rounded up)
	// - Frames are packed in the first part of the payload

	frames := make([][]byte, 3)

	// Extract each 54-bit AMBE frame
	for i := 0; i < 3; i++ {
		frames[i] = make([]byte, YSF_AMBE_FRAME_BYTES)

		// Calculate bit offset for this frame
		bitOffset := i * YSF_AMBE_FRAME_BITS

		// Extract bits and pack into bytes
		err := c.extractBits(ysfPayload, bitOffset, YSF_AMBE_FRAME_BITS, frames[i])
		if err != nil {
			return nil, fmt.Errorf("failed to extract YSF AMBE frame %d: %v", i, err)
		}
	}

	return frames, nil
}

// ExtractDMRAudio extracts AMBE frame from DMR payload
func (c *AMBEConverter) ExtractDMRAudio(dmrPayload []byte) ([]byte, error) {
	if len(dmrPayload) < 23 {
		return nil, fmt.Errorf("DMR payload too short: got %d bytes, need at least 23", len(dmrPayload))
	}

	// DMR payload structure:
	// - 1 AMBE frame of 108 bits
	// - Frame is packed in the first part of the payload

	frame := make([]byte, DMR_AMBE_FRAME_BYTES)

	// Extract 108-bit AMBE frame
	err := c.extractBits(dmrPayload, 0, DMR_AMBE_FRAME_BITS, frame)
	if err != nil {
		return nil, fmt.Errorf("failed to extract DMR AMBE frame: %v", err)
	}

	return frame, nil
}

// convertYSFToDMRFrame converts YSF AMBE frames to DMR format
func (c *AMBEConverter) convertYSFToDMRFrame(ysf1, ysf2 []byte, dmrOut []byte, firstHalf bool) {
	// This is a simplified conversion - real implementation would:
	// 1. Decode YSF AMBE frames using Golay error correction
	// 2. Extract voice parameters (pitch, gain, spectral coefficients)
	// 3. Re-encode in DMR AMBE format
	// 4. Apply appropriate error correction

	// For now, we'll do a simplified bit-wise conversion
	if firstHalf {
		// First DMR frame gets YSF frame 1 + half of frame 2
		copy(dmrOut[:YSF_AMBE_FRAME_BYTES], ysf1)
		copy(dmrOut[YSF_AMBE_FRAME_BYTES:], ysf2[:DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES])
	} else {
		// Second DMR frame gets half of frame 2 + frame 3
		copy(dmrOut[:DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES], ysf1[DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES:])
		copy(dmrOut[DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES:], ysf2)
	}
}

// convertDMRToYSFFrames converts DMR AMBE frames back to YSF format
func (c *AMBEConverter) convertDMRToYSFFrames(dmr1, dmr2, ysfOut []byte) error {
	// This is the reverse of convertYSFToDMRFrame
	// Real implementation would decode DMR AMBE and re-encode as YSF

	// Extract YSF frame 1 from first part of DMR frame 1
	copy(ysfOut[:YSF_AMBE_FRAME_BYTES], dmr1[:YSF_AMBE_FRAME_BYTES])

	// Extract YSF frame 2 from remainder of DMR frame 1 + start of DMR frame 2
	ysf2Start := YSF_AMBE_FRAME_BYTES
	copy(ysfOut[ysf2Start:ysf2Start+DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES],
		 dmr1[YSF_AMBE_FRAME_BYTES:])
	copy(ysfOut[ysf2Start+DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES:ysf2Start+YSF_AMBE_FRAME_BYTES],
		 dmr2[:YSF_AMBE_FRAME_BYTES-(DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES)])

	// Extract YSF frame 3 from remainder of DMR frame 2
	ysf3Start := YSF_AMBE_FRAME_BYTES * 2
	copy(ysfOut[ysf3Start:], dmr2[DMR_AMBE_FRAME_BYTES-YSF_AMBE_FRAME_BYTES:])

	return nil
}

// extractBits extracts a specified number of bits from source and packs them into dest
func (c *AMBEConverter) extractBits(src []byte, bitOffset, numBits int, dest []byte) error {
	if bitOffset < 0 || numBits <= 0 {
		return fmt.Errorf("invalid bit extraction parameters")
	}

	if (bitOffset+numBits+7)/8 > len(src) {
		return fmt.Errorf("not enough source data for bit extraction")
	}

	// Clear destination
	for i := range dest {
		dest[i] = 0
	}

	// Extract bits
	for i := 0; i < numBits; i++ {
		srcByteIdx := (bitOffset + i) / 8
		srcBitIdx := (bitOffset + i) % 8

		destByteIdx := i / 8
		destBitIdx := 7 - (i % 8) // MSB first

		if destByteIdx >= len(dest) {
			break
		}

		if srcByteIdx < len(src) {
			srcBit := (src[srcByteIdx] >> uint(7-srcBitIdx)) & 0x01
			dest[destByteIdx] |= srcBit << uint(destBitIdx)
		}
	}

	return nil
}

// ValidateAMBEFrame validates that a frame is the correct size for the given format
func ValidateAMBEFrame(frame []byte, format AMBEFormat) bool {
	switch format {
	case AMBE_YSF:
		return len(frame) == YSF_AMBE_FRAME_BYTES
	case AMBE_DMR:
		return len(frame) == DMR_AMBE_FRAME_BYTES
	default:
		return false
	}
}