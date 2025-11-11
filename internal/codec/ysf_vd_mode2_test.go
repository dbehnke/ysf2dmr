package codec

import (
	"testing"
)

// TestYSFVDMode2BasicRoundTrip tests basic callsign encode/decode functionality
func TestYSFVDMode2BasicRoundTrip(t *testing.T) {
	vd := NewYSFVDMode2()

	testCallsigns := []string{
		"KJ4ABC",
		"VK2DEF",
		"JA1XYZ",
		"G0TST",
		"W5TEST",
		"VE3HAM",
		"DL1ABC",
		"F4DEF",
		"OH2GHI",
		"SM0JKL",
		"1234567890", // 10 chars exactly
		"A",          // Single char
		"",           // Empty string
	}

	for _, callsign := range testCallsigns {
		t.Run(callsign, func(t *testing.T) {
			if callsign != "" && !vd.ValidateCallsign(callsign) {
				t.Logf("Callsign '%s' failed validation, skipping", callsign)
				return
			}

			// Encode the callsign
			encoded := vd.EncodeCallsign(callsign)

			// Decode it back
			decoded, ok := vd.DecodeCallsign(encoded)
			if !ok {
				t.Fatalf("Failed to decode callsign '%s'", callsign)
			}

			// For comparison, pad original to 10 chars and trim decoded
			expectedPadded := callsign
			for len(expectedPadded) < YSF_VD_MODE2_CALLSIGN_LENGTH {
				expectedPadded += " "
			}
			expectedTrimmed := expectedPadded
			for len(expectedTrimmed) > 0 && expectedTrimmed[len(expectedTrimmed)-1] == ' ' {
				expectedTrimmed = expectedTrimmed[:len(expectedTrimmed)-1]
			}

			if decoded != expectedTrimmed {
				t.Errorf("Callsign mismatch: original='%s', expected='%s', decoded='%s'",
					callsign, expectedTrimmed, decoded)
			}

			t.Logf("Callsign '%s' -> '%s' (OK)", callsign, decoded)
		})
	}
}

// TestYSFVDMode2TestRoundTrip tests the built-in round trip test
func TestYSFVDMode2TestRoundTrip(t *testing.T) {
	vd := NewYSFVDMode2()

	testCases := []string{
		"KJ4ABC",
		"TEST123",
		"HAM",
		"1234567890",
	}

	for _, callsign := range testCases {
		if !vd.TestRoundTrip(callsign) {
			t.Errorf("Round trip test failed for callsign '%s'", callsign)
		}
	}
}

// TestYSFVDMode2Validation tests callsign validation
func TestYSFVDMode2Validation(t *testing.T) {
	vd := NewYSFVDMode2()

	validCallsigns := []string{
		"KJ4ABC",
		"VK2DEF",
		"TEST123",
		"HAM RADIO",
		"1234567890",
		"A",
		"ABC 123",
	}

	invalidCallsigns := []string{
		"toolongcallsign", // Too long
		"test@ham",        // Invalid character
		"ham-radio",       // Invalid character
		"call_sign",       // Invalid character
	}

	for _, callsign := range validCallsigns {
		if !vd.ValidateCallsign(callsign) {
			t.Errorf("Valid callsign '%s' failed validation", callsign)
		}
	}

	for _, callsign := range invalidCallsigns {
		if vd.ValidateCallsign(callsign) {
			t.Errorf("Invalid callsign '%s' passed validation", callsign)
		}
	}
}

