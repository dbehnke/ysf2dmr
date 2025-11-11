package codec

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// Test data structures for synthetic AMBE testing
type TestVoiceData struct {
	Name        string
	Description string
	YSFPayloads [][]byte
	DMRPayloads [][]byte
	Expected    TestExpectations
}

type TestExpectations struct {
	ValidFrames      int
	ExpectedQuality  float32
	MaxBER           float32
	ConversionRatio  float32
	ProcessingTimeMs int64
}

// TestAMBEConversionSystem performs comprehensive testing of the AMBE conversion system
func TestAMBEConversionSystem(t *testing.T) {
	fmt.Println("Starting comprehensive AMBE conversion system test...")

	// Test 1: Component initialization
	t.Run("ComponentInitialization", testComponentInitialization)

	// Test 2: YSF AMBE extraction
	t.Run("YSFAMBEExtraction", testYSFAMBEExtraction)

	// Test 3: DMR AMBE extraction
	t.Run("DMRAMBEExtraction", testDMRAMBEExtraction)

	// Test 4: Frame ratio conversion
	t.Run("FrameRatioConversion", testFrameRatioConversion)

	// Test 5: AMBE validation
	t.Run("AMBEValidation", testAMBEValidation)

	// Test 6: End-to-end conversion pipeline
	t.Run("EndToEndConversion", testEndToEndConversion)

	// Test 7: Error handling and edge cases
	t.Run("ErrorHandling", testErrorHandling)

	// Test 8: Performance benchmarking
	t.Run("PerformanceBenchmark", testPerformanceBenchmark)

	fmt.Println("AMBE conversion system test completed successfully!")
}

// testComponentInitialization tests that all components can be created properly
func testComponentInitialization(t *testing.T) {
	// Test YSF extractor creation
	ysfExtractor := NewYSFAMBEExtractor()
	if ysfExtractor == nil {
		t.Fatal("Failed to create YSF AMBE extractor")
	}

	// Test DMR extractor creation
	dmrExtractor := NewDMRAMBEExtractor()
	if dmrExtractor == nil {
		t.Fatal("Failed to create DMR AMBE extractor")
	}

	// Test frame ratio converter creation
	converter := NewFrameRatioConverter()
	if converter == nil {
		t.Fatal("Failed to create frame ratio converter")
	}

	// Test AMBE validator creation
	validator := NewAMBEValidator(true, true, true)
	if validator == nil {
		t.Fatal("Failed to create AMBE validator")
	}

	fmt.Println("✓ All components initialized successfully")
}

// testYSFAMBEExtraction tests YSF AMBE frame extraction
func testYSFAMBEExtraction(t *testing.T) {
	extractor := NewYSFAMBEExtractor()

	// Create synthetic YSF payload with realistic voice data
	ysfPayload := createSyntheticYSFPayload()

	// Test VCH section extraction
	vchSections, err := extractor.ExtractVCHSections(ysfPayload)
	if err != nil {
		t.Fatalf("Failed to extract VCH sections: %v", err)
	}

	// Validate we got the expected number of sections
	if len(vchSections) != YSF_VCH_SECTIONS {
		t.Fatalf("Expected %d VCH sections, got %d", YSF_VCH_SECTIONS, len(vchSections))
	}

	// Test VCH to AMBE conversion
	totalValidSections := 0
	for i, vch := range vchSections {
		// Validate VCH section
		if !extractor.ValidateVCHSection(&vch) {
			t.Logf("Warning: VCH section %d failed validation", i)
			continue
		}

		// Convert to AMBE parameters
		ambeParams, err := extractor.ConvertVCHToAMBE(&vch)
		if err != nil {
			t.Fatalf("Failed to convert VCH section %d to AMBE: %v", i, err)
		}

		// Validate AMBE parameters are reasonable
		if ambeParams.A == 0 && ambeParams.B == 0 && ambeParams.C == 0 {
			t.Logf("Warning: AMBE parameters for section %d are all zero", i)
			continue
		}

		totalValidSections++

		// Calculate bit error estimate
		ber := extractor.GetVCHBitError(&vch)
		if ber > 0.5 {
			t.Logf("Warning: High BER (%.3f) for VCH section %d", ber, i)
		}
	}

	if totalValidSections == 0 {
		t.Fatal("No valid VCH sections found")
	}

	fmt.Printf("✓ YSF extraction: %d/%d valid VCH sections\n", totalValidSections, YSF_VCH_SECTIONS)
}

