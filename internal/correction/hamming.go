package correction

import "fmt"

// boolXOR performs XOR operation on multiple boolean values
func boolXOR(values ...bool) bool {
	result := false
	for _, v := range values {
		result = result != v // Boolean XOR
	}
	return result
}

// Hamming (15,11,3) variant 1 encoding
func Encode15113_1(data []bool) error {
	if len(data) != 15 {
		return fmt.Errorf("invalid data length for Hamming (15,11,3): got %d, want 15", len(data))
	}

	// Calculate parity bits for positions 11-14
	data[11] = boolXOR(data[0], data[1], data[2], data[3], data[4], data[5], data[6])
	data[12] = boolXOR(data[0], data[1], data[2], data[3], data[7], data[8], data[9])
	data[13] = boolXOR(data[0], data[1], data[4], data[5], data[7], data[8], data[10])
	data[14] = boolXOR(data[0], data[2], data[4], data[6], data[7], data[9], data[10])

	return nil
}

// Hamming (15,11,3) variant 1 decoding with error correction
func Decode15113_1(data []bool) bool {
	if len(data) != 15 {
		return false
	}

	// Calculate expected parity bits
	c0 := boolXOR(data[0], data[1], data[2], data[3], data[4], data[5], data[6])
	c1 := boolXOR(data[0], data[1], data[2], data[3], data[7], data[8], data[9])
	c2 := boolXOR(data[0], data[1], data[4], data[5], data[7], data[8], data[10])
	c3 := boolXOR(data[0], data[2], data[4], data[6], data[7], data[9], data[10])

	// Build syndrome
	var syndrome uint8
	if c0 != data[11] {
		syndrome |= 0x01
	}
	if c1 != data[12] {
		syndrome |= 0x02
	}
	if c2 != data[13] {
		syndrome |= 0x04
	}
	if c3 != data[14] {
		syndrome |= 0x08
	}

	switch syndrome {
	// No errors
	case 0x00:
		return true

	// Parity bit errors
	case 0x01:
		data[11] = !data[11]
		return true
	case 0x02:
		data[12] = !data[12]
		return true
	case 0x04:
		data[13] = !data[13]
		return true
	case 0x08:
		data[14] = !data[14]
		return true

	// Data bit errors
	case 0x0F:
		data[0] = !data[0]
		return true
	case 0x07:
		data[1] = !data[1]
		return true
	case 0x0B:
		data[2] = !data[2]
		return true
	case 0x03:
		data[3] = !data[3]
		return true
	case 0x0D:
		data[4] = !data[4]
		return true
	case 0x05:
		data[5] = !data[5]
		return true
	case 0x09:
		data[6] = !data[6]
		return true
	case 0x0E:
		data[7] = !data[7]
		return true
	case 0x06:
		data[8] = !data[8]
		return true
	case 0x0A:
		data[9] = !data[9]
		return true
	case 0x0C:
		data[10] = !data[10]
		return true

	// Multiple bit errors - can't correct
	default:
		return false
	}
}

// Hamming (15,11,3) variant 2 encoding
func Encode15113_2(data []bool) error {
	if len(data) != 15 {
		return fmt.Errorf("invalid data length for Hamming (15,11,3): got %d, want 15", len(data))
	}

	// Calculate parity bits for positions 11-14
	data[11] = boolXOR(data[0], data[1], data[2], data[3], data[5], data[7], data[8])
	data[12] = boolXOR(data[1], data[2], data[3], data[4], data[6], data[8], data[9])
	data[13] = boolXOR(data[2], data[3], data[4], data[5], data[7], data[9], data[10])
	data[14] = boolXOR(data[0], data[1], data[2], data[4], data[6], data[7], data[10])

	return nil
}

