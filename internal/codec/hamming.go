package codec

// Hamming error correction codes for BPTC and other applications
// This matches the C++ CHamming class functionality

// boolXOR performs XOR operation on multiple boolean values
func boolXOR(values ...bool) bool {
	result := false
	for _, v := range values {
		result = (result && !v) || (!result && v) // XOR operation
	}
	return result
}

// Encode15113_2 encodes data using Hamming(15,11,3) variant 2
// Input: 15-bit array where bits 0-10 are data, bits 11-14 will be set as parity
func Encode15113_2(d []bool) {
	if len(d) < 15 {
		return
	}

	// Calculate the checksum this row should have
	// Based on C++ CHamming::encode15113_2()
	d[11] = boolXOR(d[0], d[1], d[2], d[3], d[5], d[7], d[8])
	d[12] = boolXOR(d[1], d[2], d[3], d[4], d[6], d[8], d[9])
	d[13] = boolXOR(d[2], d[3], d[4], d[5], d[7], d[9], d[10])
	d[14] = boolXOR(d[0], d[1], d[2], d[4], d[6], d[7], d[10])
}

// Decode15113_2 checks and corrects errors using Hamming(15,11,3) variant 2
// Input: 15-bit array with data and parity bits
// Output: true if error was detected and corrected, false if no error
func Decode15113_2(d []bool) bool {
	if len(d) < 15 {
		return false
	}

	// Calculate the checksum this row should have
	// Based on C++ CHamming::decode15113_2()
	c0 := boolXOR(d[0], d[1], d[2], d[3], d[5], d[7], d[8])
	c1 := boolXOR(d[1], d[2], d[3], d[4], d[6], d[8], d[9])
	c2 := boolXOR(d[2], d[3], d[4], d[5], d[7], d[9], d[10])
	c3 := boolXOR(d[0], d[1], d[2], d[4], d[6], d[7], d[10])

	var n uint8
	if c0 != d[11] { n |= 0x01 }
	if c1 != d[12] { n |= 0x02 }
	if c2 != d[13] { n |= 0x04 }
	if c3 != d[14] { n |= 0x08 }

	switch n {
	// Parity bit errors
	case 0x01: d[11] = !d[11]; return true
	case 0x02: d[12] = !d[12]; return true
	case 0x04: d[13] = !d[13]; return true
	case 0x08: d[14] = !d[14]; return true

	// Data bit errors
	case 0x09: d[0] = !d[0]; return true
	case 0x0B: d[1] = !d[1]; return true
	case 0x0F: d[2] = !d[2]; return true
	case 0x07: d[3] = !d[3]; return true
	case 0x0E: d[4] = !d[4]; return true
	case 0x05: d[5] = !d[5]; return true
	case 0x0A: d[6] = !d[6]; return true
	case 0x0D: d[7] = !d[7]; return true
	case 0x03: d[8] = !d[8]; return true
	case 0x06: d[9] = !d[9]; return true
	case 0x0C: d[10] = !d[10]; return true

	// No bit errors
	default: return false
	}
}

// Encode1393 encodes data using Hamming(13,9,3)
// Input: 13-bit array where bits 0-8 are data, bits 9-12 will be set as parity
func Encode1393(d []bool) {
	if len(d) < 13 {
		return
	}

	// Calculate the checksum this column should have
	// Based on C++ CHamming::encode1393()
	d[9] = boolXOR(d[0], d[1], d[3], d[5], d[6])
	d[10] = boolXOR(d[0], d[1], d[2], d[4], d[6], d[7])
	d[11] = boolXOR(d[0], d[1], d[2], d[3], d[5], d[7], d[8])
	d[12] = boolXOR(d[0], d[2], d[4], d[5], d[8])
}

