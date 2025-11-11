package codec

// BPTC19696 implements Block Product Turbo Code (196,96) for DMR Link Control
// This matches the C++ CBPTC19696 class functionality for DMR data protection
//
// Code parameters:
// - (196,96) block code: 96 information bits, 100 parity bits
// - Matrix structure: 13×15 = 195 bits (+ 1 unused = 196 total)
// - Hamming(15,11,3) codes for rows (variant 2)
// - Hamming(13,9,3) codes for columns
// - Interleaving: (a * 181) % 196 permutation for burst error protection
// - Iterative decoding: up to 5 iterations for error convergence
// - Used for DMR Link Control data protection

// Constants from C++ implementation
const (
	BPTC19696_TOTAL_BITS   = 196 // Total bits in codeword
	BPTC19696_INFO_BITS    = 96  // Information bits
	BPTC19696_PARITY_BITS  = 100 // Parity bits (196 - 96)
	BPTC19696_INPUT_BYTES  = 33  // Input bytes for decode (packed bits)
	BPTC19696_OUTPUT_BYTES = 12  // Output bytes for payload
	BPTC19696_MATRIX_ROWS  = 13  // Matrix rows (9 data + 4 parity)
	BPTC19696_MATRIX_COLS  = 15  // Matrix columns
	BPTC19696_DATA_ROWS    = 9   // Data rows (first 9 rows)
	BPTC19696_MAX_ITER     = 5   // Maximum error correction iterations
)

// BPTC19696 represents a BPTC(196,96) encoder/decoder
type BPTC19696 struct {
	rawData     [BPTC19696_TOTAL_BITS]bool // Raw data after bit extraction
	deInterData [BPTC19696_TOTAL_BITS]bool // Deinterleaved matrix data
}

// NewBPTC19696 creates a new BPTC(196,96) encoder/decoder
func NewBPTC19696() *BPTC19696 {
	return &BPTC19696{}
}

// Decode decodes 33 bytes of input data to 12 bytes of payload
// Input: 33-byte array with encoded data
// Output: 12-byte array with decoded payload
// Equivalent to C++ CBPTC19696::decode()
func (b *BPTC19696) Decode(input []uint8) ([]uint8, bool) {
	if len(input) < BPTC19696_INPUT_BYTES {
		return nil, false
	}

	output := make([]uint8, BPTC19696_OUTPUT_BYTES)

	// Extract binary data from input bytes
	b.decodeExtractBinary(input)

	// Deinterleave using (a * 181) % 196 permutation
	b.decodeDeInterleave()

	// Iterative error correction using Hamming codes
	b.decodeErrorCheck()

	// Extract 96 payload bits from matrix
	b.decodeExtractData(output)

	return output, true
}

// Encode encodes 12 bytes of payload to 33 bytes of output data
// Input: 12-byte array with payload data
// Output: 33-byte array with encoded data
// Equivalent to C++ CBPTC19696::encode()
func (b *BPTC19696) Encode(payload []uint8) ([]uint8, bool) {
	if len(payload) < BPTC19696_OUTPUT_BYTES {
		return nil, false
	}

	output := make([]uint8, BPTC19696_INPUT_BYTES)

	// Extract 96 payload bits into matrix structure
	b.encodeExtractData(payload)

	// Calculate Hamming parity bits for rows and columns
	b.encodeErrorCheck()

	// Interleave using (a * 181) % 196 permutation
	b.encodeInterleave()

	// Pack bits into 33-byte output
	b.encodeExtractBinary(output)

	return output, true
}

// decodeExtractBinary extracts 196 bits from 33 input bytes
// Equivalent to C++ CBPTC19696::decodeExtractBinary()
func (b *BPTC19696) decodeExtractBinary(input []uint8) {
	// Clear raw data
	for i := range b.rawData {
		b.rawData[i] = false
	}

	// First block: bytes 0-12 → bits 0-97
	for i := 0; i < 13; i++ {
		ByteToBitsBE(input[i], b.rawData[i*8:(i+1)*8])
	}

	// Handle the two special bits from byte 20
	var tempBits [8]bool
	ByteToBitsBE(input[20], tempBits[:])
	b.rawData[98] = tempBits[6] // Bit 6
	b.rawData[99] = tempBits[7] // Bit 7

	// Second block: bytes 21-32 → bits 100-195
	for i := 0; i < 12; i++ {
		ByteToBitsBE(input[21+i], b.rawData[100+i*8:108+i*8])
	}
}

// decodeDeInterleave deinterleaves data using (a * 181) % 196 permutation
// Equivalent to C++ CBPTC19696::decodeDeInterleave()
func (b *BPTC19696) decodeDeInterleave() {
	// Clear deinterleaved data
	for i := range b.deInterData {
		b.deInterData[i] = false
	}

	// Apply deinterleaving permutation
	for a := 0; a < BPTC19696_TOTAL_BITS; a++ {
		// Calculate the interleave sequence: (a * 181) % 196
		interleaveSequence := (a * 181) % BPTC19696_TOTAL_BITS
		// Shuffle the data (reverse of interleaving)
		b.deInterData[a] = b.rawData[interleaveSequence]
	}
}