// TestYSFVDMode2YSFPayloadExtraction tests YSF frame payload operations
func TestYSFVDMode2YSFPayloadExtraction(t *testing.T) {
	vd := NewYSFVDMode2()

	// Create a mock YSF frame
	frameSize := YSF_SYNC_LENGTH_BYTES + YSF_FICH_LENGTH_BYTES + (YSF_DCH_SECTIONS * YSF_DCH_SECTION_SIZE)
	mockFrame := make([]uint8, frameSize)

	// Fill with test pattern
	for i := range mockFrame {
		mockFrame[i] = uint8(i & 0xFF)
	}

	// Test encoding a callsign and inserting into frame
	callsign := "TEST123"
	encoded := vd.EncodeCallsign(callsign)

	if !vd.InsertIntoYSFPayload(mockFrame, encoded) {
		t.Fatal("Failed to insert VD Mode 2 data into YSF payload")
	}

	// Extract the data back
	extracted, ok := vd.ExtractFromYSFPayload(mockFrame)
	if !ok {
		t.Fatal("Failed to extract VD Mode 2 data from YSF payload")
	}

	// Verify extraction matches insertion
	if extracted != encoded {
		t.Error("Extracted data doesn't match inserted data")
		t.Logf("Inserted:  %X", encoded)
		t.Logf("Extracted: %X", extracted)
	}

	// Decode the extracted data
	decodedCallsign, ok := vd.DecodeCallsign(extracted)
	if !ok {
		t.Fatal("Failed to decode extracted callsign")
	}

	if decodedCallsign != callsign {
		t.Errorf("Decoded callsign mismatch: expected '%s', got '%s'", callsign, decodedCallsign)
	}

	t.Logf("YSF payload test: '%s' -> extract/decode -> '%s' (OK)", callsign, decodedCallsign)
}

// TestYSFVDMode2ErrorIntroduction tests behavior with corrupted data
func TestYSFVDMode2ErrorIntroduction(t *testing.T) {
	vd := NewYSFVDMode2()
	callsign := "KJ4ABC"

	// Encode callsign
	encoded := vd.EncodeCallsign(callsign)

	// Test with no errors (should decode successfully)
	decoded, ok := vd.DecodeCallsign(encoded)
	if !ok {
		t.Fatal("Clean encoded data failed to decode")
	}
	if decoded != callsign {
		t.Errorf("Clean decode mismatch: expected '%s', got '%s'", callsign, decoded)
	}

	// Introduce single bit errors and test detection
	errorCount := 0
	successCount := 0

	for bytePos := 0; bytePos < YSF_VD_MODE2_ENCODED_LENGTH; bytePos++ {
		for bitPos := 0; bitPos < 8; bitPos++ {
			corrupted := encoded
			corrupted[bytePos] ^= (1 << bitPos) // Flip single bit

			decoded, ok := vd.DecodeCallsign(corrupted)
			if !ok {
				errorCount++
			} else {
				successCount++
				if decoded != callsign {
					t.Logf("Bit error at [%d,%d] caused callsign change: '%s' -> '%s'",
						bytePos, bitPos, callsign, decoded)
				}
			}
		}
	}

	t.Logf("Single bit error test: %d detected errors, %d successful decodes", errorCount, successCount)

	// Introduce byte errors (more severe)
	severeErrorCount := 0
	for bytePos := 0; bytePos < YSF_VD_MODE2_ENCODED_LENGTH; bytePos++ {
		corrupted := encoded
		corrupted[bytePos] ^= 0xFF // Flip all bits in byte

		_, ok := vd.DecodeCallsign(corrupted)
		if !ok {
			severeErrorCount++
		}
	}

	t.Logf("Byte error test: %d out of %d severe errors detected", severeErrorCount, YSF_VD_MODE2_ENCODED_LENGTH)
}

