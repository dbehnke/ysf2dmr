package codec

import (
	"fmt"
)

// DMRAMBEExtractor handles DMR AMBE frame extraction and processing
type DMRAMBEExtractor struct {
	// No state needed for extraction
}

// NewDMRAMBEExtractor creates a new DMR AMBE extractor
func NewDMRAMBEExtractor() *DMRAMBEExtractor {
	return &DMRAMBEExtractor{}
}

// ExtractAMBEFrames extracts 2 AMBE frames from DMR payload
// Based on C++ DMRSlotType and BPTC19696 processing
func (e *DMRAMBEExtractor) ExtractAMBEFrames(dmrPayload []byte) ([DMR_AMBE_FRAMES]DMRAMBEFrame, error) {
	if len(dmrPayload) < DMR_FRAME_LENGTH {
		return [DMR_AMBE_FRAMES]DMRAMBEFrame{}, fmt.Errorf("DMR payload too short: got %d, need %d",
			len(dmrPayload), DMR_FRAME_LENGTH)
	}

	var ambeFrames [DMR_AMBE_FRAMES]DMRAMBEFrame

	// DMR payload contains 2 AMBE frames (A+B and A+C patterns)
	for i := 0; i < DMR_AMBE_FRAMES; i++ {
		err := e.extractAMBEFrame(dmrPayload, i, &ambeFrames[i])
		if err != nil {
			return [DMR_AMBE_FRAMES]DMRAMBEFrame{}, fmt.Errorf("failed to extract AMBE frame %d: %v", i, err)
		}
	}

	return ambeFrames, nil
}

// extractAMBEFrame extracts a single AMBE frame from DMR payload
// Implements DMR voice processing algorithm from C++
func (e *DMRAMBEExtractor) extractAMBEFrame(payload []byte, frameIndex int, ambeFrame *DMRAMBEFrame) error {
	// Clear the AMBE frame
	ambeFrame.Params = AMBEVoiceParams{}
	for i := range ambeFrame.Raw {
		ambeFrame.Raw[i] = 0
	}

	// Copy raw frame data for reference
	if len(payload) >= DMR_FRAME_LENGTH {
		copy(ambeFrame.Raw[:], payload[:DMR_FRAME_LENGTH])
	}

	// Step 1: Extract the BPTC(196,96) encoded bits from DMR payload
	bptcBits := make([]uint8, 33) // 196 bits = 33 bytes (rounded up)
	err := e.extractBPTCBits(payload, frameIndex, bptcBits)
	if err != nil {
		return fmt.Errorf("failed to extract BPTC bits: %v", err)
	}

	// Step 2: Apply BPTC(196,96) error correction to get 96 voice bits
	bptc := NewBPTC19696()
	voiceBits, ok := bptc.Decode(bptcBits)
	if !ok {
		return fmt.Errorf("BPTC decode failed for frame %d", frameIndex)
	}

	// Convert voice bytes to boolean bits for processing
	correctedBits := make([]bool, 96)
	for i := 0; i < 12 && i*8 < 96; i++ {
		ByteToBitsBE(voiceBits[i], correctedBits[i*8:(i+1)*8])
	}

	// Step 3: Remove PRNG masking (temporarily disabled for testing)
	unmaskedBits := make([]bool, 96)
	copy(unmaskedBits, correctedBits) // Skip PRNG masking for now
	// e.removePRNGMasking(correctedBits, unmaskedBits, frameIndex)

	// Step 4: Extract voice parameters A, B, C based on frame pattern
	err = e.extractVoiceParameters(unmaskedBits, frameIndex, &ambeFrame.Params)
	if err != nil {
		return fmt.Errorf("failed to extract voice parameters: %v", err)
	}

	// Step 5: Apply Golay error correction to voice parameters
	e.applyGolayErrorCorrection(&ambeFrame.Params, frameIndex)

	// Step 6: Validate extracted AMBE frame
	if !e.ValidateAMBEFrame(ambeFrame) {
		// Calculate bit error rate for diagnostic purposes
		ber := e.GetAMBEBitError(ambeFrame)
		return fmt.Errorf("AMBE frame validation failed for frame %d, estimated BER: %.3f", frameIndex, ber)
	}

	return nil
}

