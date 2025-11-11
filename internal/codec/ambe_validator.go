package codec

import (
	"math"
	"time"
)

// AMBEValidationResult represents the result of AMBE frame validation
type AMBEValidationResult struct {
	Valid           bool    // Overall validity of the frame
	BitErrorRate    float32 // Estimated bit error rate (0.0 - 1.0)
	SignalQuality   float32 // Signal quality metric (0.0 - 1.0)
	ErrorFlags      uint32  // Specific error flags
	CorrectedErrors int     // Number of corrected errors
	SuggestedAction string  // Recommended action for this frame
}

// Error flags for AMBE validation
const (
	AMBE_ERROR_NONE              = 0x00000000
	AMBE_ERROR_INVALID_A_PARAM   = 0x00000001 // A parameter out of range
	AMBE_ERROR_INVALID_B_PARAM   = 0x00000002 // B parameter out of range
	AMBE_ERROR_INVALID_C_PARAM   = 0x00000004 // C parameter out of range
	AMBE_ERROR_ALL_ZEROS         = 0x00000008 // All parameters are zero
	AMBE_ERROR_ALL_ONES          = 0x00000010 // All parameters are max value
	AMBE_ERROR_GOLAY_FAILURE     = 0x00000020 // Golay error correction failed
	AMBE_ERROR_CHECKSUM_FAILURE  = 0x00000040 // Checksum validation failed
	AMBE_ERROR_RAPID_CHANGE      = 0x00000080 // Rapid parameter change detected
	AMBE_ERROR_SILENCE_DETECTED  = 0x00000100 // Extended silence detected
	AMBE_ERROR_NOISE_DETECTED    = 0x00000200 // High noise level detected
	AMBE_ERROR_SYNC_LOST         = 0x00000400 // Frame synchronization lost
	AMBE_ERROR_BUFFER_UNDERRUN   = 0x00000800 // Buffer underrun detected
	AMBE_ERROR_BUFFER_OVERRUN    = 0x00001000 // Buffer overrun detected
)

// Voice parameter ranges for validation (based on AMBE vocoder specifications)
const (
	AMBE_A_PARAM_MIN = 0x000000   // Minimum A parameter value
	AMBE_A_PARAM_MAX = 0xFFFFFF   // Maximum A parameter value (24-bit)
	AMBE_B_PARAM_MIN = 0x000000   // Minimum B parameter value
	AMBE_B_PARAM_MAX = 0x7FFFFF   // Maximum B parameter value (23-bit)
	AMBE_C_PARAM_MIN = 0x000000   // Minimum C parameter value
	AMBE_C_PARAM_MAX = 0x1FFFFFF  // Maximum C parameter value (25-bit)

	// Reasonable operating ranges (excluding extreme values)
	AMBE_A_PARAM_TYPICAL_MIN = 0x001000
	AMBE_A_PARAM_TYPICAL_MAX = 0xFFF000
	AMBE_B_PARAM_TYPICAL_MIN = 0x001000
	AMBE_B_PARAM_TYPICAL_MAX = 0x7FF000
	AMBE_C_PARAM_TYPICAL_MIN = 0x001000
	AMBE_C_PARAM_TYPICAL_MAX = 0x1FFF000

	// Quality thresholds
	AMBE_BER_EXCELLENT = 0.001 // BER < 0.1% = excellent
	AMBE_BER_GOOD      = 0.01  // BER < 1% = good
	AMBE_BER_POOR      = 0.05  // BER < 5% = poor
	AMBE_BER_BAD       = 0.1   // BER >= 10% = bad

	// Frame analysis constants
	AMBE_SILENCE_THRESHOLD    = 0.01  // Threshold for silence detection
	AMBE_NOISE_THRESHOLD      = 0.8   // Threshold for noise detection
	AMBE_CHANGE_THRESHOLD     = 0.3   // Threshold for rapid parameter change
	AMBE_HISTORY_FRAMES       = 10    // Number of frames to keep for analysis
)

// AMBEValidator provides comprehensive AMBE frame validation and error handling
type AMBEValidator struct {
	// Frame history for trend analysis
	frameHistory    []AMBEVoiceParams
	qualityHistory  []float32
	errorHistory    []uint32
	historyIndex    int
	historyFull     bool

	// Statistics tracking
	totalFrames       uint64
	validFrames       uint64
	correctedFrames   uint64
	discardedFrames   uint64
	totalBitErrors    uint64
	averageBER        float32
	averageQuality    float32

	// Error correction state
	consecutiveErrors int
	lastGoodFrame     AMBEVoiceParams
	lastFrameTime     time.Time

	// Configuration
	strictValidation bool // Enable strict validation mode
	autoCorrection   bool // Enable automatic error correction
	qualityReporting bool // Enable quality reporting
}