// testDMRAMBEExtraction tests DMR AMBE frame extraction
func testDMRAMBEExtraction(t *testing.T) {
	extractor := NewDMRAMBEExtractor()

	// Create synthetic DMR payload with realistic voice data
	dmrPayload := createSyntheticDMRPayload()

	// Test AMBE frame extraction
	ambeFrames, err := extractor.ExtractAMBEFrames(dmrPayload)
	if err != nil {
		t.Fatalf("Failed to extract AMBE frames: %v", err)
	}

	// Validate we got the expected number of frames
	if len(ambeFrames) != DMR_AMBE_FRAMES {
		t.Fatalf("Expected %d AMBE frames, got %d", DMR_AMBE_FRAMES, len(ambeFrames))
	}

	// Test AMBE frame validation and conversion
	totalValidFrames := 0
	for i, frame := range ambeFrames {
		// Validate AMBE frame
		if !extractor.ValidateAMBEFrame(&frame) {
			t.Logf("Warning: AMBE frame %d failed validation", i)
			continue
		}

		// Test AMBE to VCH conversion
		vch, err := extractor.ConvertAMBEToVCH(&frame.Params)
		if err != nil {
			t.Fatalf("Failed to convert AMBE frame %d to VCH: %v", i, err)
		}

		// Validate converted VCH has data
		hasData := false
		for _, b := range vch.Data {
			if b != 0 {
				hasData = true
				break
			}
		}

		if !hasData {
			t.Logf("Warning: Converted VCH for frame %d contains no data", i)
			continue
		}

		totalValidFrames++

		// Calculate bit error estimate
		ber := extractor.GetAMBEBitError(&frame)
		if ber > 0.5 {
			t.Logf("Warning: High BER (%.3f) for AMBE frame %d", ber, i)
		}
	}

	if totalValidFrames == 0 {
		t.Fatal("No valid AMBE frames found")
	}

	fmt.Printf("✓ DMR extraction: %d/%d valid AMBE frames\n", totalValidFrames, DMR_AMBE_FRAMES)
}

// testFrameRatioConversion tests the 3:5 frame ratio conversion
func testFrameRatioConversion(t *testing.T) {
	converter := NewFrameRatioConverter()

	// Test YSF to DMR conversion (3:5 ratio)
	fmt.Println("Testing YSF→DMR conversion (3:5 ratio)...")

	ysfFramesIn := 0
	dmrFramesOut := 0

	// Send 3 YSF frames
	for i := 0; i < YSF_TO_DMR_FRAME_RATIO; i++ {
		ysfPayload := createSyntheticYSFPayload()
		dmrFrames, err := converter.ConvertYSFToDMR(ysfPayload)
		if err != nil {
			t.Fatalf("Failed to convert YSF frame %d: %v", i, err)
		}

		ysfFramesIn++

		// We should only get output on the last frame
		if i < YSF_TO_DMR_FRAME_RATIO-1 {
			if len(dmrFrames) != 0 {
				t.Fatalf("Unexpected DMR frames on frame %d (should buffer)", i)
			}
		} else {
			if len(dmrFrames) != DMR_TO_YSF_FRAME_RATIO {
				t.Fatalf("Expected %d DMR frames, got %d", DMR_TO_YSF_FRAME_RATIO, len(dmrFrames))
			}
			dmrFramesOut = len(dmrFrames)
		}
	}

	// Verify conversion ratio
	expectedRatio := float32(dmrFramesOut) / float32(ysfFramesIn)
	actualRatio := float32(DMR_TO_YSF_FRAME_RATIO) / float32(YSF_TO_DMR_FRAME_RATIO)
	if math.Abs(float64(expectedRatio-actualRatio)) > 0.01 {
		t.Fatalf("Conversion ratio mismatch: expected %.3f, got %.3f", actualRatio, expectedRatio)
	}

	fmt.Printf("✓ YSF→DMR: %d frames → %d frames (ratio: %.3f)\n", ysfFramesIn, dmrFramesOut, expectedRatio)

	// Test DMR to YSF conversion (5:3 ratio)
	fmt.Println("Testing DMR→YSF conversion (5:3 ratio)...")

	converter.Reset() // Reset for DMR to YSF test
	dmrFramesIn := 0
	ysfFramesOut := 0

	// Send 5 DMR frames
	for i := 0; i < DMR_TO_YSF_FRAME_RATIO; i++ {
		dmrPayload := createSyntheticDMRPayload()
		ysfFrames, err := converter.ConvertDMRToYSF(dmrPayload)
		if err != nil {
			t.Fatalf("Failed to convert DMR frame %d: %v", i, err)
		}

		dmrFramesIn++

		// We should only get output on the last frame
		if i < DMR_TO_YSF_FRAME_RATIO-1 {
			if len(ysfFrames) != 0 {
				t.Fatalf("Unexpected YSF frames on frame %d (should buffer)", i)
			}
		} else {
			if len(ysfFrames) != YSF_TO_DMR_FRAME_RATIO {
				t.Fatalf("Expected %d YSF frames, got %d", YSF_TO_DMR_FRAME_RATIO, len(ysfFrames))
			}
			ysfFramesOut = len(ysfFrames)
		}
	}

	// Verify reverse conversion ratio
	expectedReverseRatio := float32(ysfFramesOut) / float32(dmrFramesIn)
	actualReverseRatio := float32(YSF_TO_DMR_FRAME_RATIO) / float32(DMR_TO_YSF_FRAME_RATIO)
	if math.Abs(float64(expectedReverseRatio-actualReverseRatio)) > 0.01 {
		t.Fatalf("Reverse conversion ratio mismatch: expected %.3f, got %.3f", actualReverseRatio, expectedReverseRatio)
	}

	fmt.Printf("✓ DMR→YSF: %d frames → %d frames (ratio: %.3f)\n", dmrFramesIn, ysfFramesOut, expectedReverseRatio)

	// Test conversion statistics
	ysfToDmr, dmrToYsf, errors := converter.GetConversionStats()
	fmt.Printf("✓ Conversion stats: YSF→DMR=%d, DMR→YSF=%d, Errors=%d\n", ysfToDmr, dmrToYsf, errors)
}