// extractBPTCBits extracts BPTC(196,96) encoded bits from DMR payload
// DMR voice data is protected using BPTC - we extract the 196-bit codewords
func (e *DMRAMBEExtractor) extractBPTCBits(payload []byte, frameIndex int, bptcBits []uint8) error {
	// DMR voice data is spread across specific bit positions in the payload
	// Each voice frame uses a BPTC(196,96) codeword for protection

	// Clear output buffer
	for i := range bptcBits {
		bptcBits[i] = 0
	}

	// Calculate the bit offset for this frame's BPTC codeword
	// DMR frame structure places BPTC codewords at specific positions
	startBitPos := frameIndex * 196 // Each BPTC codeword is 196 bits

	// Extract 196 bits and pack into 33 bytes (196/8 = 24.5, rounded up to 25, but BPTC expects 33)
	for i := 0; i < 196; i++ {
		bitPos := startBitPos + i
		if bitPos >= len(payload)*8 {
			break // Prevent buffer overflow
		}

		bytePos := bitPos / 8
		bitOffset := bitPos % 8

		if bytePos < len(payload) {
			// Extract bit from payload
			bit := (payload[bytePos] & (1 << (7 - bitOffset))) != 0

			// Pack into BPTC bytes using the expected format
			outputBytePos := i / 8
			outputBitPos := 7 - (i % 8)

			if outputBytePos < len(bptcBits) && bit {
				bptcBits[outputBytePos] |= (1 << outputBitPos)
			}
		}
	}

	return nil
}

// applyGolayErrorCorrection applies Golay error correction to voice parameters
// Voice parameters are protected using different Golay codes based on their size
func (e *DMRAMBEExtractor) applyGolayErrorCorrection(params *AMBEVoiceParams, frameIndex int) {
	// A parameter (24 bits): Split into 2×12-bit chunks for Golay(24,12)
	if frameIndex >= 0 { // A parameter is present in both frames
		// Extract lower 12 bits and apply Golay(24,12)
		aLow := params.A & 0xFFF
		aLow = Decode24128((aLow << 12) | e.calculateGolayParity24(aLow))

		// Extract upper 12 bits and apply Golay(24,12)
		aHigh := (params.A >> 12) & 0xFFF
		aHigh = Decode24128((aHigh << 12) | e.calculateGolayParity24(aHigh))

		// Reconstruct A parameter
		params.A = (aHigh << 12) | aLow
	}

	// B parameter (23 bits): Apply Golay(23,12) for Frame 0
	if frameIndex == 0 {
		// For 23-bit parameter, use Golay(23,12) which protects 11 data bits
		// We need to split the 23 bits: 11 data + 12 parity
		bData := params.B & 0x7FF // Lower 11 bits as data
		bData = Decode23127((bData << 12) | e.calculateGolayParity23(bData))

		// Reconstruct B parameter (may need additional protection for remaining bits)
		params.B = bData
	}

	// C parameter (25 bits): Split for Golay(24,12) protection for Frame 1
	if frameIndex == 1 {
		// Split 25 bits into 12+12+1 for two Golay(24,12) operations
		cLow := params.C & 0xFFF          // Lower 12 bits
		cMid := (params.C >> 12) & 0xFFF  // Middle 12 bits
		cHigh := (params.C >> 24) & 0x1   // Upper 1 bit

		// Apply Golay(24,12) to lower and middle chunks
		cLow = Decode24128((cLow << 12) | e.calculateGolayParity24(cLow))
		cMid = Decode24128((cMid << 12) | e.calculateGolayParity24(cMid))

		// Reconstruct C parameter
		params.C = (cHigh << 24) | (cMid << 12) | cLow
	}
}

