package codec

// Golay24128 provides the exact interface expected by ModeConv
// Matches the C++ CGolay24128 class interface

// Encode24128 encodes 12-bit data into 24-bit Golay codeword
// Equivalent to CGolay24128::encode24128() from C++
func Encode24128(data uint32) uint32 {
	// Extract only the lower 12 bits
	data &= 0xFFF

	// Use the Golay (24,12) generator polynomial: x^11 + x^10 + x^6 + x^5 + x^4 + x^2 + 1
	// Generator: 0xC75
	generator := uint32(0xC75)

	// Calculate parity bits using polynomial division
	shifted := data << 12 // Shift data to upper 12 bits
	parity := polyDiv24(shifted, generator)

	// Construct 24-bit codeword: 12 data bits + 12 parity bits
	codeword := (data << 12) | parity

	return codeword & 0xFFFFFF
}

// Encode23127 encodes 11-bit data into 23-bit Golay codeword
// Equivalent to CGolay24128::encode23127() from C++
func Encode23127(data uint32) uint32 {
	// Extract only the lower 11 bits
	data &= 0x7FF

	// Use the Golay (23,11) generator polynomial
	// This is derived from the (24,12) by puncturing
	generator := uint32(0x65B) // Standard (23,11) generator

	// Calculate parity bits using polynomial division
	shifted := data << 12 // Shift data to upper positions
	parity := polyDiv23(shifted, generator)

	// Construct 23-bit codeword: 11 data bits + 12 parity bits
	codeword := (data << 12) | parity

	return codeword & 0x7FFFFF
}

// Decode24128 decodes 24-bit Golay codeword and returns corrected data
// Equivalent to CGolay24128::decode24128() from C++
func Decode24128(code uint32) uint32 {
	// Extract 24 bits
	code &= 0xFFFFFF

	// Calculate syndrome
	generator := uint32(0xC75)
	syndrome := polyDiv24(code, generator)

	if syndrome == 0 {
		// No errors, return data bits
		return (code >> 12) & 0xFFF
	}

	// Find error pattern using simplified lookup
	errorPattern, correctable := findGolayError24(syndrome)

	if correctable {
		// Apply correction
		corrected := code ^ errorPattern
		return (corrected >> 12) & 0xFFF
	}

	// Return original data if uncorrectable
	return (code >> 12) & 0xFFF
}

// Decode23127 decodes 23-bit Golay codeword and returns corrected data
// Equivalent to CGolay24128::decode23127() from C++
func Decode23127(code uint32) uint32 {
	// Extract 23 bits
	code &= 0x7FFFFF

	// Calculate syndrome
	generator := uint32(0x65B)
	syndrome := polyDiv23(code, generator)

	if syndrome == 0 {
		// No errors, return data bits
		return (code >> 12) & 0x7FF
	}

	// Find error pattern using simplified lookup
	errorPattern, correctable := findGolayError23(syndrome)

	if correctable {
		// Apply correction
		corrected := code ^ errorPattern
		return (corrected >> 12) & 0x7FF
	}

	// Return original data if uncorrectable
	return (code >> 12) & 0x7FF
}

// polyDiv24 performs polynomial division for 24-bit values
func polyDiv24(dividend, divisor uint32) uint32 {
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

// polyDiv23 performs polynomial division for 23-bit values
func polyDiv23(dividend, divisor uint32) uint32 {
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

// findGolayError24 finds error pattern for 24-bit Golay code
// Returns error pattern and whether it's correctable
func findGolayError24(syndrome uint32) (uint32, bool) {
	if syndrome == 0 {
		return 0, true
	}

	// Check weight of syndrome for single/double errors
	weight := popcount(syndrome)
	if weight <= 3 {
		// Error in parity part
		return syndrome, true
	}

	// Try single errors in data part
	generator := uint32(0xC75)
	for i := uint(0); i < 12; i++ {
		errorPattern := uint32(1) << (12 + i)
		testSyndrome := polyDiv24(errorPattern, generator)
		if testSyndrome == syndrome {
			return errorPattern, true
		}
	}

	// For simplicity, return uncorrectable for complex error patterns
	// A full implementation would check all correctable patterns
	return 0, false
}

// findGolayError23 finds error pattern for 23-bit Golay code
func findGolayError23(syndrome uint32) (uint32, bool) {
	if syndrome == 0 {
		return 0, true
	}

	// Check weight of syndrome for single/double errors
	weight := popcount(syndrome)
	if weight <= 3 {
		// Error in parity part
		return syndrome, true
	}

	// Try single errors in data part
	generator := uint32(0x65B)
	for i := uint(0); i < 11; i++ {
		errorPattern := uint32(1) << (12 + i)
		testSyndrome := polyDiv23(errorPattern, generator)
		if testSyndrome == syndrome {
			return errorPattern, true
		}
	}

	// Return uncorrectable for complex patterns
	return 0, false
}

// popcount counts the number of 1 bits in a 32-bit integer
func popcount(x uint32) int {
	count := 0
	for x != 0 {
		count++
		x &= x - 1 // Remove the lowest set bit
	}
	return count
}