// Hamming (15,11,3) variant 2 decoding with error correction
func Decode15113_2(data []bool) bool {
	if len(data) != 15 {
		return false
	}

	// Calculate expected parity bits
	c0 := boolXOR(data[0], data[1], data[2], data[3], data[5], data[7], data[8])
	c1 := boolXOR(data[1], data[2], data[3], data[4], data[6], data[8], data[9])
	c2 := boolXOR(data[2], data[3], data[4], data[5], data[7], data[9], data[10])
	c3 := boolXOR(data[0], data[1], data[2], data[4], data[6], data[7], data[10])

	// Build syndrome
	var syndrome uint8
	if c0 != data[11] {
		syndrome |= 0x01
	}
	if c1 != data[12] {
		syndrome |= 0x02
	}
	if c2 != data[13] {
		syndrome |= 0x04
	}
	if c3 != data[14] {
		syndrome |= 0x08
	}

	switch syndrome {
	// No errors
	case 0x00:
		return true

	// Parity bit errors
	case 0x01:
		data[11] = !data[11]
		return true
	case 0x02:
		data[12] = !data[12]
		return true
	case 0x04:
		data[13] = !data[13]
		return true
	case 0x08:
		data[14] = !data[14]
		return true

	// Data bit errors
	case 0x09:
		data[0] = !data[0]
		return true
	case 0x0B:
		data[1] = !data[1]
		return true
	case 0x0F:
		data[2] = !data[2]
		return true
	case 0x07:
		data[3] = !data[3]
		return true
	case 0x0E:
		data[4] = !data[4]
		return true
	case 0x05:
		data[5] = !data[5]
		return true
	case 0x0A:
		data[6] = !data[6]
		return true
	case 0x0D:
		data[7] = !data[7]
		return true
	case 0x03:
		data[8] = !data[8]
		return true
	case 0x06:
		data[9] = !data[9]
		return true
	case 0x0C:
		data[10] = !data[10]
		return true

	// Multiple bit errors - can't correct
	default:
		return false
	}
}

// Hamming (13,9,3) encoding
func Encode1393(data []bool) error {
	if len(data) != 13 {
		return fmt.Errorf("invalid data length for Hamming (13,9,3): got %d, want 13", len(data))
	}

	// Calculate parity bits for positions 9-12
	data[9] = boolXOR(data[0], data[1], data[3], data[5], data[6])
	data[10] = boolXOR(data[0], data[1], data[2], data[4], data[6], data[7])
	data[11] = boolXOR(data[0], data[1], data[2], data[3], data[5], data[7], data[8])
	data[12] = boolXOR(data[0], data[2], data[4], data[5], data[8])

	return nil
}

// Hamming (13,9,3) decoding with error correction
func Decode1393(data []bool) bool {
	if len(data) != 13 {
		return false
	}

	// Calculate expected parity bits
	c0 := boolXOR(data[0], data[1], data[3], data[5], data[6])
	c1 := boolXOR(data[0], data[1], data[2], data[4], data[6], data[7])
	c2 := boolXOR(data[0], data[1], data[2], data[3], data[5], data[7], data[8])
	c3 := boolXOR(data[0], data[2], data[4], data[5], data[8])

	// Build syndrome
	var syndrome uint8
	if c0 != data[9] {
		syndrome |= 0x01
	}
	if c1 != data[10] {
		syndrome |= 0x02
	}
	if c2 != data[11] {
		syndrome |= 0x04
	}
	if c3 != data[12] {
		syndrome |= 0x08
	}

	switch syndrome {
	// No errors
	case 0x00:
		return true

	// Parity bit errors
	case 0x01:
		data[9] = !data[9]
		return true
	case 0x02:
		data[10] = !data[10]
		return true
	case 0x04:
		data[11] = !data[11]
		return true
	case 0x08:
		data[12] = !data[12]
		return true

	// Data bit errors
	case 0x0F:
		data[0] = !data[0]
		return true
	case 0x07:
		data[1] = !data[1]
		return true
	case 0x0E:
		data[2] = !data[2]
		return true
	case 0x05:
		data[3] = !data[3]
		return true
	case 0x0A:
		data[4] = !data[4]
		return true
	case 0x0D:
		data[5] = !data[5]
		return true
	case 0x03:
		data[6] = !data[6]
		return true
	case 0x06:
		data[7] = !data[7]
		return true
	case 0x0C:
		data[8] = !data[8]
		return true

	// Multiple bit errors - can't correct
	default:
		return false
	}
}

