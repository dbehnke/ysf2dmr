package codec

import (
	"fmt"
	"testing"
)

// TestBPTC19696BasicRoundTrip tests basic encode/decode functionality
func TestBPTC19696BasicRoundTrip(t *testing.T) {
	bptc := NewBPTC19696()

	testPayloads := [][]uint8{
		// All zeros
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		// All ones
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		// Sequential pattern
		{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0xFE, 0xDC, 0xBA, 0x98},
		// Alternating pattern
		{0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55},
		// Random pattern
		{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, 0x11, 0x22, 0x33, 0x44},
		// Pattern with single bits
		{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80, 0x01, 0x02, 0x04, 0x08},
	}

	for i, payload := range testPayloads {
		t.Run(fmt.Sprintf("payload_%d", i), func(t *testing.T) {
			// Encode
			encoded, ok := bptc.Encode(payload)
			if !ok {
				t.Fatalf("Test %d: Encoding failed", i)
			}

			if len(encoded) != BPTC19696_INPUT_BYTES {
				t.Fatalf("Test %d: Encoded length incorrect: expected %d, got %d",
					i, BPTC19696_INPUT_BYTES, len(encoded))
			}

			// Decode
			decoded, ok := bptc.Decode(encoded)
			if !ok {
				t.Fatalf("Test %d: Decoding failed", i)
			}

			if len(decoded) != BPTC19696_OUTPUT_BYTES {
				t.Fatalf("Test %d: Decoded length incorrect: expected %d, got %d",
					i, BPTC19696_OUTPUT_BYTES, len(decoded))
			}

			// Compare
			for j := 0; j < BPTC19696_OUTPUT_BYTES; j++ {
				if payload[j] != decoded[j] {
					t.Errorf("Test %d: Byte %d mismatch: expected 0x%02X, got 0x%02X",
						i, j, payload[j], decoded[j])
				}
			}

			t.Logf("Test %d: Round trip successful for payload %X", i, payload)
		})
	}
}

// TestBPTC19696Validation tests the built-in validation function
func TestBPTC19696Validation(t *testing.T) {
	if !ValidateBPTC19696() {
		t.Error("BPTC19696 validation failed")
	}
}

// TestBPTC19696Constants tests the constants and parameters
func TestBPTC19696Constants(t *testing.T) {
	bptc := NewBPTC19696()

	// Test matrix dimensions
	rows, cols := bptc.GetMatrixDimensions()
	if rows != BPTC19696_MATRIX_ROWS {
		t.Errorf("Matrix rows: expected %d, got %d", BPTC19696_MATRIX_ROWS, rows)
	}
	if cols != BPTC19696_MATRIX_COLS {
		t.Errorf("Matrix cols: expected %d, got %d", BPTC19696_MATRIX_COLS, cols)
	}

	// Test code parameters
	totalBits, infoBits, parityBits := bptc.GetCodeParameters()
	if totalBits != BPTC19696_TOTAL_BITS {
		t.Errorf("Total bits: expected %d, got %d", BPTC19696_TOTAL_BITS, totalBits)
	}
	if infoBits != BPTC19696_INFO_BITS {
		t.Errorf("Info bits: expected %d, got %d", BPTC19696_INFO_BITS, infoBits)
	}
	if parityBits != BPTC19696_PARITY_BITS {
		t.Errorf("Parity bits: expected %d, got %d", BPTC19696_PARITY_BITS, parityBits)
	}

	// Test code rate
	expectedRate := float64(BPTC19696_INFO_BITS) / float64(BPTC19696_TOTAL_BITS)
	if expectedRate < 0.48 || expectedRate > 0.50 {
		t.Errorf("Code rate out of expected range: %.3f", expectedRate)
	}

	t.Logf("BPTC19696 parameters: (%d,%d) code, rate %.3f", totalBits, infoBits, expectedRate)
}

// TestBPTC19696SingleBitErrors tests single bit error correction
func TestBPTC19696SingleBitErrors(t *testing.T) {
	bptc := NewBPTC19696()

	payload := []uint8{
		0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5,
	}

	// Encode clean data
	encoded, ok := bptc.Encode(payload)
	if !ok {
		t.Fatal("Failed to encode test payload")
	}

	correctedCount := 0
	totalErrors := 0

	// Test single bit errors in each byte position
	for bytePos := 0; bytePos < len(encoded); bytePos++ {
		for bitPos := 0; bitPos < 8; bitPos++ {
			// Create corrupted copy
			corrupted := make([]uint8, len(encoded))
			copy(corrupted, encoded)

			// Flip single bit
			corrupted[bytePos] ^= (1 << bitPos)
			totalErrors++

			// Attempt to decode
			decoded, ok := bptc.Decode(corrupted)
			if !ok {
				continue // Decoding failed
			}

			// Check if original data was recovered
			isCorrect := true
			for j := 0; j < BPTC19696_OUTPUT_BYTES; j++ {
				if payload[j] != decoded[j] {
					isCorrect = false
					break
				}
			}

			if isCorrect {
				correctedCount++
			}
		}
	}

	t.Logf("Single bit error correction: %d/%d errors corrected (%.1f%%)",
		correctedCount, totalErrors, float64(correctedCount)*100.0/float64(totalErrors))

	// BPTC should correct a significant portion of single bit errors
	if correctedCount == 0 {
		t.Error("No single bit errors were corrected")
	}
}

