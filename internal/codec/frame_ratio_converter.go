package codec

import (
	"fmt"
	"time"
)

// FrameRatioConverter handles the 3:5 frame ratio conversion between YSF and DMR
// 3 YSF frames (15 VCH sections) → 5 DMR frames (10 AMBE parameters)
type FrameRatioConverter struct {
	// YSF to DMR conversion buffers
	ysfFrameBuffer    [YSF_TO_DMR_FRAME_RATIO][]YSFVCHSection // Buffer for 3 YSF frames
	ysfFrameCount     int                                      // Current count of buffered YSF frames
	ysfBufferComplete bool                                     // True when we have 3 complete YSF frames

	// DMR to YSF conversion buffers
	dmrFrameBuffer    [DMR_TO_YSF_FRAME_RATIO][]AMBEVoiceParams // Buffer for 5 DMR frames
	dmrFrameCount     int                                        // Current count of buffered DMR frames
	dmrBufferComplete bool                                       // True when we have 5 complete DMR frames

	// Extractors for AMBE processing
	ysfExtractor *YSFAMBEExtractor
	dmrExtractor *DMRAMBEExtractor

	// Timing tracking for frame rate conversion
	lastYSFTime time.Time
	lastDMRTime time.Time

	// Statistics
	ysfToDmrConversions uint64
	dmrToYsfConversions uint64
	conversionErrors    uint64
}

// NewFrameRatioConverter creates a new frame ratio converter
func NewFrameRatioConverter() *FrameRatioConverter {
	return &FrameRatioConverter{
		ysfExtractor: NewYSFAMBEExtractor(),
		dmrExtractor: NewDMRAMBEExtractor(),
		lastYSFTime:  time.Now(),
		lastDMRTime:  time.Now(),
	}
}

// ConvertYSFToDMR converts YSF frames to DMR frames using 3:5 ratio
// Buffers YSF frames until we have 3, then produces 5 DMR frames
func (c *FrameRatioConverter) ConvertYSFToDMR(ysfPayload []byte) ([][]byte, error) {
	// Extract VCH sections from this YSF frame
	vchSections, err := c.ysfExtractor.ExtractVCHSections(ysfPayload)
	if err != nil {
		c.conversionErrors++
		return nil, fmt.Errorf("failed to extract YSF VCH sections: %v", err)
	}

	// Add VCH sections to buffer
	c.ysfFrameBuffer[c.ysfFrameCount] = vchSections[:]
	c.ysfFrameCount++

	// Check if we have enough YSF frames for conversion
	if c.ysfFrameCount < YSF_TO_DMR_FRAME_RATIO {
		// Not enough frames yet, return empty
		return [][]byte{}, nil
	}

	// We have 3 YSF frames (15 VCH sections total), convert to 5 DMR frames
	dmrFrames, err := c.convertBufferedYSFToDMR()
	if err != nil {
		c.conversionErrors++
		return nil, fmt.Errorf("failed to convert buffered YSF frames: %v", err)
	}

	// Reset YSF buffer for next conversion cycle
	c.ysfFrameCount = 0
	c.ysfBufferComplete = false
	c.ysfToDmrConversions++
	c.lastYSFTime = time.Now()

	return dmrFrames, nil
}

// ConvertDMRToYSF converts DMR frames to YSF frames using 5:3 ratio
// Buffers DMR frames until we have 5, then produces 3 YSF frames
func (c *FrameRatioConverter) ConvertDMRToYSF(dmrPayload []byte) ([][]byte, error) {
	// Extract AMBE frames from this DMR payload
	ambeFrames, err := c.dmrExtractor.ExtractAMBEFrames(dmrPayload)
	if err != nil {
		c.conversionErrors++
		return nil, fmt.Errorf("failed to extract DMR AMBE frames: %v", err)
	}

	// Add AMBE parameters to buffer (2 parameters per DMR frame, but count as 1 DMR frame)
	if c.dmrFrameCount < DMR_TO_YSF_FRAME_RATIO {
		// Store both AMBE frames from this DMR payload
		params := make([]AMBEVoiceParams, DMR_AMBE_FRAMES)
		for i := 0; i < DMR_AMBE_FRAMES; i++ {
			params[i] = ambeFrames[i].Params
		}
		c.dmrFrameBuffer[c.dmrFrameCount] = params
		c.dmrFrameCount++
	}

	// Check if we have enough DMR frames for conversion
	if c.dmrFrameCount < DMR_TO_YSF_FRAME_RATIO {
		// Not enough frames yet, return empty
		return [][]byte{}, nil
	}

	// We have 5 DMR frames (10 AMBE parameters total), convert to 3 YSF frames
	ysfFrames, err := c.convertBufferedDMRToYSF()
	if err != nil {
		c.conversionErrors++
		return nil, fmt.Errorf("failed to convert buffered DMR frames: %v", err)
	}

	// Reset DMR buffer for next conversion cycle
	c.dmrFrameCount = 0
	c.dmrBufferComplete = false
	c.dmrToYsfConversions++
	c.lastDMRTime = time.Now()

	return ysfFrames, nil
}