// decodeErrorCheck performs iterative error correction using Hamming codes
// Equivalent to C++ CBPTC19696::decodeErrorCheck()
func (b *BPTC19696) decodeErrorCheck() {
	var fixing bool
	count := 0

	// Iterative error correction (up to 5 iterations)
	for {
		fixing = false

		// Run through each of the 15 columns using Hamming(13,9,3)
		var col [13]bool
		for c := 0; c < BPTC19696_MATRIX_COLS; c++ {
			pos := c + 1
			for a := 0; a < 13; a++ {
				if pos < BPTC19696_TOTAL_BITS {
					col[a] = b.deInterData[pos]
				} else {
					col[a] = false
				}
				pos += BPTC19696_MATRIX_COLS
			}

			if Decode1393(col[:]) {
				pos = c + 1
				for a := 0; a < 13; a++ {
					if pos < BPTC19696_TOTAL_BITS {
						b.deInterData[pos] = col[a]
					}
					pos += BPTC19696_MATRIX_COLS
				}
				fixing = true
			}
		}

		// Run through each of the 9 rows containing data using Hamming(15,11,3)
		for r := 0; r < BPTC19696_DATA_ROWS; r++ {
			pos := (r * BPTC19696_MATRIX_COLS) + 1
			if pos+BPTC19696_MATRIX_COLS <= BPTC19696_TOTAL_BITS {
				if Decode15113_2(b.deInterData[pos : pos+BPTC19696_MATRIX_COLS]) {
					fixing = true
				}
			}
		}

		count++
		if !fixing || count >= BPTC19696_MAX_ITER {
			break
		}
	}
}

// decodeExtractData extracts 96 payload bits from deinterleaved matrix
// Equivalent to C++ CBPTC19696::decodeExtractData()
func (b *BPTC19696) decodeExtractData(output []uint8) {
	var bData [BPTC19696_INFO_BITS]bool
	pos := 0

	// Extract data bits from specific matrix positions
	// These positions correspond to the data bit locations in the BPTC matrix
	ranges := [][]int{
		{4, 11},   // Positions 4-11
		{16, 26},  // Positions 16-26
		{31, 41},  // Positions 31-41
		{46, 56},  // Positions 46-56
		{61, 71},  // Positions 61-71
		{76, 86},  // Positions 76-86
		{91, 101}, // Positions 91-101
		{106, 116}, // Positions 106-116
		{121, 131}, // Positions 121-131
	}

	for _, r := range ranges {
		for a := r[0]; a <= r[1] && pos < BPTC19696_INFO_BITS; a++ {
			if a < BPTC19696_TOTAL_BITS {
				bData[pos] = b.deInterData[a]
			}
			pos++
		}
	}

	// Convert 96 bits to 12 bytes
	for i := 0; i < BPTC19696_OUTPUT_BYTES && i*8 < BPTC19696_INFO_BITS; i++ {
		output[i] = BitsToByteBE(bData[i*8 : (i+1)*8])
	}
}

// encodeExtractData places 96 payload bits into matrix structure
// Equivalent to C++ CBPTC19696::encodeExtractData()
func (b *BPTC19696) encodeExtractData(payload []uint8) {
	// Convert 12 bytes to 96 bits
	var bData [BPTC19696_INFO_BITS]bool
	for i := 0; i < BPTC19696_OUTPUT_BYTES && i*8 < BPTC19696_INFO_BITS; i++ {
		ByteToBitsBE(payload[i], bData[i*8:(i+1)*8])
	}

	// Clear matrix
	for i := range b.deInterData {
		b.deInterData[i] = false
	}

	// Place data bits into specific matrix positions
	pos := 0
	ranges := [][]int{
		{4, 11},   // Positions 4-11
		{16, 26},  // Positions 16-26
		{31, 41},  // Positions 31-41
		{46, 56},  // Positions 46-56
		{61, 71},  // Positions 61-71
		{76, 86},  // Positions 76-86
		{91, 101}, // Positions 91-101
		{106, 116}, // Positions 106-116
		{121, 131}, // Positions 121-131
	}

	for _, r := range ranges {
		for a := r[0]; a <= r[1] && pos < BPTC19696_INFO_BITS; a++ {
			if a < BPTC19696_TOTAL_BITS {
				b.deInterData[a] = bData[pos]
			}
			pos++
		}
	}
}