// TestBPTC19696MultipleErrors tests behavior with multiple errors
func TestBPTC19696MultipleErrors(t *testing.T) {
	bptc := NewBPTC19696()

	payload := []uint8{
		0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, 0x11, 0x22, 0x33, 0x44,
	}

	// Test with specific error patterns
	errorPatterns := [][]int{
		{0, 1},       // Two adjacent bit errors
		{0, 8},       // Errors in different bytes
		{0, 16},      // Errors far apart
		{5, 10, 15},  // Three bit errors
		{0, 1, 2, 3}, // Four bit errors (burst)
	}

	for i, errorBits := range errorPatterns {
		success := bptc.TestErrorCorrection(payload, errorBits)
		t.Logf("Error pattern %d (%v): correction success = %t", i, errorBits, success)
	}
}

// TestBPTC19696Interleaving tests the interleaving function
func TestBPTC19696Interleaving(t *testing.T) {
	// Test interleave sequence calculation
	testPositions := []int{0, 1, 2, 10, 50, 100, 150, 195}

	for _, pos := range testPositions {
		// Forward interleave: (a * 181) % 196
		interleaved := (pos * 181) % BPTC19696_TOTAL_BITS

		// Verify it's within range
		if interleaved < 0 || interleaved >= BPTC19696_TOTAL_BITS {
			t.Errorf("Interleaved position %d out of range: %d", pos, interleaved)
		}

		t.Logf("Position %d → %d", pos, interleaved)
	}

	// Test that interleaving is a permutation (each position maps to unique position)
	used := make([]bool, BPTC19696_TOTAL_BITS)
	for i := 0; i < BPTC19696_TOTAL_BITS; i++ {
		interleaved := (i * 181) % BPTC19696_TOTAL_BITS
		if used[interleaved] {
			t.Errorf("Interleaving collision at position %d", interleaved)
		}
		used[interleaved] = true
	}

	// All positions should be used exactly once
	for i, wasUsed := range used {
		if !wasUsed {
			t.Errorf("Position %d was not used in interleaving", i)
		}
	}

	t.Log("Interleaving permutation verified")
}

// TestBPTC19696MatrixStructure tests the matrix data extraction
func TestBPTC19696MatrixStructure(t *testing.T) {
	// Verify the data bit positions match the specification
	expectedRanges := [][]int{
		{4, 11},    // First row data
		{16, 26},   // Second row data
		{31, 41},   // Third row data
		{46, 56},   // Fourth row data
		{61, 71},   // Fifth row data
		{76, 86},   // Sixth row data
		{91, 101},  // Seventh row data
		{106, 116}, // Eighth row data
		{121, 131}, // Ninth row data
	}

	totalDataBits := 0
	for _, r := range expectedRanges {
		bits := r[1] - r[0] + 1
		totalDataBits += bits
		t.Logf("Row data range %d-%d: %d bits", r[0], r[1], bits)
	}

	if totalDataBits != BPTC19696_INFO_BITS {
		t.Errorf("Total data bits mismatch: expected %d, calculated %d",
			BPTC19696_INFO_BITS, totalDataBits)
	}

	t.Logf("Matrix structure verified: %d data bits total", totalDataBits)
}

// TestBPTC19696EdgeCases tests edge cases and boundary conditions
func TestBPTC19696EdgeCases(t *testing.T) {
	bptc := NewBPTC19696()

	// Test with nil/empty input
	_, ok := bptc.Encode(nil)
	if ok {
		t.Error("Encode should fail with nil input")
	}

	_, ok = bptc.Decode(nil)
	if ok {
		t.Error("Decode should fail with nil input")
	}

	// Test with short input
	shortPayload := []uint8{0x01, 0x02, 0x03}
	_, ok = bptc.Encode(shortPayload)
	if ok {
		t.Error("Encode should fail with short input")
	}

	shortEncoded := []uint8{0x01, 0x02, 0x03, 0x04, 0x05}
	_, ok = bptc.Decode(shortEncoded)
	if ok {
		t.Error("Decode should fail with short input")
	}

	// Test with correct size but corrupted data (all errors)
	corruptedData := make([]uint8, BPTC19696_INPUT_BYTES)
	for i := range corruptedData {
		corruptedData[i] = 0xFF
	}

	decoded, ok := bptc.Decode(corruptedData)
	if ok {
		t.Logf("Highly corrupted data was decoded (may be false positive): %X", decoded[:4])
	} else {
		t.Log("Highly corrupted data was rejected (expected)")
	}
}

