package correction

import "fmt"

// Golay (20,8,7) generator polynomial: x^12 + x^11 + x^10 + x^8 + x^5 + x^2 + 1
const GOLAY_20_8_GENERATOR = 0x1ED1

// Golay (24,12,8) generator polynomial: x^11 + x^10 + x^6 + x^5 + x^4 + x^2 + 1
const GOLAY_24_12_GENERATOR = 0xC75

// Golay2087Encode encodes 8-bit data into 20-bit Golay codeword
// Input: 3 bytes (24 bits total, using first 8 data bits)
// Output: Same 3 bytes with parity bits added
func Golay2087Encode(data []byte) error {
	if len(data) < 3 {
		return fmt.Errorf("invalid data length for Golay (20,8): need at least 3 bytes, got %d", len(data))
	}

	// Extract 8 data bits from first byte
	dataBits := uint32(data[0])

	// Calculate 12 parity bits using polynomial division
	// Shift data left by 12 positions
	shifted := dataBits << 12

	// Perform polynomial division
	parity := polyDiv20(shifted, GOLAY_20_8_GENERATOR)

	// Construct 20-bit codeword: 8 data bits + 12 parity bits
	codeword := (dataBits << 12) ^ parity

	// Store back in 3 bytes (24 bits total, 4 bits unused)
	data[0] = uint8((codeword >> 12) & 0xFF) // Data bits
	data[1] = uint8((codeword >> 4) & 0xFF)  // Parity bits [11:4]
	data[2] = uint8((codeword << 4) & 0xF0) | (data[2] & 0x0F) // Parity bits [3:0] + preserve lower 4 bits

	return nil
}

// Golay2087Decode decodes 20-bit Golay codeword and corrects errors
// Returns the number of errors detected/corrected
func Golay2087Decode(data []byte) uint8 {
	if len(data) < 3 {
		return 0xFF // Error indicator
	}

	// Extract 20-bit codeword from 3 bytes
	codeword := (uint32(data[0]) << 12) | (uint32(data[1]) << 4) | (uint32(data[2]) >> 4)

	// Calculate syndrome
	syndrome := polyDiv20(codeword, GOLAY_20_8_GENERATOR)

	if syndrome == 0 {
		return 0 // No errors
	}

	// Find error pattern
	errorPattern, errorCount := findGolayErrorPattern20(syndrome)

	if errorCount > 0 {
		// Apply correction
		corrected := codeword ^ errorPattern

		// Store corrected codeword back
		data[0] = uint8((corrected >> 12) & 0xFF)
		data[1] = uint8((corrected >> 4) & 0xFF)
		data[2] = (uint8(corrected << 4) & 0xF0) | (data[2] & 0x0F)
	}

	return errorCount
}

// Golay24128Encode encodes 12-bit data into 24-bit Golay codeword
// Input: 3 bytes (24 bits total, using first 12 data bits)
// Output: Same 3 bytes with parity bits added
func Golay24128Encode(data []byte) error {
	if len(data) < 3 {
		return fmt.Errorf("invalid data length for Golay (24,12): need at least 3 bytes, got %d", len(data))
	}

	// Extract 12 data bits from first 1.5 bytes
	dataBits := (uint32(data[0]) << 4) | (uint32(data[1]) >> 4)

	// Calculate 12 parity bits using polynomial division
	// Shift data left by 12 positions
	shifted := dataBits << 12

	// Perform polynomial division
	parity := polyDiv24(shifted, GOLAY_24_12_GENERATOR)

	// Construct 24-bit codeword: 12 data bits + 12 parity bits
	codeword := (dataBits << 12) ^ parity

	// Store back in 3 bytes
	data[0] = uint8((codeword >> 16) & 0xFF) // Data bits [11:4]
	data[1] = uint8((codeword >> 8) & 0xFF)  // Data bits [3:0] + Parity bits [11:8]
	data[2] = uint8(codeword & 0xFF)         // Parity bits [7:0]

	return nil
}

// Golay24128Decode decodes 24-bit Golay codeword and corrects errors
// Returns the number of errors detected/corrected
func Golay24128Decode(data []byte) uint8 {
	if len(data) < 3 {
		return 0xFF // Error indicator
	}

	// Extract 24-bit codeword from 3 bytes
	codeword := (uint32(data[0]) << 16) | (uint32(data[1]) << 8) | uint32(data[2])

	// Calculate syndrome
	syndrome := polyDiv24(codeword, GOLAY_24_12_GENERATOR)

	if syndrome == 0 {
		return 0 // No errors
	}

	// Find error pattern
	errorPattern, errorCount := findGolayErrorPattern24(syndrome)

	if errorCount > 0 {
		// Apply correction
		corrected := codeword ^ errorPattern

		// Store corrected codeword back
		data[0] = uint8((corrected >> 16) & 0xFF)
		data[1] = uint8((corrected >> 8) & 0xFF)
		data[2] = uint8(corrected & 0xFF)
	}

	return errorCount
}

// polyDiv20 performs polynomial division for 20-bit codeword
func polyDiv20(dividend, divisor uint32) uint32 {
	// Ensure we're working with 20-bit values
	dividend &= 0xFFFFF
	divisor &= 0xFFF

	remainder := dividend
	for i := 19; i >= 12; i-- {
		if (remainder & (1 << uint(i))) != 0 {
			remainder ^= divisor << uint(i-11)
		}
	}

	return remainder & 0xFFF
}

// polyDiv24 performs polynomial division for 24-bit codeword
func polyDiv24(dividend, divisor uint32) uint32 {
	// Ensure we're working with 24-bit values
	dividend &= 0xFFFFFF
	divisor &= 0xFFF

	remainder := dividend
	for i := 23; i >= 12; i-- {
		if (remainder & (1 << uint(i))) != 0 {
			remainder ^= divisor << uint(i-11)
		}
	}

	return remainder & 0xFFF
}