// NewAMBEValidator creates a new AMBE validator
func NewAMBEValidator(strictMode, autoCorrect, qualityReport bool) *AMBEValidator {
	return &AMBEValidator{
		frameHistory:     make([]AMBEVoiceParams, AMBE_HISTORY_FRAMES),
		qualityHistory:   make([]float32, AMBE_HISTORY_FRAMES),
		errorHistory:     make([]uint32, AMBE_HISTORY_FRAMES),
		strictValidation: strictMode,
		autoCorrection:   autoCorrect,
		qualityReporting: qualityReport,
		lastFrameTime:    time.Now(),
	}
}

// ValidateAMBEFrame performs comprehensive validation of an AMBE frame
func (v *AMBEValidator) ValidateAMBEFrame(params *AMBEVoiceParams) AMBEValidationResult {
	result := AMBEValidationResult{
		Valid:         true,
		BitErrorRate:  0.0,
		SignalQuality: 1.0,
		ErrorFlags:    AMBE_ERROR_NONE,
	}

	v.totalFrames++

	// Basic parameter range validation
	v.validateParameterRanges(params, &result)

	// Advanced parameter analysis
	v.analyzeParameterPattern(params, &result)

	// Trend analysis using frame history
	v.analyzeTrends(params, &result)

	// Quality assessment
	v.assessSignalQuality(params, &result)

	// Error correction if enabled
	if v.autoCorrection && !result.Valid {
		corrected := v.attemptErrorCorrection(params, &result)
		if corrected {
			v.correctedFrames++
			result.CorrectedErrors++
		}
	}

	// Update statistics and history
	v.updateHistory(params, &result)
	v.updateStatistics(&result)

	// Determine suggested action
	v.determineSuggestedAction(&result)

	if result.Valid {
		v.validFrames++
		v.lastGoodFrame = *params
	} else {
		v.discardedFrames++
		v.consecutiveErrors++
	}

	v.lastFrameTime = time.Now()

	return result
}

// validateParameterRanges performs basic range validation on voice parameters
func (v *AMBEValidator) validateParameterRanges(params *AMBEVoiceParams, result *AMBEValidationResult) {
	// Check A parameter (fundamental frequency and voicing)
	if params.A < AMBE_A_PARAM_MIN || params.A > AMBE_A_PARAM_MAX {
		result.Valid = false
		result.ErrorFlags |= AMBE_ERROR_INVALID_A_PARAM
	}

	// Check B parameter (spectral coefficients)
	if params.B < AMBE_B_PARAM_MIN || params.B > AMBE_B_PARAM_MAX {
		result.Valid = false
		result.ErrorFlags |= AMBE_ERROR_INVALID_B_PARAM
	}

	// Check C parameter (additional voice parameters)
	if params.C < AMBE_C_PARAM_MIN || params.C > AMBE_C_PARAM_MAX {
		result.Valid = false
		result.ErrorFlags |= AMBE_ERROR_INVALID_C_PARAM
	}

	// Check for all zeros (silence or error)
	if params.A == 0 && params.B == 0 && params.C == 0 {
		result.ErrorFlags |= AMBE_ERROR_ALL_ZEROS
		if v.strictValidation {
			result.Valid = false
		}
	}

	// Check for all maximum values (error condition)
	if params.A == AMBE_A_PARAM_MAX && params.B == AMBE_B_PARAM_MAX && params.C == AMBE_C_PARAM_MAX {
		result.Valid = false
		result.ErrorFlags |= AMBE_ERROR_ALL_ONES
	}
}

// analyzeParameterPattern analyzes parameter patterns for anomalies
func (v *AMBEValidator) analyzeParameterPattern(params *AMBEVoiceParams, result *AMBEValidationResult) {
	// Check for parameters outside typical operating ranges
	if v.strictValidation {
		if params.A < AMBE_A_PARAM_TYPICAL_MIN || params.A > AMBE_A_PARAM_TYPICAL_MAX {
			result.SignalQuality *= 0.8 // Reduce quality score
		}
		if params.B < AMBE_B_PARAM_TYPICAL_MIN || params.B > AMBE_B_PARAM_TYPICAL_MAX {
			result.SignalQuality *= 0.8
		}
		if params.C < AMBE_C_PARAM_TYPICAL_MIN || params.C > AMBE_C_PARAM_TYPICAL_MAX {
			result.SignalQuality *= 0.8
		}
	}

	// Analyze parameter relationships
	v.analyzeParameterRelationships(params, result)
}

