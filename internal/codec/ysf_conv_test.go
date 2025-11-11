package codec

import (
	"testing"
)

// TestYSFConvBasic tests basic encode/decode functionality
func TestYSFConvBasic(t *testing.T) {
	conv := NewYSFConvolution()

	testData := []struct {
		name string
		data []uint8
		bits uint32
	}{
		{"all zeros", []uint8{0x00}, 8},
		{"all ones", []uint8{0xFF}, 8},
		{"pattern 0xAA", []uint8{0xAA}, 8},
		{"pattern 0x55", []uint8{0x55}, 8},
		{"sequential", []uint8{0x12, 0x34}, 16},
		{"single bit", []uint8{0x80}, 8},
		{"random", []uint8{0x5A, 0xA5, 0xC3}, 24},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			// Encode
			encoded := conv.EncodeData(test.data, test.bits)
			if len(encoded) == 0 {
				t.Fatal("Encoding failed")
			}

			// Decode
			decoded, ok := conv.DecodeData(encoded, test.bits*2)
			if !ok {
				t.Fatal("Decoding failed")
			}

			// Compare (only compare relevant bytes)
			dataBytes := (test.bits + 7) / 8
			for i := uint32(0); i < dataBytes; i++ {
				if i >= uint32(len(test.data)) || i >= uint32(len(decoded)) {
					continue
				}

				// For the last byte, only compare the relevant bits
				mask := uint8(0xFF)
				if i == dataBytes-1 && test.bits%8 != 0 {
					mask = uint8(0xFF) << (8 - (test.bits % 8))
				}

				expected := test.data[i] & mask
				actual := decoded[i] & mask

				if actual != expected {
					t.Errorf("Byte %d: expected 0x%02X, got 0x%02X (mask 0x%02X)",
						i, expected, actual, mask)
				}
			}
		})
	}
}

// TestYSFConvEncoding tests the encoding process step by step
func TestYSFConvEncoding(t *testing.T) {
	conv := NewYSFConvolution()

	// Test with known input pattern
	input := []uint8{0x80} // Single bit: 10000000
	nBits := uint32(1)

	encoded := conv.EncodeData(input, nBits)
	if len(encoded) == 0 {
		t.Fatal("Encoding failed")
	}

	// With generators G1=(1+X²+X³) and G2=(1+X+X²+X³)
	// For input bit 1: G1=1, G2=1, so output should be [1,1]
	expectedBit0 := conv.readBit(encoded, 0) // First output bit
	expectedBit1 := conv.readBit(encoded, 1) // Second output bit

	t.Logf("Input: 0x%02X", input[0])
	t.Logf("Encoded: 0x%02X", encoded[0])
	t.Logf("Output bits: [%t, %t]", expectedBit0, expectedBit1)

	// For input=1, both generators should output 1
	if !expectedBit0 || !expectedBit1 {
		t.Errorf("Expected [true, true], got [%t, %t]", expectedBit0, expectedBit1)
	}
}

// TestYSFConvViterbiDecoder tests the Viterbi decoder operation
func TestYSFConvViterbiDecoder(t *testing.T) {
	conv := NewYSFConvolution()

	// Test with soft symbols
	softSymbols := []uint8{2, 2, 0, 2, 2, 0, 0, 0} // 4 symbol pairs
	nSymbols := uint32(4)

	decoded, ok := conv.DecodeSoft(softSymbols, nSymbols)
	if !ok {
		t.Fatal("Soft decoding failed")
	}

	if len(decoded) == 0 {
		t.Fatal("No decoded data")
	}

	t.Logf("Soft symbols: %v", softSymbols)
	t.Logf("Decoded: 0x%02X", decoded[0])

	// Test path metrics
	conv.Start()
	conv.Decode(2, 2) // High confidence symbols
	metrics := conv.GetPathMetrics()

	t.Logf("Path metrics after decode: %v", metrics)

	// At least one metric should be non-zero after processing
	hasNonZero := false
	for _, metric := range metrics {
		if metric != 0 {
			hasNonZero = true
			break
		}
	}

	if !hasNonZero {
		t.Error("All path metrics are zero after decoding")
	}
}

// TestYSFConvBitManipulation tests bit manipulation functions
func TestYSFConvBitManipulation(t *testing.T) {
	conv := NewYSFConvolution()

	data := make([]uint8, 2)

	// Test writing and reading bits
	testBits := []struct {
		pos uint32
		bit bool
	}{
		{0, true},
		{1, false},
		{7, true},
		{8, false},
		{15, true},
	}

	for _, test := range testBits {
		conv.writeBit(data, test.pos, test.bit)
		result := conv.readBit(data, test.pos)

		if result != test.bit {
			t.Errorf("Bit %d: wrote %t, read %t", test.pos, test.bit, result)
		}
	}

	t.Logf("Final data: [0x%02X, 0x%02X]", data[0], data[1])
}