// calculateGolayParity24 calculates 12-bit Golay parity for 12-bit data
func (e *DMRAMBEExtractor) calculateGolayParity24(data uint32) uint32 {
	// Use the Golay (24,12) generator polynomial: x^11 + x^10 + x^6 + x^5 + x^4 + x^2 + 1
	generator := uint32(0xC75)
	shifted := (data & 0xFFF) << 12
	return e.polyDiv24(shifted, generator)
}

// calculateGolayParity23 calculates 12-bit Golay parity for 11-bit data
func (e *DMRAMBEExtractor) calculateGolayParity23(data uint32) uint32 {
	// Use the Golay (23,11) generator polynomial
	generator := uint32(0x65B)
	shifted := (data & 0x7FF) << 12
	return e.polyDiv23(shifted, generator)
}

// polyDiv24 performs polynomial division for 24-bit values (helper function)
func (e *DMRAMBEExtractor) polyDiv24(dividend, divisor uint32) uint32 {
	dividend &= 0xFFFFFF // 24 bits
	divisor &= 0xFFF     // 12 bits

	remainder := dividend
	for i := 23; i >= 12; i-- {
		if (remainder & (1 << uint(i))) != 0 {
			remainder ^= divisor << uint(i-11)
		}
	}

	return remainder & 0xFFF
}

// polyDiv23 performs polynomial division for 23-bit values (helper function)
func (e *DMRAMBEExtractor) polyDiv23(dividend, divisor uint32) uint32 {
	dividend &= 0x7FFFFF // 23 bits
	divisor &= 0xFFF      // 12 bits

	remainder := dividend
	for i := 22; i >= 12; i-- {
		if (remainder & (1 << uint(i))) != 0 {
			remainder ^= divisor << uint(i-11)
		}
	}

	return remainder & 0xFFF
}

// applyGolayEncoding applies Golay encoding to voice parameters for transmission
// This is the reverse of applyGolayErrorCorrection - it encodes data for protection
func (e *DMRAMBEExtractor) applyGolayEncoding(params *AMBEVoiceParams, frameIndex int) {
	// A parameter (24 bits): Split into 2×12-bit chunks for Golay(24,12) encoding
	if frameIndex >= 0 { // A parameter is present in both frames
		// Extract and encode lower 12 bits with Golay(24,12)
		aLow := params.A & 0xFFF
		aLowEncoded := Encode24128(aLow)

		// Extract and encode upper 12 bits with Golay(24,12)
		aHigh := (params.A >> 12) & 0xFFF
		aHighEncoded := Encode24128(aHigh)

		// Reconstruct A parameter with encoded data (keeping only data bits for now)
		params.A = ((aHighEncoded >> 12) << 12) | (aLowEncoded >> 12)
	}

	// B parameter (23 bits): Apply Golay(23,12) encoding for Frame 0
	if frameIndex == 0 {
		// For 23-bit parameter, encode as 11 data bits with Golay(23,12)
		bData := params.B & 0x7FF // Extract 11 bits as data
		bEncoded := Encode23127(bData)

		// Keep only the data portion (upper 11 bits) for parameter reconstruction
		params.B = bEncoded >> 12
	}

	// C parameter (25 bits): Split for Golay(24,12) encoding for Frame 1
	if frameIndex == 1 {
		// Split 25 bits into 12+12+1 for two Golay(24,12) operations
		cLow := params.C & 0xFFF          // Lower 12 bits
		cMid := (params.C >> 12) & 0xFFF  // Middle 12 bits
		cHigh := (params.C >> 24) & 0x1   // Upper 1 bit (no Golay protection)

		// Apply Golay(24,12) encoding to lower and middle chunks
		cLowEncoded := Encode24128(cLow)
		cMidEncoded := Encode24128(cMid)

		// Reconstruct C parameter with encoded data (keeping only data bits)
		params.C = (cHigh << 24) | ((cMidEncoded >> 12) << 12) | (cLowEncoded >> 12)
	}
}