// findGolayErrorPattern20 finds error pattern for Golay (20,8) code
// Returns error pattern and number of errors
func findGolayErrorPattern20(syndrome uint32) (uint32, uint8) {
	if syndrome == 0 {
		return 0, 0
	}

	// Weight of syndrome
	syndromeWeight := popcount32(syndrome)

	// Single error patterns: weight(syndrome) <= 3
	if syndromeWeight <= 3 {
		// Error in parity bits
		return syndrome, uint8(syndromeWeight)
	}

	// Try error patterns with errors in data bits
	for i := uint(0); i < 8; i++ {
		// Error in data bit i
		errorPattern := uint32(1) << (12 + i)
		testSyndrome := polyDiv20(errorPattern, GOLAY_20_8_GENERATOR)

		// Check if this matches our syndrome (single error in data)
		if testSyndrome == syndrome {
			return errorPattern, 1
		}

		// Check for double error patterns (data bit + parity bit)
		for j := uint(0); j < 12; j++ {
			doublePattern := errorPattern | (uint32(1) << j)
			testSyndrome = polyDiv20(doublePattern, GOLAY_20_8_GENERATOR)
			if testSyndrome == syndrome {
				return doublePattern, 2
			}
		}

		// Check for triple error patterns
		for j := uint(0); j < 12; j++ {
			for k := uint(j + 1); k < 12; k++ {
				triplePattern := errorPattern | (uint32(1) << j) | (uint32(1) << k)
				testSyndrome = polyDiv20(triplePattern, GOLAY_20_8_GENERATOR)
				if testSyndrome == syndrome {
					return triplePattern, 3
				}
			}
		}
	}

	// Check for errors in parity bits only (2-3 errors)
	for i := uint(0); i < 12; i++ {
		for j := uint(i + 1); j < 12; j++ {
			doublePattern := (uint32(1) << i) | (uint32(1) << j)
			if doublePattern == syndrome {
				return doublePattern, 2
			}

			// Triple error in parity
			for k := uint(j + 1); k < 12; k++ {
				triplePattern := doublePattern | (uint32(1) << k)
				if triplePattern == syndrome {
					return triplePattern, 3
				}
			}
		}
	}

	// Uncorrectable error pattern
	return 0, 0xFF
}

// findGolayErrorPattern24 finds error pattern for Golay (24,12) code
// Returns error pattern and number of errors
func findGolayErrorPattern24(syndrome uint32) (uint32, uint8) {
	if syndrome == 0 {
		return 0, 0
	}

	// Weight of syndrome
	syndromeWeight := popcount32(syndrome)

	// Single error patterns: weight(syndrome) <= 3
	if syndromeWeight <= 3 {
		// Error in parity bits
		return syndrome, uint8(syndromeWeight)
	}

	// Try error patterns with errors in data bits
	for i := uint(0); i < 12; i++ {
		// Error in data bit i
		errorPattern := uint32(1) << (12 + i)
		testSyndrome := polyDiv24(errorPattern, GOLAY_24_12_GENERATOR)

		// Check if this matches our syndrome (single error in data)
		if testSyndrome == syndrome {
			return errorPattern, 1
		}

		// Check for double error patterns (data bit + parity bit)
		for j := uint(0); j < 12; j++ {
			doublePattern := errorPattern | (uint32(1) << j)
			testSyndrome = polyDiv24(doublePattern, GOLAY_24_12_GENERATOR)
			if testSyndrome == syndrome {
				return doublePattern, 2
			}
		}

		// Check for triple error patterns
		for j := uint(0); j < 12; j++ {
			for k := uint(j + 1); k < 12; k++ {
				triplePattern := errorPattern | (uint32(1) << j) | (uint32(1) << k)
				testSyndrome = polyDiv24(triplePattern, GOLAY_24_12_GENERATOR)
				if testSyndrome == syndrome {
					return triplePattern, 3
				}
			}
		}
	}

	// Check for errors between data bits
	for i := uint(12); i < 24; i++ {
		for j := uint(i + 1); j < 24; j++ {
			doublePattern := (uint32(1) << i) | (uint32(1) << j)
			testSyndrome := polyDiv24(doublePattern, GOLAY_24_12_GENERATOR)
			if testSyndrome == syndrome {
				return doublePattern, 2
			}

			// Triple errors
			for k := uint(j + 1); k < 24; k++ {
				triplePattern := doublePattern | (uint32(1) << k)
				testSyndrome = polyDiv24(triplePattern, GOLAY_24_12_GENERATOR)
				if testSyndrome == syndrome {
					return triplePattern, 3
				}
			}
		}
	}

	// Check for errors in parity bits only (2-3 errors)
	for i := uint(0); i < 12; i++ {
		for j := uint(i + 1); j < 12; j++ {
			doublePattern := (uint32(1) << i) | (uint32(1) << j)
			if doublePattern == syndrome {
				return doublePattern, 2
			}

			// Triple error in parity
			for k := uint(j + 1); k < 12; k++ {
				triplePattern := doublePattern | (uint32(1) << k)
				if triplePattern == syndrome {
					return triplePattern, 3
				}
			}
		}
	}

	// Uncorrectable error pattern
	return 0, 0xFF
}

// popcount32 counts the number of 1 bits in a 32-bit integer
func popcount32(x uint32) uint8 {
	var count uint8
	for x != 0 {
		count++
		x &= x - 1 // Remove the lowest set bit
	}
	return count
}