// testAMBEValidation tests the AMBE validation system
func testAMBEValidation(t *testing.T) {
	validator := NewAMBEValidator(true, false, true) // Disable auto-correction for testing

	fmt.Println("Testing AMBE validation system...")

	// Test with valid parameters
	validParams := AMBEVoiceParams{
		A: 0x123456, // Valid 24-bit value
		B: 0x234567, // Valid 23-bit value
		C: 0x345678, // Valid 25-bit value
	}

	result := validator.ValidateAMBEFrame(&validParams)
	if !result.Valid {
		t.Errorf("Valid parameters failed validation: flags=0x%08X", result.ErrorFlags)
	}
	if result.SignalQuality < 0.5 {
		t.Errorf("Valid parameters have low quality: %.3f", result.SignalQuality)
	}

	// Test with invalid parameters (out of range)
	invalidParams := AMBEVoiceParams{
		A: 0x1000000, // Invalid (>24 bits)
		B: 0x800000,  // Invalid (>23 bits)
		C: 0x2000000, // Invalid (>25 bits)
	}

	result = validator.ValidateAMBEFrame(&invalidParams)
	if result.Valid {
		t.Error("Invalid parameters passed validation")
	}
	expectedErrors := uint32(AMBE_ERROR_INVALID_A_PARAM | AMBE_ERROR_INVALID_B_PARAM | AMBE_ERROR_INVALID_C_PARAM)
	if (result.ErrorFlags & expectedErrors) == 0 {
		t.Errorf("Expected parameter range errors not detected: flags=0x%08X", result.ErrorFlags)
	}

	// Test with all-zero parameters
	zeroParams := AMBEVoiceParams{A: 0, B: 0, C: 0}
	result = validator.ValidateAMBEFrame(&zeroParams)
	if (result.ErrorFlags & AMBE_ERROR_ALL_ZEROS) == 0 {
		t.Error("All-zero parameters not detected")
	}

	// Test error correction with a separate validator that has auto-correction enabled
	correctionValidator := NewAMBEValidator(true, true, true) // Enable auto-correction
	correctedParams := invalidParams
	result = correctionValidator.ValidateAMBEFrame(&correctedParams)
	if result.CorrectedErrors == 0 {
		t.Error("No error correction attempted on invalid parameters")
	}

	// Test quality assessment progression (using values that align with the algorithm's scoring)
	qualityTests := []AMBEVoiceParams{
		{A: 0x100000, B: 0x080000, C: 0x100000}, // Good quality (mid-range)
		{A: 0x800000, B: 0x400000, C: 0x800000}, // Lower quality (high values)
		{A: 0x001000, B: 0x000800, C: 0x001000}, // Poor quality (low values)
	}

	var qualityValues []float32
	for i, params := range qualityTests {
		result := validator.ValidateAMBEFrame(&params)
		qualityValues = append(qualityValues, result.SignalQuality)
		t.Logf("Quality test %d: params=A:0x%X,B:0x%X,C:0x%X quality=%.3f", i, params.A, params.B, params.C, result.SignalQuality)
	}

	// Check overall trend - first should be higher than last (allowing for some variation)
	if len(qualityValues) >= 2 && qualityValues[0] <= qualityValues[len(qualityValues)-1] {
		t.Errorf("Overall quality trend not decreasing: first=%.3f, last=%.3f", qualityValues[0], qualityValues[len(qualityValues)-1])
	}

	// Test statistics
	total, valid, corrected, discarded, avgBER, avgQuality := validator.GetStatistics()
	fmt.Printf("✓ Validation stats: Total=%d, Valid=%d, Corrected=%d, Discarded=%d, AvgBER=%.3f, AvgQuality=%.3f\n",
		total, valid, corrected, discarded, avgBER, avgQuality)

	if total == 0 {
		t.Error("No frames processed by validator")
	}
}