// TestBPTC19696Performance tests encoding/decoding performance
func TestBPTC19696Performance(t *testing.T) {
	bptc := NewBPTC19696()

	payload := []uint8{
		0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5,
	}

	iterations := 100
	for i := 0; i < iterations; i++ {
		encoded, ok := bptc.Encode(payload)
		if !ok {
			t.Fatalf("Encoding failed at iteration %d", i)
		}

		decoded, ok := bptc.Decode(encoded)
		if !ok {
			t.Fatalf("Decoding failed at iteration %d", i)
		}

		// Verify data integrity
		for j := 0; j < BPTC19696_OUTPUT_BYTES; j++ {
			if payload[j] != decoded[j] {
				t.Fatalf("Data corruption at iteration %d, byte %d", i, j)
			}
		}
	}

	t.Logf("Performance test completed: %d successful round trips", iterations)
}

// BenchmarkBPTC19696Encode benchmarks encoding performance
func BenchmarkBPTC19696Encode(b *testing.B) {
	bptc := NewBPTC19696()
	payload := []uint8{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0xFE, 0xDC, 0xBA, 0x98,
	}

	for i := 0; i < b.N; i++ {
		bptc.Encode(payload)
	}
}

// BenchmarkBPTC19696Decode benchmarks decoding performance
func BenchmarkBPTC19696Decode(b *testing.B) {
	bptc := NewBPTC19696()
	payload := []uint8{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0xFE, 0xDC, 0xBA, 0x98,
	}

	// Pre-encode the payload
	encoded, _ := bptc.Encode(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bptc.Decode(encoded)
	}
}

// BenchmarkBPTC19696RoundTrip benchmarks complete round trip performance
func BenchmarkBPTC19696RoundTrip(b *testing.B) {
	bptc := NewBPTC19696()
	payload := []uint8{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0xFE, 0xDC, 0xBA, 0x98,
	}

	for i := 0; i < b.N; i++ {
		encoded, _ := bptc.Encode(payload)
		bptc.Decode(encoded)
	}
}

// TestBPTC19696CodeRate tests the code efficiency
func TestBPTC19696CodeRate(t *testing.T) {
	// Calculate code rate
	rate := float64(BPTC19696_INFO_BITS) / float64(BPTC19696_TOTAL_BITS)
	overhead := float64(BPTC19696_PARITY_BITS) / float64(BPTC19696_INFO_BITS)

	t.Logf("BPTC19696 Code Rate: %.3f (%d/%d)", rate, BPTC19696_INFO_BITS, BPTC19696_TOTAL_BITS)
	t.Logf("Redundancy Overhead: %.1f%% (%d parity bits for %d data bits)",
		overhead*100, BPTC19696_PARITY_BITS, BPTC19696_INFO_BITS)

	// Input/output byte efficiency
	byteEfficiency := float64(BPTC19696_OUTPUT_BYTES) / float64(BPTC19696_INPUT_BYTES)
	t.Logf("Byte Efficiency: %.3f (%d output bytes from %d input bytes)",
		byteEfficiency, BPTC19696_OUTPUT_BYTES, BPTC19696_INPUT_BYTES)

	// Verify expected values
	expectedRate := 96.0 / 196.0 // approximately 0.490
	if rate < expectedRate-0.01 || rate > expectedRate+0.01 {
		t.Errorf("Code rate unexpected: %.3f (expected ~%.3f)", rate, expectedRate)
	}
}

// TestBPTC19696SpecificErrorPatterns tests known error patterns
func TestBPTC19696SpecificErrorPatterns(t *testing.T) {
	bptc := NewBPTC19696()

	payload := []uint8{
		0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A, 0xA5, 0x5A,
	}

	// Test patterns that should be correctable
	correctablePatterns := [][]int{
		{0},     // Single bit error
		{63},    // Single bit error in middle
		{199},   // Single bit error near end
	}

	for i, pattern := range correctablePatterns {
		success := bptc.TestErrorCorrection(payload, pattern)
		if !success {
			t.Errorf("Pattern %d should be correctable but failed: %v", i, pattern)
		} else {
			t.Logf("Pattern %d corrected successfully: %v", i, pattern)
		}
	}

	// Test patterns that may not be correctable
	difficultPatterns := [][]int{
		{0, 1, 2, 3, 4, 5, 6, 7}, // Byte error (8 bits)
		{0, 8, 16, 24, 32},       // Distributed errors
	}

	for i, pattern := range difficultPatterns {
		success := bptc.TestErrorCorrection(payload, pattern)
		if success {
			t.Logf("Difficult pattern %d was corrected: %v", i, pattern)
		} else {
			t.Logf("Difficult pattern %d was not corrected (expected): %v", i, pattern)
		}
	}
}