// analyzeParameterRelationships checks relationships between A, B, C parameters
func (v *AMBEValidator) analyzeParameterRelationships(params *AMBEVoiceParams, result *AMBEValidationResult) {
	// Check for unrealistic parameter combinations
	// A parameter represents fundamental frequency - should correlate with spectral content

	// Calculate parameter ratios
	aRatio := float32(params.A) / float32(AMBE_A_PARAM_MAX)
	bRatio := float32(params.B) / float32(AMBE_B_PARAM_MAX)
	cRatio := float32(params.C) / float32(AMBE_C_PARAM_MAX)

	// Check for extreme parameter imbalances
	maxRatio := float32(math.Max(float64(aRatio), math.Max(float64(bRatio), float64(cRatio))))
	minRatio := float32(math.Min(float64(aRatio), math.Min(float64(bRatio), float64(cRatio))))

	if maxRatio > 0.95 && minRatio < 0.05 {
		// Extreme imbalance detected
		result.SignalQuality *= 0.5
		result.ErrorFlags |= AMBE_ERROR_NOISE_DETECTED
	}
}

// analyzeTrends analyzes parameter trends across multiple frames
func (v *AMBEValidator) analyzeTrends(params *AMBEVoiceParams, result *AMBEValidationResult) {
	if !v.historyFull && v.historyIndex == 0 {
		return // No history available yet
	}

	// Get previous frame for comparison
	prevIndex := (v.historyIndex - 1 + AMBE_HISTORY_FRAMES) % AMBE_HISTORY_FRAMES
	prevParams := v.frameHistory[prevIndex]

	// Calculate parameter changes
	aChange := v.calculateParameterChange(params.A, prevParams.A, AMBE_A_PARAM_MAX)
	bChange := v.calculateParameterChange(params.B, prevParams.B, AMBE_B_PARAM_MAX)
	cChange := v.calculateParameterChange(params.C, prevParams.C, AMBE_C_PARAM_MAX)

	// Check for rapid changes
	maxChange := float32(math.Max(float64(aChange), math.Max(float64(bChange), float64(cChange))))
	if maxChange > AMBE_CHANGE_THRESHOLD {
		result.ErrorFlags |= AMBE_ERROR_RAPID_CHANGE
		result.SignalQuality *= 0.7
	}

	// Analyze silence patterns
	v.analyzeSilencePattern(params, result)
}

// calculateParameterChange calculates normalized change between two parameter values
func (v *AMBEValidator) calculateParameterChange(current, previous, maxValue uint32) float32 {
	if maxValue == 0 {
		return 0.0
	}
	diff := float32(int32(current) - int32(previous))
	if diff < 0 {
		diff = -diff
	}
	return diff / float32(maxValue)
}

// analyzeSilencePattern detects extended silence or noise patterns
func (v *AMBEValidator) analyzeSilencePattern(params *AMBEVoiceParams, result *AMBEValidationResult) {
	// Calculate energy level from parameters
	energy := v.calculateEnergyLevel(params)

	if energy < AMBE_SILENCE_THRESHOLD {
		result.ErrorFlags |= AMBE_ERROR_SILENCE_DETECTED
	} else if energy > AMBE_NOISE_THRESHOLD {
		result.ErrorFlags |= AMBE_ERROR_NOISE_DETECTED
		result.SignalQuality *= 0.6
	}
}

// calculateEnergyLevel estimates energy level from AMBE parameters
func (v *AMBEValidator) calculateEnergyLevel(params *AMBEVoiceParams) float32 {
	// Simple energy estimation based on parameter magnitudes
	aEnergy := float32(params.A) / float32(AMBE_A_PARAM_MAX)
	bEnergy := float32(params.B) / float32(AMBE_B_PARAM_MAX)
	cEnergy := float32(params.C) / float32(AMBE_C_PARAM_MAX)

	// Weighted combination (A parameter typically carries more energy information)
	return (aEnergy*0.5 + bEnergy*0.3 + cEnergy*0.2)
}