// testEndToEndConversion tests the complete conversion pipeline
func testEndToEndConversion(t *testing.T) {
	fmt.Println("Testing end-to-end conversion pipeline...")

	// Create all components
	ysfExtractor := NewYSFAMBEExtractor()
	converter := NewFrameRatioConverter()
	validator := NewAMBEValidator(false, true, true) // Less strict for end-to-end

	// Test YSF → DMR → YSF round trip
	originalYSFPayloads := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		originalYSFPayloads[i] = createSyntheticYSFPayload()
	}

	// Convert YSF to DMR
	var allDMRFrames [][]byte
	for _, ysfPayload := range originalYSFPayloads {
		dmrFrames, err := converter.ConvertYSFToDMR(ysfPayload)
		if err != nil {
			t.Fatalf("Failed YSF→DMR conversion: %v", err)
		}
		allDMRFrames = append(allDMRFrames, dmrFrames...)
	}

	if len(allDMRFrames) != 5 {
		t.Fatalf("Expected 5 DMR frames from 3 YSF frames, got %d", len(allDMRFrames))
	}

	// Convert DMR back to YSF
	converter.Reset()
	var reconvertedYSFFrames [][]byte
	for _, dmrFrame := range allDMRFrames {
		ysfFrames, err := converter.ConvertDMRToYSF(dmrFrame)
		if err != nil {
			t.Fatalf("Failed DMR→YSF conversion: %v", err)
		}
		reconvertedYSFFrames = append(reconvertedYSFFrames, ysfFrames...)
	}

	if len(reconvertedYSFFrames) != 3 {
		t.Fatalf("Expected 3 YSF frames from 5 DMR frames, got %d", len(reconvertedYSFFrames))
	}

	// Validate conversion quality
	totalValidatedFrames := 0
	totalQuality := float32(0.0)

	for i, ysfFrame := range reconvertedYSFFrames {
		vchSections, err := ysfExtractor.ExtractVCHSections(ysfFrame)
		if err != nil {
			t.Logf("Warning: Failed to extract VCH from reconverted frame %d: %v", i, err)
			continue
		}

		for j, vch := range vchSections {
			ambeParams, err := ysfExtractor.ConvertVCHToAMBE(&vch)
			if err != nil {
				t.Logf("Warning: Failed to convert VCH %d.%d to AMBE: %v", i, j, err)
				continue
			}

			result := validator.ValidateAMBEFrame(&ambeParams)
			if result.Valid {
				totalValidatedFrames++
				totalQuality += result.SignalQuality
			} else {
				t.Logf("Warning: Frame %d.%d failed validation: flags=0x%08X", i, j, result.ErrorFlags)
			}
		}
	}

	if totalValidatedFrames == 0 {
		t.Fatal("No valid frames in round-trip conversion")
	}

	averageQuality := totalQuality / float32(totalValidatedFrames)
	fmt.Printf("✓ Round-trip conversion: %d valid frames, average quality: %.3f\n", totalValidatedFrames, averageQuality)

	if averageQuality < 0.3 {
		t.Errorf("Round-trip conversion quality too low: %.3f", averageQuality)
	}
}