// TestYSFVDMode2Constants tests the constants and tables
func TestYSFVDMode2Constants(t *testing.T) {
	// Test constant values
	if YSF_VD_MODE2_CALLSIGN_LENGTH != 10 {
		t.Errorf("CALLSIGN_LENGTH should be 10, got %d", YSF_VD_MODE2_CALLSIGN_LENGTH)
	}

	if YSF_VD_MODE2_DATA_LENGTH != 12 {
		t.Errorf("DATA_LENGTH should be 12, got %d", YSF_VD_MODE2_DATA_LENGTH)
	}

	if YSF_VD_MODE2_ENCODED_LENGTH != 25 {
		t.Errorf("ENCODED_LENGTH should be 25, got %d", YSF_VD_MODE2_ENCODED_LENGTH)
	}

	if YSF_VD_MODE2_INFO_BITS != 96 {
		t.Errorf("INFO_BITS should be 96, got %d", YSF_VD_MODE2_INFO_BITS)
	}

	if YSF_VD_MODE2_ENCODED_BITS != 200 {
		t.Errorf("ENCODED_BITS should be 200, got %d", YSF_VD_MODE2_ENCODED_BITS)
	}

	if YSF_VD_MODE2_INTERLEAVE_SIZE != 100 {
		t.Errorf("INTERLEAVE_SIZE should be 100, got %d", YSF_VD_MODE2_INTERLEAVE_SIZE)
	}

	// Test table sizes
	if len(YSF_VD_MODE2_INTERLEAVE_TABLE) != YSF_VD_MODE2_INTERLEAVE_SIZE {
		t.Errorf("INTERLEAVE_TABLE size should be %d, got %d",
			YSF_VD_MODE2_INTERLEAVE_SIZE, len(YSF_VD_MODE2_INTERLEAVE_TABLE))
	}

	if len(YSF_VD_MODE2_WHITENING_DATA) != 20 {
		t.Errorf("WHITENING_DATA size should be 20, got %d", len(YSF_VD_MODE2_WHITENING_DATA))
	}

	if len(YSF_VD_MODE2_BIT_MASK_TABLE) != 8 {
		t.Errorf("BIT_MASK_TABLE size should be 8, got %d", len(YSF_VD_MODE2_BIT_MASK_TABLE))
	}

	// Test specific interleave table values (spot check)
	expectedFirst5 := [5]uint32{0, 40, 80, 120, 160}
	for i := 0; i < 5; i++ {
		if YSF_VD_MODE2_INTERLEAVE_TABLE[i] != expectedFirst5[i] {
			t.Errorf("INTERLEAVE_TABLE[%d] should be %d, got %d",
				i, expectedFirst5[i], YSF_VD_MODE2_INTERLEAVE_TABLE[i])
		}
	}
}

// TestYSFVDMode2GettersAndHelpers tests getter methods and helper functions
func TestYSFVDMode2GettersAndHelpers(t *testing.T) {
	vd := NewYSFVDMode2()

	// Test getter methods
	if vd.GetDataLength() != YSF_VD_MODE2_CALLSIGN_LENGTH {
		t.Errorf("GetDataLength() should return %d, got %d",
			YSF_VD_MODE2_CALLSIGN_LENGTH, vd.GetDataLength())
	}

	if vd.GetEncodedLength() != YSF_VD_MODE2_ENCODED_LENGTH {
		t.Errorf("GetEncodedLength() should return %d, got %d",
			YSF_VD_MODE2_ENCODED_LENGTH, vd.GetEncodedLength())
	}

	// Test pattern getters
	interleavePattern := vd.GetInterleavePattern()
	if len(interleavePattern) != YSF_VD_MODE2_INTERLEAVE_SIZE {
		t.Errorf("GetInterleavePattern() size should be %d, got %d",
			YSF_VD_MODE2_INTERLEAVE_SIZE, len(interleavePattern))
	}

	whiteningPattern := vd.GetWhiteningPattern()
	if len(whiteningPattern) != 20 {
		t.Errorf("GetWhiteningPattern() size should be 20, got %d", len(whiteningPattern))
	}

	// Verify patterns match global constants
	for i := 0; i < YSF_VD_MODE2_INTERLEAVE_SIZE; i++ {
		if interleavePattern[i] != YSF_VD_MODE2_INTERLEAVE_TABLE[i] {
			t.Errorf("GetInterleavePattern()[%d] mismatch", i)
		}
	}

	for i := 0; i < 20; i++ {
		if whiteningPattern[i] != YSF_VD_MODE2_WHITENING_DATA[i] {
			t.Errorf("GetWhiteningPattern()[%d] mismatch", i)
		}
	}
}