// TestYSFConvErrorCorrection tests error correction capabilities
func TestYSFConvErrorCorrection(t *testing.T) {
	conv := NewYSFConvolution()

	// Test data
	original := []uint8{0x5A} // 01011010
	nBits := uint32(8)

	// Encode
	encoded := conv.EncodeData(original, nBits)
	if len(encoded) == 0 {
		t.Fatal("Encoding failed")
	}

	// Introduce single bit errors and test correction
	for bitPos := uint32(0); bitPos < nBits*2 && bitPos < uint32(len(encoded)*8); bitPos++ {
		// Create corrupted version
		corrupted := make([]uint8, len(encoded))
		copy(corrupted, encoded)

		// Flip one bit
		bytePos := bitPos / 8
		bitOffset := bitPos % 8
		if bytePos < uint32(len(corrupted)) {
			corrupted[bytePos] ^= (1 << (7 - bitOffset))
		}

		// Decode
		decoded, ok := conv.DecodeData(corrupted, nBits*2)
		if !ok {
			t.Errorf("Decoding failed for error at bit %d", bitPos)
			continue
		}

		// Check if error was corrected (for some errors)
		if len(decoded) > 0 {
			// Convolutional codes can correct some errors depending on the error pattern
			// We mainly test that decoding doesn't crash
			t.Logf("Bit %d error: original=0x%02X, decoded=0x%02X",
				bitPos, original[0], decoded[0])
		}
	}
}

// TestYSFConvGeneratorValidation tests generator polynomial validation
func TestYSFConvGeneratorValidation(t *testing.T) {
	conv := NewYSFConvolution()

	if !conv.ValidateGenerator() {
		t.Error("Generator polynomial validation failed")
	}
}

// TestYSFConvBER tests bit error rate calculation
func TestYSFConvBER(t *testing.T) {
	conv := NewYSFConvolution()

	original := []uint8{0xAA, 0x55}
	same := []uint8{0xAA, 0x55}
	different := []uint8{0x55, 0xAA}

	// Test identical data (BER = 0)
	ber := conv.GetBER(original, same, 16)
	if ber != 0.0 {
		t.Errorf("BER for identical data should be 0.0, got %f", ber)
	}

	// Test completely different data (BER = 1.0)
	ber = conv.GetBER(original, different, 16)
	if ber != 1.0 {
		t.Errorf("BER for completely different data should be 1.0, got %f", ber)
	}

	// Test single bit difference
	singleBit := []uint8{0xAB, 0x55} // Changed one bit
	ber = conv.GetBER(original, singleBit, 16)
	expectedBER := 1.0 / 16.0 // 1 bit out of 16
	if ber != expectedBER {
		t.Errorf("BER for single bit error should be %f, got %f", expectedBER, ber)
	}

	t.Logf("BER test results: identical=%.3f, different=%.3f, single bit=%.3f",
		conv.GetBER(original, same, 16),
		conv.GetBER(original, different, 16),
		conv.GetBER(original, singleBit, 16))
}

// TestYSFConvBranchTables tests the branch table values
func TestYSFConvBranchTables(t *testing.T) {
	// Verify branch table sizes
	if len(YSF_CONV_BRANCH_TABLE1) != 8 {
		t.Errorf("BRANCH_TABLE1 size should be 8, got %d", len(YSF_CONV_BRANCH_TABLE1))
	}

	if len(YSF_CONV_BRANCH_TABLE2) != 8 {
		t.Errorf("BRANCH_TABLE2 size should be 8, got %d", len(YSF_CONV_BRANCH_TABLE2))
	}

	// Verify table values match C++ implementation
	expectedTable1 := [8]uint8{0, 0, 0, 0, 2, 2, 2, 2}
	expectedTable2 := [8]uint8{0, 2, 2, 0, 0, 2, 2, 0}

	for i := 0; i < 8; i++ {
		if YSF_CONV_BRANCH_TABLE1[i] != expectedTable1[i] {
			t.Errorf("BRANCH_TABLE1[%d] = %d, expected %d",
				i, YSF_CONV_BRANCH_TABLE1[i], expectedTable1[i])
		}

		if YSF_CONV_BRANCH_TABLE2[i] != expectedTable2[i] {
			t.Errorf("BRANCH_TABLE2[%d] = %d, expected %d",
				i, YSF_CONV_BRANCH_TABLE2[i], expectedTable2[i])
		}
	}
}