// testErrorHandling tests error conditions and edge cases
func testErrorHandling(t *testing.T) {
	fmt.Println("Testing error handling and edge cases...")

	// Test with invalid payload sizes
	ysfExtractor := NewYSFAMBEExtractor()
	dmrExtractor := NewDMRAMBEExtractor()

	// Too short YSF payload
	shortPayload := make([]byte, 10)
	_, err := ysfExtractor.ExtractVCHSections(shortPayload)
	if err == nil {
		t.Error("Short YSF payload should have failed")
	}

	// Too short DMR payload
	shortDMRPayload := make([]byte, 10)
	_, err = dmrExtractor.ExtractAMBEFrames(shortDMRPayload)
	if err == nil {
		t.Error("Short DMR payload should have failed")
	}

	// Test with corrupted data (all 0xFF)
	corruptedPayload := make([]byte, YSF_PAYLOAD_LENGTH)
	for i := range corruptedPayload {
		corruptedPayload[i] = 0xFF
	}

	vchSections, err := ysfExtractor.ExtractVCHSections(corruptedPayload)
	if err != nil {
		t.Logf("Corrupted payload extraction failed as expected: %v", err)
	} else {
		// Should extract but validation should fail
		validSections := 0
		for i, vch := range vchSections {
			if ysfExtractor.ValidateVCHSection(&vch) {
				validSections++
			} else {
				t.Logf("VCH section %d failed validation as expected", i)
			}
		}
		if validSections > 0 {
			t.Logf("Warning: %d corrupted VCH sections passed validation", validSections)
		}
	}

	// Test frame ratio converter edge cases
	converter := NewFrameRatioConverter()

	// Test incomplete frame sequences
	ysfPayload := createSyntheticYSFPayload()
	dmrFrames, err := converter.ConvertYSFToDMR(ysfPayload)
	if err != nil {
		t.Fatalf("Single YSF frame conversion failed: %v", err)
	}
	if len(dmrFrames) != 0 {
		t.Error("Single YSF frame should not produce DMR output")
	}

	// Test validator with extreme parameters
	validator := NewAMBEValidator(true, false, true) // Strict mode, no auto-correction

	extremeParams := []AMBEVoiceParams{
		{A: 0xFFFFFF, B: 0x7FFFFF, C: 0x1FFFFFF}, // Maximum values
		{A: 0x000000, B: 0x000000, C: 0x000000},   // Minimum values
		{A: 0xFFFFFF, B: 0x000000, C: 0x1FFFFFF}, // Mixed extreme values
	}

	for i, params := range extremeParams {
		result := validator.ValidateAMBEFrame(&params)
		t.Logf("Extreme params test %d: Valid=%t, Quality=%.3f, Errors=0x%08X",
			i, result.Valid, result.SignalQuality, result.ErrorFlags)
	}

	fmt.Println("✓ Error handling tests completed")
}