// TestYSFVDMode2EdgeCases tests edge cases and boundary conditions
func TestYSFVDMode2EdgeCases(t *testing.T) {
	vd := NewYSFVDMode2()

	// Test empty callsign
	if vd.ValidateCallsign("") {
		t.Error("Empty callsign should fail validation")
	}

	// Test maximum length callsign
	maxCallsign := "1234567890" // Exactly 10 characters
	if !vd.ValidateCallsign(maxCallsign) {
		t.Error("Maximum length callsign should pass validation")
	}

	// Test oversized callsign
	oversizeCallsign := "12345678901" // 11 characters
	if vd.ValidateCallsign(oversizeCallsign) {
		t.Error("Oversize callsign should fail validation")
	}

	// Test YSF payload operations with insufficient data
	shortFrame := make([]uint8, 10) // Too short
	encoded := vd.EncodeCallsign("TEST")

	if vd.InsertIntoYSFPayload(shortFrame, encoded) {
		t.Error("Insert into short frame should fail")
	}

	_, ok := vd.ExtractFromYSFPayload(shortFrame)
	if ok {
		t.Error("Extract from short frame should fail")
	}

	// Test decode with corrupted length
	corruptedEncoded := [YSF_VD_MODE2_ENCODED_LENGTH]uint8{}
	_, ok = vd.DecodeCallsign(corruptedEncoded)
	// This may succeed or fail depending on CRC, both are acceptable
	t.Logf("Decode of zero data: success=%t", ok)
}

// TestYSFVDMode2BitManipulation tests internal bit manipulation functions
func TestYSFVDMode2BitManipulation(t *testing.T) {
	vd := NewYSFVDMode2()

	// Test bit writing and reading
	data := make([]uint8, 4)

	testBits := []struct {
		pos uint32
		bit bool
	}{
		{0, true},
		{1, false},
		{7, true},
		{8, false},
		{15, true},
		{24, false},
		{31, true},
	}

	// Write bits
	for _, test := range testBits {
		vd.writeBit(data, test.pos, test.bit)
	}

	// Read bits back
	for _, test := range testBits {
		result := vd.readBit(data, test.pos)
		if result != test.bit {
			t.Errorf("Bit %d: wrote %t, read %t", test.pos, test.bit, result)
		}
	}

	t.Logf("Bit manipulation test data: [%02X %02X %02X %02X]",
		data[0], data[1], data[2], data[3])
}

// BenchmarkYSFVDMode2Encode benchmarks callsign encoding performance
func BenchmarkYSFVDMode2Encode(b *testing.B) {
	vd := NewYSFVDMode2()
	callsign := "KJ4ABC"

	for i := 0; i < b.N; i++ {
		vd.EncodeCallsign(callsign)
	}
}

// BenchmarkYSFVDMode2Decode benchmarks callsign decoding performance
func BenchmarkYSFVDMode2Decode(b *testing.B) {
	vd := NewYSFVDMode2()
	callsign := "KJ4ABC"

	// Pre-encode the callsign
	encoded := vd.EncodeCallsign(callsign)

	for i := 0; i < b.N; i++ {
		vd.DecodeCallsign(encoded)
	}
}

// BenchmarkYSFVDMode2RoundTrip benchmarks complete round trip
func BenchmarkYSFVDMode2RoundTrip(b *testing.B) {
	vd := NewYSFVDMode2()
	callsign := "KJ4ABC"

	for i := 0; i < b.N; i++ {
		encoded := vd.EncodeCallsign(callsign)
		vd.DecodeCallsign(encoded)
	}
}

// BenchmarkYSFVDMode2YSFPayload benchmarks YSF payload operations
func BenchmarkYSFVDMode2YSFPayload(b *testing.B) {
	vd := NewYSFVDMode2()
	callsign := "KJ4ABC"
	encoded := vd.EncodeCallsign(callsign)

	frameSize := YSF_SYNC_LENGTH_BYTES + YSF_FICH_LENGTH_BYTES + (YSF_DCH_SECTIONS * YSF_DCH_SECTION_SIZE)
	frame := make([]uint8, frameSize)

	for i := 0; i < b.N; i++ {
		vd.InsertIntoYSFPayload(frame, encoded)
		vd.ExtractFromYSFPayload(frame)
	}
}