// Decode1393 checks and corrects errors using Hamming(13,9,3)
// Input: 13-bit array with data and parity bits
// Output: true if error was detected and corrected, false if no error
func Decode1393(d []bool) bool {
	if len(d) < 13 {
		return false
	}

	// Calculate the checksum this column should have
	// Based on C++ CHamming::decode1393()
	c0 := boolXOR(d[0], d[1], d[3], d[5], d[6])
	c1 := boolXOR(d[0], d[1], d[2], d[4], d[6], d[7])
	c2 := boolXOR(d[0], d[1], d[2], d[3], d[5], d[7], d[8])
	c3 := boolXOR(d[0], d[2], d[4], d[5], d[8])

	var n uint8
	if c0 != d[9] { n |= 0x01 }
	if c1 != d[10] { n |= 0x02 }
	if c2 != d[11] { n |= 0x04 }
	if c3 != d[12] { n |= 0x08 }

	switch n {
	// Parity bit errors
	case 0x01: d[9] = !d[9]; return true
	case 0x02: d[10] = !d[10]; return true
	case 0x04: d[11] = !d[11]; return true
	case 0x08: d[12] = !d[12]; return true

	// Data bit errors
	case 0x0F: d[0] = !d[0]; return true
	case 0x07: d[1] = !d[1]; return true
	case 0x0E: d[2] = !d[2]; return true
	case 0x05: d[3] = !d[3]; return true
	case 0x0A: d[4] = !d[4]; return true
	case 0x0D: d[5] = !d[5]; return true
	case 0x03: d[6] = !d[6]; return true
	case 0x06: d[7] = !d[7]; return true
	case 0x0C: d[8] = !d[8]; return true

	// No bit errors
	default: return false
	}
}

// ByteToBitsBE converts a byte to 8 bits in big-endian order
// Input: byte value
// Output: 8-bit boolean array (bits[0] = MSB, bits[7] = LSB)
func ByteToBitsBE(b uint8, bits []bool) {
	if len(bits) < 8 {
		return
	}

	bits[0] = (b & 0x80) != 0
	bits[1] = (b & 0x40) != 0
	bits[2] = (b & 0x20) != 0
	bits[3] = (b & 0x10) != 0
	bits[4] = (b & 0x08) != 0
	bits[5] = (b & 0x04) != 0
	bits[6] = (b & 0x02) != 0
	bits[7] = (b & 0x01) != 0
}

// BitsToByteBE converts 8 bits to a byte in big-endian order
// Input: 8-bit boolean array (bits[0] = MSB, bits[7] = LSB)
// Output: byte value
func BitsToByteBE(bits []bool) uint8 {
	if len(bits) < 8 {
		return 0
	}

	var b uint8
	if bits[0] { b |= 0x80 }
	if bits[1] { b |= 0x40 }
	if bits[2] { b |= 0x20 }
	if bits[3] { b |= 0x10 }
	if bits[4] { b |= 0x08 }
	if bits[5] { b |= 0x04 }
	if bits[6] { b |= 0x02 }
	if bits[7] { b |= 0x01 }

	return b
}

// ValidateHamming tests the Hamming code implementations
func ValidateHamming() bool {
	// Test Hamming(15,11,3) variant 2
	testData15113 := make([]bool, 15)

	// Set test pattern in data bits
	testData15113[0] = true
	testData15113[1] = false
	testData15113[2] = true
	testData15113[3] = false
	testData15113[4] = true
	testData15113[5] = false
	testData15113[6] = true
	testData15113[7] = false
	testData15113[8] = true
	testData15113[9] = false
	testData15113[10] = true

	// Encode
	Encode15113_2(testData15113)

	// Should decode without error
	if Decode15113_2(testData15113) {
		return false // Should not detect error in valid codeword
	}

	// Introduce single bit error and test correction
	testData15113[5] = !testData15113[5] // Flip bit 5
	if !Decode15113_2(testData15113) {
		return false // Should detect and correct error
	}

	// Test Hamming(13,9,3)
	testData1393 := make([]bool, 13)

	// Set test pattern in data bits
	testData1393[0] = true
	testData1393[1] = true
	testData1393[2] = false
	testData1393[3] = true
	testData1393[4] = false
	testData1393[5] = false
	testData1393[6] = true
	testData1393[7] = true
	testData1393[8] = false

	// Encode
	Encode1393(testData1393)

	// Should decode without error
	if Decode1393(testData1393) {
		return false // Should not detect error in valid codeword
	}

	// Introduce single bit error and test correction
	testData1393[3] = !testData1393[3] // Flip bit 3
	if !Decode1393(testData1393) {
		return false // Should detect and correct error
	}

	return true
}