// convertBufferedYSFToDMR converts 3 buffered YSF frames to 5 DMR frames
func (c *FrameRatioConverter) convertBufferedYSFToDMR() ([][]byte, error) {
	// We have 3 YSF frames × 5 VCH sections = 15 VCH sections total
	// Need to produce 5 DMR frames × 2 AMBE parameters = 10 AMBE parameters total

	// Extract all 15 VCH sections into AMBE voice parameters
	allVCHSections := make([]YSFVCHSection, 0, 15)
	for i := 0; i < YSF_TO_DMR_FRAME_RATIO; i++ {
		allVCHSections = append(allVCHSections, c.ysfFrameBuffer[i]...)
	}

	// Convert VCH sections to AMBE parameters with interpolation
	ambeParams := make([]AMBEVoiceParams, 10)
	for i := 0; i < 10; i++ {
		// Map from 15 VCH sections to 10 AMBE parameters with interpolation
		sourceIndex := (i * 15) / 10 // This gives us source indices 0,1,3,4,6,7,9,10,12,13

		if sourceIndex < len(allVCHSections) {
			// Convert VCH section to AMBE parameters
			params, err := c.ysfExtractor.ConvertVCHToAMBE(&allVCHSections[sourceIndex])
			if err != nil {
				return nil, fmt.Errorf("failed to convert VCH %d to AMBE: %v", sourceIndex, err)
			}
			ambeParams[i] = params

			// If we're not at the exact mapping, interpolate with next section
			nextIndex := sourceIndex + 1
			if nextIndex < len(allVCHSections) && (i*15)%10 != 0 {
				nextParams, err := c.ysfExtractor.ConvertVCHToAMBE(&allVCHSections[nextIndex])
				if err == nil {
					// Simple interpolation between parameters
					ambeParams[i] = c.interpolateAMBEParams(params, nextParams, 0.5)
				}
			}
		}
	}

	// Create 5 DMR frames from 10 AMBE parameters
	dmrFrames := make([][]byte, DMR_TO_YSF_FRAME_RATIO)
	for i := 0; i < DMR_TO_YSF_FRAME_RATIO; i++ {
		framePayload := make([]byte, DMR_FRAME_LENGTH)

		// Each DMR frame contains 2 AMBE parameters
		param1 := ambeParams[i*2]
		param2 := ambeParams[i*2+1]

		// Encode first AMBE parameter (frame 0 pattern: A+B)
		err := c.dmrExtractor.EncodeAMBEFrame(&param1, 0, framePayload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode AMBE frame %d.0: %v", i, err)
		}

		// Encode second AMBE parameter (frame 1 pattern: A+C)
		// For now, we overlay the second parameter - real implementation would multiplex properly
		tempPayload := make([]byte, DMR_FRAME_LENGTH)
		err = c.dmrExtractor.EncodeAMBEFrame(&param2, 1, tempPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode AMBE frame %d.1: %v", i, err)
		}

		// Combine the two encoded frames (simplified - real implementation would use proper multiplexing)
		for j := 0; j < DMR_FRAME_LENGTH; j++ {
			framePayload[j] = framePayload[j] ^ tempPayload[j] // Simple combination
		}

		dmrFrames[i] = framePayload
	}

	return dmrFrames, nil
}

