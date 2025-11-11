package codec

// YSFConvolution implements YSF Convolutional coding with Viterbi decoder
// This matches the C++ CNXDNConvolution class functionality for YSF voice protection
//
// Code parameters:
// - Rate 1/2 convolutional code (1 input bit → 2 output bits)
// - Constraint length K=5 (memory length 4, 16 states)
// - Generator polynomials: G1=(1+X²+X³), G2=(1+X+X²+X³)
// - Viterbi decoder with soft decision decoding
// - Used for YSF voice frame protection

// Constants from C++ implementation
const (
	YSF_CONV_NUM_STATES_D2 = 8  // Half the number of states
	YSF_CONV_NUM_STATES    = 16 // Total number of states (2^(K-1))
	YSF_CONV_M             = 2  // Maximum branch metric value (C++ const uint32_t M = 2U)
	YSF_CONV_K             = 5  // Constraint length
	YSF_CONV_MAX_BITS      = 300 // Maximum number of bits for decision storage
)

// Bit manipulation macros from C++
var YSF_CONV_BIT_MASK_TABLE = [8]uint8{0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01}

// Branch tables for metric calculation (from C++ BRANCH_TABLE1/2)
var YSF_CONV_BRANCH_TABLE1 = [8]uint8{0, 0, 0, 0, 1, 1, 1, 1}
var YSF_CONV_BRANCH_TABLE2 = [8]uint8{0, 1, 1, 0, 0, 1, 1, 0}

// YSFConvolution represents a convolutional encoder/decoder
type YSFConvolution struct {
	metrics1    [YSF_CONV_NUM_STATES]uint16  // Path metrics buffer 1
	metrics2    [YSF_CONV_NUM_STATES]uint16  // Path metrics buffer 2
	oldMetrics  []uint16                     // Pointer to current old metrics
	newMetrics  []uint16                     // Pointer to current new metrics
	decisions   [YSF_CONV_MAX_BITS]uint64    // Decision storage for chainback
	dp          int                          // Decision pointer
}

// NewYSFConvolution creates a new YSF convolutional encoder/decoder
func NewYSFConvolution() *YSFConvolution {
	conv := &YSFConvolution{}
	conv.oldMetrics = conv.metrics1[:]
	conv.newMetrics = conv.metrics2[:]
	return conv
}

// writeBit writes a bit to a byte array at the specified bit position
func (c *YSFConvolution) writeBit(data []uint8, pos uint32, bit bool) {
	bytePos := pos >> 3
	bitPos := pos & 7

	if bytePos < uint32(len(data)) {
		if bit {
			data[bytePos] |= YSF_CONV_BIT_MASK_TABLE[bitPos]
		} else {
			data[bytePos] &= ^YSF_CONV_BIT_MASK_TABLE[bitPos]
		}
	}
}

// readBit reads a bit from a byte array at the specified bit position
func (c *YSFConvolution) readBit(data []uint8, pos uint32) bool {
	bytePos := pos >> 3
	bitPos := pos & 7

	if bytePos < uint32(len(data)) {
		return (data[bytePos] & YSF_CONV_BIT_MASK_TABLE[bitPos]) != 0
	}
	return false
}

// Start initializes the decoder state
// Equivalent to C++ CNXDNConvolution::start()
func (c *YSFConvolution) Start() {
	// Initialize all metrics to zero
	for i := range c.metrics1 {
		c.metrics1[i] = 0
		c.metrics2[i] = 0
	}

	// Clear decisions array
	for i := range c.decisions {
		c.decisions[i] = 0
	}

	// Set up metric pointers
	c.oldMetrics = c.metrics1[:]
	c.newMetrics = c.metrics2[:]
	c.dp = 0
}