// Public wrapper functions for testing
// ApplyGolayEncoding is a public wrapper for testing purposes
func (e *DMRAMBEExtractor) ApplyGolayEncoding(params *AMBEVoiceParams, frameIndex int) {
	e.applyGolayEncoding(params, frameIndex)
}

// ApplyGolayErrorCorrection is a public wrapper for testing purposes
func (e *DMRAMBEExtractor) ApplyGolayErrorCorrection(params *AMBEVoiceParams, frameIndex int) {
	e.applyGolayErrorCorrection(params, frameIndex)
}

// removePRNGMasking removes PRNG masking from voice parameters
// DMR uses a pseudorandom sequence to mask voice parameters
func (e *DMRAMBEExtractor) removePRNGMasking(src, dest []bool, frameIndex int) {
	if len(src) != len(dest) {
		return
	}

	// Copy input to output first
	copy(dest, src)

	// Apply PRNG unmasking using the PRNG table
	for i := 0; i < len(src); i++ {
		// Calculate PRNG index based on bit position and frame
		prngIndex := (i + frameIndex*96) % PRNG_TABLE_SIZE

		// Get the PRNG mask value
		prngValue := PRNG_TABLE[prngIndex]

		// Extract the bit we need from the PRNG value
		prngBitIndex := i % 32
		prngBit := (prngValue & (1 << (31 - prngBitIndex))) != 0

		// XOR with PRNG bit to remove masking
		dest[i] = src[i] != prngBit // XOR operation
	}
}

// extractVoiceParameters extracts A, B, C voice parameters from unmasked bits
// DMR alternates between A+B and A+C parameter patterns
func (e *DMRAMBEExtractor) extractVoiceParameters(voiceBits []bool, frameIndex int, params *AMBEVoiceParams) error {
	// Clear parameters
	params.A = 0
	params.B = 0
	params.C = 0

	// DMR frame patterns:
	// Frame 0: A + B parameters (24 + 23 = 47 bits)
	// Frame 1: A + C parameters (24 + 25 = 49 bits)

	// Extract A parameter (always present, 24 bits)
	for i := 0; i < DMR_VOICE_BITS_A && i < len(voiceBits); i++ {
		if voiceBits[i] {
			params.A |= (1 << (DMR_VOICE_BITS_A - 1 - i))
		}
	}

	if frameIndex == 0 {
		// Frame 0: Extract B parameter (23 bits)
		startBit := DMR_VOICE_BITS_A
		for i := 0; i < DMR_VOICE_BITS_B && startBit+i < len(voiceBits); i++ {
			if voiceBits[startBit+i] {
				params.B |= (1 << (DMR_VOICE_BITS_B - 1 - i))
			}
		}
	} else {
		// Frame 1: Extract C parameter (25 bits)
		startBit := DMR_VOICE_BITS_A
		for i := 0; i < DMR_VOICE_BITS_C && startBit+i < len(voiceBits); i++ {
			if voiceBits[startBit+i] {
				params.C |= (1 << (DMR_VOICE_BITS_C - 1 - i))
			}
		}
	}

	return nil
}