// assessSignalQuality provides overall signal quality assessment
func (v *AMBEValidator) assessSignalQuality(params *AMBEVoiceParams, result *AMBEValidationResult) {
	// Start with base quality
	quality := result.SignalQuality

	// Factor in error flags
	errorCount := v.countErrorFlags(result.ErrorFlags)
	quality *= (1.0 - float32(errorCount)*0.1) // Reduce quality by 10% per error type

	// Factor in parameter distributions
	quality *= v.assessParameterDistribution(params)

	// Factor in consecutive error count
	if v.consecutiveErrors > 0 {
		quality *= (1.0 - float32(v.consecutiveErrors)*0.05) // 5% penalty per consecutive error
	}

	// Ensure quality stays in valid range
	if quality < 0.0 {
		quality = 0.0
	}
	if quality > 1.0 {
		quality = 1.0
	}

	result.SignalQuality = quality

	// Estimate bit error rate based on quality
	result.BitErrorRate = v.estimateBER(quality, result.ErrorFlags)
}

// countErrorFlags counts the number of error flags set
func (v *AMBEValidator) countErrorFlags(flags uint32) int {
	count := 0
	for i := 0; i < 32; i++ {
		if (flags & (1 << i)) != 0 {
			count++
		}
	}
	return count
}

// assessParameterDistribution assesses how well parameters fit expected distributions
func (v *AMBEValidator) assessParameterDistribution(params *AMBEVoiceParams) float32 {
	// Simple assessment based on parameter values relative to typical ranges
	aScore := v.scoreParameterValue(params.A, AMBE_A_PARAM_TYPICAL_MIN, AMBE_A_PARAM_TYPICAL_MAX)
	bScore := v.scoreParameterValue(params.B, AMBE_B_PARAM_TYPICAL_MIN, AMBE_B_PARAM_TYPICAL_MAX)
	cScore := v.scoreParameterValue(params.C, AMBE_C_PARAM_TYPICAL_MIN, AMBE_C_PARAM_TYPICAL_MAX)

	return (aScore + bScore + cScore) / 3.0
}

// scoreParameterValue scores a parameter value based on its position in the typical range
func (v *AMBEValidator) scoreParameterValue(value, minTypical, maxTypical uint32) float32 {
	if value >= minTypical && value <= maxTypical {
		return 1.0 // Perfect score for typical range
	}

	// Score based on distance from typical range
	if value < minTypical {
		ratio := float32(value) / float32(minTypical)
		return ratio // Linear falloff below minimum
	} else {
		// value > maxTypical
		excess := float32(value - maxTypical)
		maxExcess := float32(AMBE_A_PARAM_MAX - maxTypical) // Use A param as reference
		ratio := 1.0 - (excess / maxExcess)
		if ratio < 0 {
			ratio = 0
		}
		return ratio
	}
}

// estimateBER estimates bit error rate based on quality and error flags
func (v *AMBEValidator) estimateBER(quality float32, errorFlags uint32) float32 {
	baseBER := (1.0 - quality) * 0.1 // Base BER from quality

	// Add BER based on specific error types
	if (errorFlags & AMBE_ERROR_GOLAY_FAILURE) != 0 {
		baseBER += 0.05 // Golay failure adds significant BER
	}
	if (errorFlags & (AMBE_ERROR_INVALID_A_PARAM | AMBE_ERROR_INVALID_B_PARAM | AMBE_ERROR_INVALID_C_PARAM)) != 0 {
		baseBER += 0.02 // Parameter range errors add moderate BER
	}
	if (errorFlags & AMBE_ERROR_RAPID_CHANGE) != 0 {
		baseBER += 0.01 // Rapid changes add minor BER
	}

	// Cap at maximum realistic BER
	if baseBER > 0.5 {
		baseBER = 0.5
	}

	return baseBER
}

// attemptErrorCorrection attempts to correct errors in AMBE parameters
func (v *AMBEValidator) attemptErrorCorrection(params *AMBEVoiceParams, result *AMBEValidationResult) bool {
	corrected := false
	originalParams := *params

	// Attempt range corrections
	if (result.ErrorFlags & AMBE_ERROR_INVALID_A_PARAM) != 0 {
		if v.correctParameterRange(&params.A, AMBE_A_PARAM_MIN, AMBE_A_PARAM_MAX) {
			corrected = true
		}
	}

	if (result.ErrorFlags & AMBE_ERROR_INVALID_B_PARAM) != 0 {
		if v.correctParameterRange(&params.B, AMBE_B_PARAM_MIN, AMBE_B_PARAM_MAX) {
			corrected = true
		}
	}

	if (result.ErrorFlags & AMBE_ERROR_INVALID_C_PARAM) != 0 {
		if v.correctParameterRange(&params.C, AMBE_C_PARAM_MIN, AMBE_C_PARAM_MAX) {
			corrected = true
		}
	}

	// Attempt interpolation correction using last good frame
	if (result.ErrorFlags & AMBE_ERROR_ALL_ZEROS) != 0 && v.validFrames > 0 {
		*params = v.interpolateWithLastGood(originalParams)
		corrected = true
	}

	if corrected {
		result.Valid = true
		result.ErrorFlags = AMBE_ERROR_NONE // Clear errors after correction
	}

	return corrected
}