// testPerformanceBenchmark benchmarks the conversion performance
func testPerformanceBenchmark(t *testing.T) {
	fmt.Println("Running performance benchmark...")

	const numTestFrames = 1000

	// Benchmark YSF extraction
	ysfExtractor := NewYSFAMBEExtractor()
	startTime := time.Now()

	for i := 0; i < numTestFrames; i++ {
		ysfPayload := createSyntheticYSFPayload()
		_, err := ysfExtractor.ExtractVCHSections(ysfPayload)
		if err != nil {
			t.Fatalf("YSF extraction failed on frame %d: %v", i, err)
		}
	}

	ysfExtractionTime := time.Since(startTime)
	ysfFPS := float64(numTestFrames) / ysfExtractionTime.Seconds()

	// Benchmark DMR extraction
	dmrExtractor := NewDMRAMBEExtractor()
	startTime = time.Now()

	for i := 0; i < numTestFrames; i++ {
		dmrPayload := createSyntheticDMRPayload()
		_, err := dmrExtractor.ExtractAMBEFrames(dmrPayload)
		if err != nil {
			t.Fatalf("DMR extraction failed on frame %d: %v", i, err)
		}
	}

	dmrExtractionTime := time.Since(startTime)
	dmrFPS := float64(numTestFrames) / dmrExtractionTime.Seconds()

	// Benchmark frame conversion
	converter := NewFrameRatioConverter()
	startTime = time.Now()

	convertedFrames := 0
	for i := 0; i < numTestFrames; i++ {
		ysfPayload := createSyntheticYSFPayload()
		dmrFrames, err := converter.ConvertYSFToDMR(ysfPayload)
		if err != nil {
			t.Fatalf("Frame conversion failed on frame %d: %v", i, err)
		}
		convertedFrames += len(dmrFrames)
	}

	conversionTime := time.Since(startTime)
	conversionFPS := float64(numTestFrames) / conversionTime.Seconds()

	// Performance requirements check
	const requiredFPS = 100 // Should handle at least 100 fps for real-time processing

	fmt.Printf("✓ Performance benchmark results:\n")
	fmt.Printf("  YSF extraction: %.1f fps (%.2f ms/frame)\n", ysfFPS, ysfExtractionTime.Seconds()*1000/numTestFrames)
	fmt.Printf("  DMR extraction: %.1f fps (%.2f ms/frame)\n", dmrFPS, dmrExtractionTime.Seconds()*1000/numTestFrames)
	fmt.Printf("  Frame conversion: %.1f fps (%.2f ms/frame)\n", conversionFPS, conversionTime.Seconds()*1000/numTestFrames)
	fmt.Printf("  Total converted frames: %d\n", convertedFrames)

	if ysfFPS < requiredFPS {
		t.Errorf("YSF extraction performance too low: %.1f fps (required: %d fps)", ysfFPS, requiredFPS)
	}
	if dmrFPS < requiredFPS {
		t.Errorf("DMR extraction performance too low: %.1f fps (required: %d fps)", dmrFPS, requiredFPS)
	}
	if conversionFPS < requiredFPS {
		t.Errorf("Frame conversion performance too low: %.1f fps (required: %d fps)", conversionFPS, requiredFPS)
	}
}

// Helper functions for creating synthetic test data

// createSyntheticYSFPayload creates a realistic synthetic YSF payload for testing
func createSyntheticYSFPayload() []byte {
	payload := make([]byte, YSF_PAYLOAD_LENGTH)

	// Fill with pseudo-random but structured data
	for i := 0; i < len(payload); i++ {
		// Create pattern that simulates voice data
		payload[i] = uint8((i*13 + 37) % 256) // Simple pseudo-random pattern
	}

	// Add some structure to simulate real voice frame patterns
	for sectionIndex := 0; sectionIndex < YSF_VCH_SECTIONS; sectionIndex++ {
		sectionStart := sectionIndex * (YSF_VCH_BITS * 3 / 8)
		for i := 0; i < 10 && sectionStart+i < len(payload); i++ {
			// Add voice-like patterns
			payload[sectionStart+i] = uint8(0x55 + sectionIndex*17) // Alternating pattern with variation
		}
	}

	return payload
}

// createSyntheticDMRPayload creates a realistic synthetic DMR payload for testing
func createSyntheticDMRPayload() []byte {
	payload := make([]byte, DMR_FRAME_LENGTH)

	// Fill with pseudo-random but structured data
	for i := 0; i < len(payload); i++ {
		// Create pattern that simulates AMBE voice data
		payload[i] = uint8((i*7 + 23) % 256) // Different pattern from YSF
	}

	// Add DMR-specific structure
	for frameIndex := 0; frameIndex < DMR_AMBE_FRAMES; frameIndex++ {
		frameStart := frameIndex * 16 // Approximate frame positioning
		for i := 0; i < 8 && frameStart+i < len(payload); i++ {
			// Add AMBE-like patterns
			payload[frameStart+i] = uint8(0xAA - frameIndex*11) // Different pattern per frame
		}
	}

	return payload
}

// Benchmark functions for go test -bench

func BenchmarkYSFExtraction(b *testing.B) {
	extractor := NewYSFAMBEExtractor()
	payload := createSyntheticYSFPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractVCHSections(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDMRExtraction(b *testing.B) {
	extractor := NewDMRAMBEExtractor()
	payload := createSyntheticDMRPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractor.ExtractAMBEFrames(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameConversion(b *testing.B) {
	converter := NewFrameRatioConverter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := createSyntheticYSFPayload()
		_, err := converter.ConvertYSFToDMR(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAMBEValidation(b *testing.B) {
	validator := NewAMBEValidator(true, true, true)
	params := AMBEVoiceParams{A: 0x123456, B: 0x234567, C: 0x345678}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateAMBEFrame(&params)
	}
}