// Hamming (10,6,3) encoding
func Encode1063(data []bool) error {
	if len(data) != 10 {
		return fmt.Errorf("invalid data length for Hamming (10,6,3): got %d, want 10", len(data))
	}

	// Calculate parity bits for positions 6-9
	data[6] = boolXOR(data[0], data[1], data[2], data[5])
	data[7] = boolXOR(data[0], data[1], data[3], data[5])
	data[8] = boolXOR(data[0], data[2], data[3], data[4])
	data[9] = boolXOR(data[1], data[2], data[3], data[4])

	return nil
}

// Hamming (10,6,3) decoding with error correction
func Decode1063(data []bool) bool {
	if len(data) != 10 {
		return false
	}

	// Calculate expected parity bits
	c0 := boolXOR(data[0], data[1], data[2], data[5])
	c1 := boolXOR(data[0], data[1], data[3], data[5])
	c2 := boolXOR(data[0], data[2], data[3], data[4])
	c3 := boolXOR(data[1], data[2], data[3], data[4])

	// Build syndrome
	var syndrome uint8
	if c0 != data[6] {
		syndrome |= 0x01
	}
	if c1 != data[7] {
		syndrome |= 0x02
	}
	if c2 != data[8] {
		syndrome |= 0x04
	}
	if c3 != data[9] {
		syndrome |= 0x08
	}

	switch syndrome {
	// No errors
	case 0x00:
		return true

	// Parity bit errors
	case 0x01:
		data[6] = !data[6]
		return true
	case 0x02:
		data[7] = !data[7]
		return true
	case 0x04:
		data[8] = !data[8]
		return true
	case 0x08:
		data[9] = !data[9]
		return true

	// Data bit errors
	case 0x07:
		data[0] = !data[0]
		return true
	case 0x0B:
		data[1] = !data[1]
		return true
	case 0x0D:
		data[2] = !data[2]
		return true
	case 0x0E:
		data[3] = !data[3]
		return true
	case 0x0C:
		data[4] = !data[4]
		return true
	case 0x03:
		data[5] = !data[5]
		return true

	// Multiple bit errors - can't correct
	default:
		return false
	}
}

// Hamming (16,11,4) encoding
func Encode16114(data []bool) error {
	if len(data) != 16 {
		return fmt.Errorf("invalid data length for Hamming (16,11,4): got %d, want 16", len(data))
	}

	// Calculate parity bits for positions 11-15
	data[11] = boolXOR(data[0], data[1], data[2], data[3], data[5], data[7], data[8])
	data[12] = boolXOR(data[1], data[2], data[3], data[4], data[6], data[8], data[9])
	data[13] = boolXOR(data[2], data[3], data[4], data[5], data[7], data[9], data[10])
	data[14] = boolXOR(data[0], data[1], data[2], data[4], data[6], data[7], data[10])
	data[15] = boolXOR(data[0], data[2], data[5], data[6], data[8], data[9], data[10])

	return nil
}

// Hamming (16,11,4) decoding with error correction
func Decode16114(data []bool) bool {
	if len(data) != 16 {
		return false
	}

	// Calculate expected parity bits
	c0 := boolXOR(data[0], data[1], data[2], data[3], data[5], data[7], data[8])
	c1 := boolXOR(data[1], data[2], data[3], data[4], data[6], data[8], data[9])
	c2 := boolXOR(data[2], data[3], data[4], data[5], data[7], data[9], data[10])
	c3 := boolXOR(data[0], data[1], data[2], data[4], data[6], data[7], data[10])
	c4 := boolXOR(data[0], data[2], data[5], data[6], data[8], data[9], data[10])

	// Build syndrome
	var syndrome uint8
	if c0 != data[11] {
		syndrome |= 0x01
	}
	if c1 != data[12] {
		syndrome |= 0x02
	}
	if c2 != data[13] {
		syndrome |= 0x04
	}
	if c3 != data[14] {
		syndrome |= 0x08
	}
	if c4 != data[15] {
		syndrome |= 0x10
	}

	switch syndrome {
	// No errors
	case 0x00:
		return true

	// Parity bit errors
	case 0x01:
		data[11] = !data[11]
		return true
	case 0x02:
		data[12] = !data[12]
		return true
	case 0x04:
		data[13] = !data[13]
		return true
	case 0x08:
		data[14] = !data[14]
		return true
	case 0x10:
		data[15] = !data[15]
		return true

	// Data bit errors
	case 0x19:
		data[0] = !data[0]
		return true
	case 0x0B:
		data[1] = !data[1]
		return true
	case 0x1F:
		data[2] = !data[2]
		return true
	case 0x07:
		data[3] = !data[3]
		return true
	case 0x0E:
		data[4] = !data[4]
		return true
	case 0x15:
		data[5] = !data[5]
		return true
	case 0x1A:
		data[6] = !data[6]
		return true
	case 0x0D:
		data[7] = !data[7]
		return true
	case 0x13:
		data[8] = !data[8]
		return true
	case 0x16:
		data[9] = !data[9]
		return true
	case 0x1C:
		data[10] = !data[10]
		return true

	// Multiple bit errors - can't correct
	default:
		return false
	}
}