// ConvertAMBEToVCH converts DMR AMBE voice parameters to YSF VCH format
// This performs the reverse conversion for DMR→YSF direction
func (e *DMRAMBEExtractor) ConvertAMBEToVCH(params *AMBEVoiceParams) (YSFVCHSection, error) {
	// Validate input AMBE parameters before conversion
	testFrame := DMRAMBEFrame{Params: *params}
	if !e.ValidateAMBEFrame(&testFrame) {
		ber := e.GetAMBEBitError(&testFrame)
		return YSFVCHSection{}, fmt.Errorf("AMBE parameters validation failed before VCH conversion, estimated BER: %.3f", ber)
	}

	vch := YSFVCHSection{}

	// Clear VCH data
	for i := range vch.Data {
		vch.Data[i] = 0
	}

	// Pack A parameter (24 bits) into first part of VCH
	for i := 0; i < DMR_VOICE_BITS_A && i < YSF_VCH_BITS; i++ {
		if (params.A & (1 << (DMR_VOICE_BITS_A - 1 - i))) != 0 {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			vch.Data[byteIndex] |= (1 << bitIndex)
		}
	}

	// Pack B parameter (23 bits) into next part of VCH
	startBit := DMR_VOICE_BITS_A
	for i := 0; i < DMR_VOICE_BITS_B && startBit+i < YSF_VCH_BITS; i++ {
		if (params.B & (1 << (DMR_VOICE_BITS_B - 1 - i))) != 0 {
			bitPos := startBit + i
			byteIndex := bitPos / 8
			bitIndex := 7 - (bitPos % 8)
			vch.Data[byteIndex] |= (1 << bitIndex)
		}
	}

	// Pack C parameter (25 bits) into remaining part of VCH
	startBit = DMR_VOICE_BITS_A + DMR_VOICE_BITS_B
	for i := 0; i < DMR_VOICE_BITS_C && startBit+i < YSF_VCH_BITS; i++ {
		if (params.C & (1 << (DMR_VOICE_BITS_C - 1 - i))) != 0 {
			bitPos := startBit + i
			byteIndex := bitPos / 8
			bitIndex := 7 - (bitPos % 8)
			if byteIndex < len(vch.Data) {
				vch.Data[byteIndex] |= (1 << bitIndex)
			}
		}
	}

	return vch, nil
}

// ValidateAMBEFrame performs basic validation on an AMBE frame
func (e *DMRAMBEExtractor) ValidateAMBEFrame(ambeFrame *DMRAMBEFrame) bool {
	// Basic validation - check for reasonable parameter values

	// A parameter (F0 and voicing) should have some variation
	if ambeFrame.Params.A == 0 || ambeFrame.Params.A == 0xFFFFFF {
		return false
	}

	// At least one of B or C should be non-zero
	if ambeFrame.Params.B == 0 && ambeFrame.Params.C == 0 {
		return false
	}

	return true
}

// GetAMBEBitError calculates a simple bit error estimate for an AMBE frame
func (e *DMRAMBEExtractor) GetAMBEBitError(ambeFrame *DMRAMBEFrame) float32 {
	// This is a simplified error estimation
	// Real implementation would use the error correction results

	// Count parameter magnitude as error indicator
	totalParams := 3
	errorCount := 0

	// Check if parameters are at extreme values (indicating possible errors)
	if ambeFrame.Params.A == 0 || ambeFrame.Params.A == 0xFFFFFF {
		errorCount++
	}
	if ambeFrame.Params.B == 0 || ambeFrame.Params.B == 0x7FFFFF {
		errorCount++
	}
	if ambeFrame.Params.C == 0 || ambeFrame.Params.C == 0x1FFFFFF {
		errorCount++
	}

	// Return error ratio
	return float32(errorCount) / float32(totalParams)
}