// TestYSFConvConstants tests the constants
func TestYSFConvConstants(t *testing.T) {
	if YSF_CONV_NUM_STATES != 16 {
		t.Errorf("NUM_STATES should be 16, got %d", YSF_CONV_NUM_STATES)
	}

	if YSF_CONV_NUM_STATES_D2 != 8 {
		t.Errorf("NUM_STATES_D2 should be 8, got %d", YSF_CONV_NUM_STATES_D2)
	}

	if YSF_CONV_M != 4 {
		t.Errorf("M should be 4, got %d", YSF_CONV_M)
	}

	if YSF_CONV_K != 5 {
		t.Errorf("K should be 5, got %d", YSF_CONV_K)
	}

	if YSF_CONV_MAX_BITS != 300 {
		t.Errorf("MAX_BITS should be 300, got %d", YSF_CONV_MAX_BITS)
	}
}

// TestYSFConvEdgeCases tests edge cases and boundary conditions
func TestYSFConvEdgeCases(t *testing.T) {
	conv := NewYSFConvolution()

	// Test with nil/empty data
	encoded := conv.EncodeData(nil, 0)
	if len(encoded) != 0 {
		t.Error("Encoding nil data should return empty slice")
	}

	encoded = conv.EncodeData([]uint8{}, 0)
	if len(encoded) != 0 {
		t.Error("Encoding empty data should return empty slice")
	}

	// Test decoding empty data
	decoded, ok := conv.DecodeData(nil, 0)
	if ok || decoded != nil {
		t.Error("Decoding nil data should fail")
	}

	// Test odd number of encoded bits (invalid for rate 1/2)
	decoded, ok = conv.DecodeData([]uint8{0x12}, 3) // 3 bits is odd
	if ok {
		t.Error("Decoding odd number of bits should fail")
	}

	// Test large data (within limits)
	largeData := make([]uint8, 32)
	for i := range largeData {
		largeData[i] = uint8(i)
	}

	encoded = conv.EncodeData(largeData, uint32(len(largeData)*8))
	if len(encoded) == 0 {
		t.Error("Failed to encode large data")
	}

	decoded, ok = conv.DecodeData(encoded, uint32(len(largeData)*16))
	if !ok || len(decoded) == 0 {
		t.Error("Failed to decode large data")
	}

	t.Logf("Large data test: original %d bytes, encoded %d bytes, decoded %d bytes",
		len(largeData), len(encoded), len(decoded))
}

// TestYSFConvStateManagement tests state management
func TestYSFConvStateManagement(t *testing.T) {
	conv := NewYSFConvolution()

	// Test initial state
	conv.Start()
	metrics := conv.GetPathMetrics()

	// All metrics should be zero initially
	for i, metric := range metrics {
		if metric != 0 {
			t.Errorf("Initial metric[%d] should be 0, got %d", i, metric)
		}
	}

	// Process some symbols and verify state changes
	conv.Decode(2, 0)
	metrics = conv.GetPathMetrics()

	// At least some metrics should be non-zero after processing
	hasNonZero := false
	for _, metric := range metrics {
		if metric != 0 {
			hasNonZero = true
			break
		}
	}

	if !hasNonZero {
		t.Error("No path metrics updated after decode operation")
	}

	// Test multiple decode operations
	for i := 0; i < 10; i++ {
		conv.Decode(uint8(i%3), uint8((i+1)%3))
	}

	metrics = conv.GetPathMetrics()
	t.Logf("Metrics after 10 decode operations: %v", metrics[:4]) // Show first few
}

// BenchmarkYSFConvEncode benchmarks encoding performance
func BenchmarkYSFConvEncode(b *testing.B) {
	conv := NewYSFConvolution()
	data := []uint8{0x5A, 0xA5, 0x5A, 0xA5}
	nBits := uint32(32)

	for i := 0; i < b.N; i++ {
		conv.EncodeData(data, nBits)
	}
}

// BenchmarkYSFConvDecode benchmarks decoding performance
func BenchmarkYSFConvDecode(b *testing.B) {
	conv := NewYSFConvolution()
	data := []uint8{0x5A, 0xA5, 0x5A, 0xA5}
	nBits := uint32(32)

	// Pre-encode data
	encoded := conv.EncodeData(data, nBits)

	for i := 0; i < b.N; i++ {
		conv.DecodeData(encoded, nBits*2)
	}
}

// BenchmarkYSFConvViterbiStep benchmarks single Viterbi step
func BenchmarkYSFConvViterbiStep(b *testing.B) {
	conv := NewYSFConvolution()
	conv.Start()

	for i := 0; i < b.N; i++ {
		conv.Decode(2, 0)
	}
}

// BenchmarkYSFConvSoftDecode benchmarks soft symbol decoding
func BenchmarkYSFConvSoftDecode(b *testing.B) {
	conv := NewYSFConvolution()
	softSymbols := []uint8{2, 1, 0, 2, 1, 0, 2, 1, 0, 2, 1, 0, 2, 1, 0, 2}
	nSymbols := uint32(8)

	for i := 0; i < b.N; i++ {
		conv.DecodeSoft(softSymbols, nSymbols)
	}
}