// Decode processes two soft decision symbols through the Viterbi decoder
// s0, s1: soft decision symbols (typically 0 or 2 for hard decisions)
// Equivalent to C++ CNXDNConvolution::decode()
func (c *YSFConvolution) Decode(s0, s1 uint8) {
	if c.dp >= YSF_CONV_MAX_BITS {
		return // Prevent buffer overflow
	}

	c.decisions[c.dp] = 0

	// Process all state transitions (exactly matching C++ logic)
	for i := uint8(0); i < YSF_CONV_NUM_STATES_D2; i++ {
		j := i * 2

		// Calculate branch metric (C++ version: (BRANCH_TABLE1[i] ^ s0) + (BRANCH_TABLE2[i] ^ s1))
		metric := uint16((YSF_CONV_BRANCH_TABLE1[i] ^ s0) + (YSF_CONV_BRANCH_TABLE2[i] ^ s1))

		// First transition (C++ lines)
		m0 := c.oldMetrics[i] + metric
		m1 := c.oldMetrics[i+YSF_CONV_NUM_STATES_D2] + (YSF_CONV_M - metric)
		var decision0 uint8
		if m0 >= m1 {
			decision0 = 1
		} else {
			decision0 = 0
		}
		// Store metric: decision0 != 0 ? m1 : m0
		if decision0 != 0 {
			c.newMetrics[j+0] = m1
		} else {
			c.newMetrics[j+0] = m0
		}

		// Second transition (C++ lines)
		m0 = c.oldMetrics[i] + (YSF_CONV_M - metric)
		m1 = c.oldMetrics[i+YSF_CONV_NUM_STATES_D2] + metric
		var decision1 uint8
		if m0 >= m1 {
			decision1 = 1
		} else {
			decision1 = 0
		}
		// Store metric: decision1 != 0 ? m1 : m0
		if decision1 != 0 {
			c.newMetrics[j+1] = m1
		} else {
			c.newMetrics[j+1] = m0
		}

		// Store decisions for chainback (C++ uses j+0U explicitly)
		c.decisions[c.dp] |= (uint64(decision1) << (j + 1)) | (uint64(decision0) << (j + 0))
	}

	c.dp++

	// Swap metric buffers
	c.oldMetrics, c.newMetrics = c.newMetrics, c.oldMetrics
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Chainback performs traceback to recover the original data bits
// out: output buffer for decoded bits
// nBits: number of bits to decode
// Equivalent to C++ CYSFConvolution::chainback()
func (c *YSFConvolution) Chainback(out []uint8, nBits uint32) {
	if len(out) == 0 || nBits == 0 {
		return
	}

	state := uint32(0)

	// Equivalent to C++ while (nBits-- > 0) - post-decrement behavior
	for nBits > 0 {
		nBits-- // Decrement first to match C++ post-decrement in condition

		c.dp--
		if c.dp < 0 {
			break
		}

		// Calculate decision bit position: state >> (9 - K)
		i := state >> (9 - YSF_CONV_K)
		bit := uint8((c.decisions[c.dp] >> i) & 1)

		// Update state: state = (bit << 7) | (state >> 1)
		state = (uint32(bit) << 7) | (state >> 1)

		// Write decoded bit to position nBits (now decremented)
		// This matches C++ WRITE_BIT1(out, nBits, bit != 0U)
		c.writeBit(out, nBits, bit != 0)
	}
}

// Encode performs convolutional encoding
// in: input data bits
// out: output encoded bits (twice the length of input)
// nBits: number of input bits
// Equivalent to C++ CNXDNConvolution::encode()
func (c *YSFConvolution) Encode(in []uint8, out []uint8, nBits uint32) {
	if len(in) == 0 || len(out) == 0 || nBits == 0 {
		return
	}

	// Initialize shift register
	var d1, d2, d3, d4 uint8
	k := uint32(0)

	for i := uint32(0); i < nBits; i++ {
		// Read input bit
		var d uint8
		if c.readBit(in, i) {
			d = 1
		} else {
			d = 0
		}

		// Calculate generator outputs
		g1 := (d + d3 + d4) & 1  // Generator polynomial 1+X²+X³
		g2 := (d + d1 + d2 + d4) & 1  // Generator polynomial 1+X+X²+X³

		// Shift register
		d4 = d3
		d3 = d2
		d2 = d1
		d1 = d

		// Write encoded bits
		c.writeBit(out, k, g1 != 0)
		k++
		c.writeBit(out, k, g2 != 0)
		k++
	}
}

// EncodeData is a convenience function for encoding data
// Returns the encoded data (twice the input length in bytes)
func (c *YSFConvolution) EncodeData(data []uint8, nBits uint32) []uint8 {
	// Calculate output size (twice the input bits, rounded up to bytes)
	outputBits := nBits * 2
	outputBytes := (outputBits + 7) / 8

	out := make([]uint8, outputBytes)
	c.Encode(data, out, nBits)
	return out
}

// DecodeData is a convenience function for decoding data
// Returns the decoded data and success flag
func (c *YSFConvolution) DecodeData(encoded []uint8, nEncodedBits uint32) ([]uint8, bool) {
	if nEncodedBits%2 != 0 || nEncodedBits == 0 {
		return nil, false // Encoded bits must be even (rate 1/2)
	}

	nDataBits := nEncodedBits / 2
	dataBytes := (nDataBits + 7) / 8
	out := make([]uint8, dataBytes)

	// Clear output buffer
	for i := range out {
		out[i] = 0
	}

	c.Start()

	// Process encoded symbols in pairs
	for i := uint32(0); i < nEncodedBits; i += 2 {
		var s0, s1 uint8

		// Convert bits to soft symbols (0 or 2 for hard decisions)
		if c.readBit(encoded, i) {
			s0 = 2
		} else {
			s0 = 0
		}

		if i+1 < nEncodedBits {
			if c.readBit(encoded, i+1) {
				s1 = 2
			} else {
				s1 = 0
			}
		} else {
			s1 = 0
		}

		c.Decode(s0, s1)
	}

	c.Chainback(out, nDataBits)
	return out, true
}

// DecodeSoft performs soft decision decoding
// encoded: array of soft symbols (typically 0-4 range)
// nSymbols: number of symbol pairs
func (c *YSFConvolution) DecodeSoft(encoded []uint8, nSymbols uint32) ([]uint8, bool) {
	if nSymbols == 0 || len(encoded) < int(nSymbols*2) {
		return nil, false
	}

	nDataBits := nSymbols
	dataBytes := (nDataBits + 7) / 8
	out := make([]uint8, dataBytes)

	c.Start()

	// Process soft symbol pairs
	for i := uint32(0); i < nSymbols; i++ {
		s0 := encoded[i*2]
		s1 := encoded[i*2+1]
		c.Decode(s0, s1)
	}

	c.Chainback(out, nDataBits)
	return out, true
}

// GetPathMetrics returns the current path metrics (for debugging)
func (c *YSFConvolution) GetPathMetrics() [YSF_CONV_NUM_STATES]uint16 {
	var metrics [YSF_CONV_NUM_STATES]uint16
	copy(metrics[:], c.oldMetrics)
	return metrics
}

// ValidateGenerator tests the generator polynomials
func (c *YSFConvolution) ValidateGenerator() bool {
	// Test encoding with known patterns
	testData := []uint8{0xAA} // 10101010 pattern
	nBits := uint32(8)

	encoded := c.EncodeData(testData, nBits)
	if len(encoded) == 0 {
		return false
	}

	// Decode and verify
	decoded, ok := c.DecodeData(encoded, nBits*2)
	if !ok || len(decoded) == 0 {
		return false
	}

	// Compare first byte (may have padding in last bits)
	return (decoded[0] & 0xFF) == (testData[0] & 0xFF)
}

// GetBER calculates bit error rate for soft symbols
func (c *YSFConvolution) GetBER(original, received []uint8, nBits uint32) float64 {
	if len(original) == 0 || len(received) == 0 {
		return 1.0
	}

	errors := 0
	totalBits := 0

	for i := uint32(0); i < nBits && i < uint32(len(original)*8) && i < uint32(len(received)*8); i++ {
		origBit := c.readBit(original, i)
		recvBit := c.readBit(received, i)

		if origBit != recvBit {
			errors++
		}
		totalBits++
	}

	if totalBits == 0 {
		return 1.0
	}

	return float64(errors) / float64(totalBits)
}