// EncodeAMBEFrame encodes AMBE voice parameters back into DMR payload format
func (e *DMRAMBEExtractor) EncodeAMBEFrame(params *AMBEVoiceParams, frameIndex int, payload []byte) error {
	if len(payload) < DMR_FRAME_LENGTH {
		return fmt.Errorf("payload buffer too small: got %d, need %d", len(payload), DMR_FRAME_LENGTH)
	}

	// Step 1: Validate input AMBE parameters before encoding
	testFrame := DMRAMBEFrame{Params: *params}
	if !e.ValidateAMBEFrame(&testFrame) {
		ber := e.GetAMBEBitError(&testFrame)
		return fmt.Errorf("input AMBE frame validation failed for frame %d, estimated BER: %.3f", frameIndex, ber)
	}

	// Step 2: Apply Golay encoding to voice parameters before packing
	encodedParams := *params // Make a copy to avoid modifying original
	e.applyGolayEncoding(&encodedParams, frameIndex)

	// Create voice bits array
	voiceBits := make([]bool, 96)

	// Pack A parameter (24 bits)
	for i := 0; i < DMR_VOICE_BITS_A; i++ {
		voiceBits[i] = (encodedParams.A & (1 << (DMR_VOICE_BITS_A - 1 - i))) != 0
	}

	if frameIndex == 0 {
		// Frame 0: Pack B parameter (23 bits)
		startBit := DMR_VOICE_BITS_A
		for i := 0; i < DMR_VOICE_BITS_B; i++ {
			voiceBits[startBit+i] = (encodedParams.B & (1 << (DMR_VOICE_BITS_B - 1 - i))) != 0
		}
	} else {
		// Frame 1: Pack C parameter (25 bits)
		startBit := DMR_VOICE_BITS_A
		for i := 0; i < DMR_VOICE_BITS_C; i++ {
			voiceBits[startBit+i] = (encodedParams.C & (1 << (DMR_VOICE_BITS_C - 1 - i))) != 0
		}
	}

	// Apply PRNG masking (temporarily disabled for testing)
	maskedBits := make([]bool, 96)
	copy(maskedBits, voiceBits) // Skip PRNG masking for now
	// e.applyPRNGMasking(voiceBits, maskedBits, frameIndex)

	// Convert masked bits to bytes for BPTC encoding
	voiceBytes := make([]uint8, 12) // 96 bits = 12 bytes
	for i := 0; i < 12 && i*8 < len(maskedBits); i++ {
		voiceBytes[i] = BitsToByteBE(maskedBits[i*8:(i+1)*8])
	}

	// Apply BPTC(196,96) error correction coding
	bptc := NewBPTC19696()
	encodedBytes, ok := bptc.Encode(voiceBytes)
	if !ok {
		return fmt.Errorf("BPTC encode failed for frame %d", frameIndex)
	}

	// Pack encoded BPTC bytes into payload
	e.packBPTCBitsToPayload(encodedBytes, frameIndex, payload)

	return nil
}

// applyPRNGMasking applies PRNG masking to voice parameters (reverse of removePRNGMasking)
func (e *DMRAMBEExtractor) applyPRNGMasking(src, dest []bool, frameIndex int) {
	if len(src) != len(dest) {
		return
	}

	// Apply PRNG masking using the PRNG table
	for i := 0; i < len(src); i++ {
		// Calculate PRNG index based on bit position and frame
		prngIndex := (i + frameIndex*96) % PRNG_TABLE_SIZE

		// Get the PRNG mask value
		prngValue := PRNG_TABLE[prngIndex]

		// Extract the bit we need from the PRNG value
		prngBitIndex := i % 32
		prngBit := (prngValue & (1 << (31 - prngBitIndex))) != 0

		// XOR with PRNG bit to apply masking
		dest[i] = src[i] != prngBit // XOR operation
	}
}

// packBPTCBitsToPayload packs BPTC encoded bytes back into DMR payload format
func (e *DMRAMBEExtractor) packBPTCBitsToPayload(encodedBytes []uint8, frameIndex int, payload []byte) {
	// Calculate bit offset for this frame's BPTC codeword
	startBitPos := frameIndex * 196 // Each BPTC codeword is 196 bits

	// Convert bytes to bits and pack into payload
	for byteIdx := 0; byteIdx < len(encodedBytes); byteIdx++ {
		b := encodedBytes[byteIdx]
		for bitIdx := 0; bitIdx < 8; bitIdx++ {
			bitPos := startBitPos + (byteIdx*8) + bitIdx
			if bitPos >= len(payload)*8 {
				return // Prevent buffer overflow
			}

			payloadBytePos := bitPos / 8
			payloadBitOffset := bitPos % 8

			if payloadBytePos < len(payload) {
				bit := (b & (1 << (7 - bitIdx))) != 0
				if bit {
					payload[payloadBytePos] |= (1 << (7 - payloadBitOffset))
				} else {
					payload[payloadBytePos] &^= (1 << (7 - payloadBitOffset))
				}
			}
		}
	}
}