// convertBufferedDMRToYSF converts 5 buffered DMR frames to 3 YSF frames
func (c *FrameRatioConverter) convertBufferedDMRToYSF() ([][]byte, error) {
	// We have 5 DMR frames × 2 AMBE parameters = 10 AMBE parameters total
	// Need to produce 3 YSF frames × 5 VCH sections = 15 VCH sections total

	// Extract all 10 AMBE parameters
	allAMBEParams := make([]AMBEVoiceParams, 0, 10)
	for i := 0; i < DMR_TO_YSF_FRAME_RATIO; i++ {
		allAMBEParams = append(allAMBEParams, c.dmrFrameBuffer[i]...)
	}

	// Convert AMBE parameters to VCH sections with interpolation
	vchSections := make([]YSFVCHSection, 15)
	for i := 0; i < 15; i++ {
		// Map from 10 AMBE parameters to 15 VCH sections with interpolation
		sourceIndex := (i * 10) / 15 // This gives us source indices distributed across 10 parameters

		if sourceIndex < len(allAMBEParams) {
			// Convert AMBE parameters to VCH section
			vch, err := c.dmrExtractor.ConvertAMBEToVCH(&allAMBEParams[sourceIndex])
			if err != nil {
				return nil, fmt.Errorf("failed to convert AMBE %d to VCH: %v", sourceIndex, err)
			}
			vchSections[i] = vch

			// If we're not at the exact mapping, interpolate with next parameter
			nextIndex := sourceIndex + 1
			if nextIndex < len(allAMBEParams) && (i*10)%15 != 0 {
				nextVCH, err := c.dmrExtractor.ConvertAMBEToVCH(&allAMBEParams[nextIndex])
				if err == nil {
					// Simple interpolation between VCH sections
					vchSections[i] = c.interpolateVCHSections(vch, nextVCH, 0.5)
				}
			}
		}
	}

	// Create 3 YSF frames from 15 VCH sections
	ysfFrames := make([][]byte, YSF_TO_DMR_FRAME_RATIO)
	for i := 0; i < YSF_TO_DMR_FRAME_RATIO; i++ {
		framePayload := make([]byte, YSF_PAYLOAD_LENGTH)

		// Each YSF frame contains 5 VCH sections
		frameSections := [YSF_VCH_SECTIONS]YSFVCHSection{}
		for j := 0; j < YSF_VCH_SECTIONS; j++ {
			frameSections[j] = vchSections[i*YSF_VCH_SECTIONS+j]
		}

		// Encode VCH sections into YSF payload
		err := c.encodeVCHSectionsToPayload(frameSections[:], framePayload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode YSF frame %d: %v", i, err)
		}

		ysfFrames[i] = framePayload
	}

	return ysfFrames, nil
}

// interpolateAMBEParams performs simple interpolation between two AMBE parameter sets
func (c *FrameRatioConverter) interpolateAMBEParams(params1, params2 AMBEVoiceParams, ratio float32) AMBEVoiceParams {
	// Simple linear interpolation between parameters
	// Real implementation would use proper voice parameter interpolation

	result := AMBEVoiceParams{}

	// Interpolate A parameter (fundamental frequency and voicing)
	result.A = uint32(float32(params1.A)*(1.0-ratio) + float32(params2.A)*ratio)

	// Interpolate B parameter (spectral coefficients)
	result.B = uint32(float32(params1.B)*(1.0-ratio) + float32(params2.B)*ratio)

	// Interpolate C parameter (additional voice parameters)
	result.C = uint32(float32(params1.C)*(1.0-ratio) + float32(params2.C)*ratio)

	return result
}

// interpolateVCHSections performs simple interpolation between two VCH sections
func (c *FrameRatioConverter) interpolateVCHSections(vch1, vch2 YSFVCHSection, ratio float32) YSFVCHSection {
	// Simple byte-wise interpolation
	// Real implementation would use proper voice parameter interpolation

	result := YSFVCHSection{}

	for i := 0; i < len(result.Data); i++ {
		val1 := float32(vch1.Data[i])
		val2 := float32(vch2.Data[i])
		result.Data[i] = uint8(val1*(1.0-ratio) + val2*ratio)
	}

	return result
}