// Hamming (17,12,3) encoding
func Encode17123(data []bool) error {
	if len(data) != 17 {
		return fmt.Errorf("invalid data length for Hamming (17,12,3): got %d, want 17", len(data))
	}

	// Calculate parity bits for positions 12-16
	data[12] = boolXOR(data[0], data[1], data[2], data[3], data[6], data[7], data[9])
	data[13] = boolXOR(data[0], data[1], data[2], data[3], data[4], data[7], data[8], data[10])
	data[14] = boolXOR(data[1], data[2], data[3], data[4], data[5], data[8], data[9], data[11])
	data[15] = boolXOR(data[0], data[1], data[4], data[5], data[7], data[10])
	data[16] = boolXOR(data[0], data[2], data[5], data[6], data[8], data[11])

	return nil
}

// Hamming (17,12,3) decoding with error correction
func Decode17123(data []bool) bool {
	if len(data) != 17 {
		return false
	}

	// Calculate expected parity bits
	c0 := boolXOR(data[0], data[1], data[2], data[3], data[6], data[7], data[9])
	c1 := boolXOR(data[0], data[1], data[2], data[3], data[4], data[7], data[8], data[10])
	c2 := boolXOR(data[1], data[2], data[3], data[4], data[5], data[8], data[9], data[11])
	c3 := boolXOR(data[0], data[1], data[4], data[5], data[7], data[10])
	c4 := boolXOR(data[0], data[2], data[5], data[6], data[8], data[11])

	// Build syndrome
	var syndrome uint8
	if c0 != data[12] {
		syndrome |= 0x01
	}
	if c1 != data[13] {
		syndrome |= 0x02
	}
	if c2 != data[14] {
		syndrome |= 0x04
	}
	if c3 != data[15] {
		syndrome |= 0x08
	}
	if c4 != data[16] {
		syndrome |= 0x10
	}

	switch syndrome {
	// No errors
	case 0x00:
		return true

	// Parity bit errors
	case 0x01:
		data[12] = !data[12]
		return true
	case 0x02:
		data[13] = !data[13]
		return true
	case 0x04:
		data[14] = !data[14]
		return true
	case 0x08:
		data[15] = !data[15]
		return true
	case 0x10:
		data[16] = !data[16]
		return true

	// Data bit errors (patterns from original C++ code)
	case 0x1B:
		data[0] = !data[0]
		return true
	case 0x1F:
		data[1] = !data[1]
		return true
	case 0x17:
		data[2] = !data[2]
		return true
	case 0x07:
		data[3] = !data[3]
		return true
	case 0x0E:
		data[4] = !data[4]
		return true
	case 0x1C:
		data[5] = !data[5]
		return true
	case 0x11:
		data[6] = !data[6]
		return true
	case 0x0B:
		data[7] = !data[7]
		return true
	case 0x16:
		data[8] = !data[8]
		return true
	case 0x05:
		data[9] = !data[9]
		return true
	case 0x0A:
		data[10] = !data[10]
		return true
	case 0x14:
		data[11] = !data[11]
		return true

	// Multiple bit errors - can't correct
	default:
		return false
	}
}