// encodeErrorCheck calculates Hamming parity bits for rows and columns
// Equivalent to C++ CBPTC19696::encodeErrorCheck()
func (b *BPTC19696) encodeErrorCheck() {
	// Run through each of the 9 rows containing data using Hamming(15,11,3)
	for r := 0; r < BPTC19696_DATA_ROWS; r++ {
		pos := (r * BPTC19696_MATRIX_COLS) + 1
		if pos+BPTC19696_MATRIX_COLS <= BPTC19696_TOTAL_BITS {
			Encode15113_2(b.deInterData[pos : pos+BPTC19696_MATRIX_COLS])
		}
	}

	// Run through each of the 15 columns using Hamming(13,9,3)
	var col [13]bool
	for c := 0; c < BPTC19696_MATRIX_COLS; c++ {
		pos := c + 1
		for a := 0; a < 13; a++ {
			if pos < BPTC19696_TOTAL_BITS {
				col[a] = b.deInterData[pos]
			} else {
				col[a] = false
			}
			pos += BPTC19696_MATRIX_COLS
		}

		Encode1393(col[:])

		pos = c + 1
		for a := 0; a < 13; a++ {
			if pos < BPTC19696_TOTAL_BITS {
				b.deInterData[pos] = col[a]
			}
			pos += BPTC19696_MATRIX_COLS
		}
	}
}

// encodeInterleave interleaves data using (a * 181) % 196 permutation
// Equivalent to C++ CBPTC19696::encodeInterleave()
func (b *BPTC19696) encodeInterleave() {
	// Clear raw data
	for i := range b.rawData {
		b.rawData[i] = false
	}

	// Apply interleaving permutation
	for a := 0; a < BPTC19696_TOTAL_BITS; a++ {
		// Calculate the interleave sequence: (a * 181) % 196
		interleaveSequence := (a * 181) % BPTC19696_TOTAL_BITS
		// Unshuffle the data (reverse of deinterleaving)
		b.rawData[interleaveSequence] = b.deInterData[a]
	}
}

// encodeExtractBinary packs 196 bits into 33 output bytes
// Equivalent to C++ CBPTC19696::encodeExtractBinary()
func (b *BPTC19696) encodeExtractBinary(output []uint8) {
	// Clear output
	for i := range output {
		output[i] = 0
	}

	// First block: bits 0-97 → bytes 0-12
	for i := 0; i < 13; i++ {
		if (i+1)*8 <= len(b.rawData) {
			output[i] = BitsToByteBE(b.rawData[i*8 : (i+1)*8])
		}
	}

	// Handle the two special bits: bits 98-99 → byte 20 bits 6-7
	if len(output) > 20 && len(b.rawData) > 99 {
		var tempByte uint8
		if 96+8 <= len(b.rawData) {
			tempByte = BitsToByteBE(b.rawData[96:104])
		}
		output[12] = (output[12] & 0x3F) | ((tempByte >> 0) & 0xC0)
		output[20] = (output[20] & 0xFC) | ((tempByte >> 4) & 0x03)
	}

	// Second block: bits 100-195 → bytes 21-32
	for i := 0; i < 12 && 21+i < len(output); i++ {
		startBit := 100 + i*8
		endBit := startBit + 8
		if endBit <= len(b.rawData) {
			output[21+i] = BitsToByteBE(b.rawData[startBit:endBit])
		}
	}
}

// ValidateBPTC19696 validates the BPTC implementation with round-trip test
func ValidateBPTC19696() bool {
	bptc := NewBPTC19696()

	// Test with known payload
	payload := []uint8{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF,
		0xFE, 0xDC, 0xBA, 0x98,
	}

	// Encode
	encoded, ok := bptc.Encode(payload)
	if !ok || len(encoded) != BPTC19696_INPUT_BYTES {
		return false
	}

	// Decode
	decoded, ok := bptc.Decode(encoded)
	if !ok || len(decoded) != BPTC19696_OUTPUT_BYTES {
		return false
	}

	// Compare
	for i := 0; i < BPTC19696_OUTPUT_BYTES; i++ {
		if payload[i] != decoded[i] {
			return false
		}
	}

	return true
}

// GetMatrixDimensions returns the BPTC matrix dimensions for debugging
func (b *BPTC19696) GetMatrixDimensions() (int, int) {
	return BPTC19696_MATRIX_ROWS, BPTC19696_MATRIX_COLS
}

// GetCodeParameters returns the BPTC code parameters for debugging
func (b *BPTC19696) GetCodeParameters() (int, int, int) {
	return BPTC19696_TOTAL_BITS, BPTC19696_INFO_BITS, BPTC19696_PARITY_BITS
}

// TestErrorCorrection tests the error correction capabilities
func (b *BPTC19696) TestErrorCorrection(payload []uint8, errorPositions []int) bool {
	if len(payload) != BPTC19696_OUTPUT_BYTES {
		return false
	}

	// Encode clean data
	encoded, ok := b.Encode(payload)
	if !ok {
		return false
	}

	// Introduce errors at specified bit positions
	for _, pos := range errorPositions {
		if pos >= 0 && pos < len(encoded)*8 {
			bytePos := pos / 8
			bitPos := pos % 8
			encoded[bytePos] ^= (1 << (7 - bitPos))
		}
	}

	// Attempt to decode corrupted data
	decoded, ok := b.Decode(encoded)
	if !ok {
		return false
	}

	// Check if original data was recovered
	for i := 0; i < BPTC19696_OUTPUT_BYTES; i++ {
		if payload[i] != decoded[i] {
			return false
		}
	}

	return true
}