// encodeVCHSectionsToPayload encodes VCH sections back into YSF payload format
func (c *FrameRatioConverter) encodeVCHSectionsToPayload(vchSections []YSFVCHSection, payload []byte) error {
	if len(payload) < YSF_PAYLOAD_LENGTH {
		return fmt.Errorf("payload buffer too small: got %d, need %d", len(payload), YSF_PAYLOAD_LENGTH)
	}

	// Clear payload
	for i := range payload {
		payload[i] = 0
	}

	// Encode each VCH section with proper interleaving and whitening
	for sectionIndex, vch := range vchSections {
		if sectionIndex >= YSF_VCH_SECTIONS {
			break // Don't exceed expected sections
		}

		// Convert VCH data to voice bits
		voiceBits := make([]bool, YSF_VCH_BITS)
		for i := 0; i < YSF_VCH_BITS; i++ {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			if byteIndex < len(vch.Data) {
				voiceBits[i] = (vch.Data[byteIndex] & (1 << bitIndex)) != 0
			}
		}

		// Apply whitening
		whitenedBits := make([]bool, YSF_VCH_BITS*3)
		c.applyWhitening(voiceBits, whitenedBits, sectionIndex)

		// Apply interleaving
		interleavedBits := make([]bool, YSF_VCH_BITS*3)
		c.applyInterleaving(whitenedBits, interleavedBits)

		// Pack interleaved bits into payload
		startBitPos := sectionIndex * YSF_VCH_BITS * 3
		for i := 0; i < len(interleavedBits); i++ {
			bitPos := startBitPos + i
			if bitPos >= len(payload)*8 {
				break
			}

			bytePos := bitPos / 8
			bitOffset := bitPos % 8

			if bytePos < len(payload) && interleavedBits[i] {
				payload[bytePos] |= (1 << (7 - bitOffset))
			}
		}
	}

	return nil
}

// applyWhitening applies whitening/scrambling to voice bits (reverse of removeWhitening)
func (c *FrameRatioConverter) applyWhitening(voiceBits, whitenedBits []bool, sectionIndex int) {
	// Create triple-bit redundancy first
	for i := 0; i < len(voiceBits) && i*3+2 < len(whitenedBits); i++ {
		// Each voice bit becomes 3 identical bits
		whitenedBits[i*3] = voiceBits[i]
		whitenedBits[i*3+1] = voiceBits[i]
		whitenedBits[i*3+2] = voiceBits[i]
	}

	// Apply whitening pattern
	for i := 0; i < len(whitenedBits); i++ {
		whiteningByteIndex := (i + sectionIndex*YSF_VCH_BITS*3/8) % WHITENING_DATA_SIZE
		whiteningBitIndex := i % 8

		whiteningBit := (WHITENING_DATA[whiteningByteIndex] & (1 << (7 - whiteningBitIndex))) != 0

		// XOR with whitening bit to apply scrambling
		whitenedBits[i] = whitenedBits[i] != whiteningBit
	}
}

// applyInterleaving applies interleaving to voice bits (reverse of deinterleave)
func (c *FrameRatioConverter) applyInterleaving(src, dest []bool) {
	if len(src) != len(dest) || len(src) < YSF_VCH_BITS*3 {
		return
	}

	// Clear destination
	for i := range dest {
		dest[i] = false
	}

	// Apply interleaving using the interleave table
	for i := 0; i < YSF_VCH_BITS*3 && i < len(INTERLEAVE_TABLE_26_4); i++ {
		destIndex := INTERLEAVE_TABLE_26_4[i]
		if int(destIndex) < len(dest) {
			dest[destIndex] = src[i]
		}
	}
}

// GetConversionStats returns conversion statistics
func (c *FrameRatioConverter) GetConversionStats() (uint64, uint64, uint64) {
	return c.ysfToDmrConversions, c.dmrToYsfConversions, c.conversionErrors
}

// Reset clears all buffers and resets the converter state
func (c *FrameRatioConverter) Reset() {
	c.ysfFrameCount = 0
	c.ysfBufferComplete = false
	c.dmrFrameCount = 0
	c.dmrBufferComplete = false

	// Clear buffers
	for i := range c.ysfFrameBuffer {
		c.ysfFrameBuffer[i] = nil
	}
	for i := range c.dmrFrameBuffer {
		c.dmrFrameBuffer[i] = nil
	}
}

// IsYSFBufferReady returns true if we have enough YSF frames for conversion
func (c *FrameRatioConverter) IsYSFBufferReady() bool {
	return c.ysfFrameCount >= YSF_TO_DMR_FRAME_RATIO
}

// IsDMRBufferReady returns true if we have enough DMR frames for conversion
func (c *FrameRatioConverter) IsDMRBufferReady() bool {
	return c.dmrFrameCount >= DMR_TO_YSF_FRAME_RATIO
}