// correctParameterRange corrects a parameter to fit within valid range
func (v *AMBEValidator) correctParameterRange(param *uint32, minVal, maxVal uint32) bool {
	if *param < minVal {
		*param = minVal
		return true
	}
	if *param > maxVal {
		*param = maxVal
		return true
	}
	return false
}

// interpolateWithLastGood interpolates current frame with last good frame
func (v *AMBEValidator) interpolateWithLastGood(current AMBEVoiceParams) AMBEVoiceParams {
	// Simple 50/50 interpolation
	result := AMBEVoiceParams{
		A: (current.A + v.lastGoodFrame.A) / 2,
		B: (current.B + v.lastGoodFrame.B) / 2,
		C: (current.C + v.lastGoodFrame.C) / 2,
	}
	return result
}

// updateHistory updates the frame history with current frame
func (v *AMBEValidator) updateHistory(params *AMBEVoiceParams, result *AMBEValidationResult) {
	v.frameHistory[v.historyIndex] = *params
	v.qualityHistory[v.historyIndex] = result.SignalQuality
	v.errorHistory[v.historyIndex] = result.ErrorFlags

	v.historyIndex = (v.historyIndex + 1) % AMBE_HISTORY_FRAMES
	if v.historyIndex == 0 {
		v.historyFull = true
	}
}

// updateStatistics updates running statistics
func (v *AMBEValidator) updateStatistics(result *AMBEValidationResult) {
	// Update BER average
	v.averageBER = (v.averageBER*float32(v.totalFrames-1) + result.BitErrorRate) / float32(v.totalFrames)

	// Update quality average
	v.averageQuality = (v.averageQuality*float32(v.totalFrames-1) + result.SignalQuality) / float32(v.totalFrames)

	// Update bit error count
	if result.BitErrorRate > 0 {
		estimatedBits := uint64(96) // Typical AMBE frame size
		v.totalBitErrors += uint64(float32(estimatedBits) * result.BitErrorRate)
	}

	// Reset consecutive error count on valid frame
	if result.Valid {
		v.consecutiveErrors = 0
	}
}

// determineSuggestedAction determines recommended action based on validation result
func (v *AMBEValidator) determineSuggestedAction(result *AMBEValidationResult) {
	if result.Valid {
		if result.SignalQuality > 0.8 {
			result.SuggestedAction = "PASS"
		} else {
			result.SuggestedAction = "PASS_WITH_WARNING"
		}
	} else {
		if v.autoCorrection && result.CorrectedErrors > 0 {
			result.SuggestedAction = "CORRECTED"
		} else if result.BitErrorRate < AMBE_BER_BAD {
			result.SuggestedAction = "INTERPOLATE"
		} else {
			result.SuggestedAction = "DISCARD"
		}
	}
}

// GetStatistics returns validation statistics
func (v *AMBEValidator) GetStatistics() (uint64, uint64, uint64, uint64, float32, float32) {
	return v.totalFrames, v.validFrames, v.correctedFrames, v.discardedFrames, v.averageBER, v.averageQuality
}

// Reset resets the validator state
func (v *AMBEValidator) Reset() {
	v.historyIndex = 0
	v.historyFull = false
	v.totalFrames = 0
	v.validFrames = 0
	v.correctedFrames = 0
	v.discardedFrames = 0
	v.totalBitErrors = 0
	v.averageBER = 0.0
	v.averageQuality = 0.0
	v.consecutiveErrors = 0
	v.lastFrameTime = time.Now()

	// Clear histories
	for i := range v.frameHistory {
		v.frameHistory[i] = AMBEVoiceParams{}
	}
	for i := range v.qualityHistory {
		v.qualityHistory[i] = 0.0
	}
	for i := range v.errorHistory {
		v.errorHistory[i] = AMBE_ERROR_NONE
	}
}