package network

import (
	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

// DelayBuffer manages network jitter by buffering DMR frames
// Equivalent to C++ CDelayBuffer
type DelayBuffer struct {
	blockSize      int                                   // Size of each data block (55 bytes)
	blockTime      int                                   // Time per block in ms (60ms for DMR)
	buffer         [][]byte                             // Circular buffer of data blocks
	sequence       []uint8                              // Sequence numbers for each block
	missing        []bool                               // Missing flag for each block
	readPtr        int                                   // Read pointer
	writePtr       int                                   // Write pointer
	length         int                                   // Number of blocks in buffer
	jitterTime     int                                   // Jitter buffer time in ms
	jitterSlots    int                                   // Number of jitter slots
	currentTime    int                                   // Current time in ms (driven by Clock)
	lastWriteTime  int                                   // Time when last write occurred
	sequenceNumber uint8                                 // Expected sequence number
	running        bool                                  // Buffer is running
}

// NewDelayBuffer creates a new delay buffer
// Equivalent to C++ CDelayBuffer constructor
func NewDelayBuffer(blockSize int, blockTime int, jitterTime int) *DelayBuffer {
	// Calculate number of jitter slots needed
	jitterSlots := jitterTime / blockTime
	if jitterSlots < 1 {
		jitterSlots = 1
	}

	// Total buffer length includes jitter slots plus some overhead
	length := jitterSlots + 10

	buffer := &DelayBuffer{
		blockSize:      blockSize,
		blockTime:      blockTime,
		buffer:         make([][]byte, length),
		sequence:       make([]uint8, length),
		missing:        make([]bool, length),
		readPtr:        0,
		writePtr:       0,
		length:         length,
		jitterTime:     jitterTime,
		jitterSlots:    jitterSlots,
		currentTime:    0,
		lastWriteTime:  0,
		sequenceNumber: 0,
		running:        false,
	}

	// Initialize buffer slots
	for i := range buffer.buffer {
		buffer.buffer[i] = make([]byte, blockSize)
	}

	return buffer
}

// AddData adds a data block to the buffer
// Equivalent to C++ CDelayBuffer::addData()
func (db *DelayBuffer) AddData(data []byte, seqNo uint8) bool {
	if len(data) != db.blockSize {
		return false
	}

	// Start the buffer if not running
	if !db.running {
		db.running = true
		db.sequenceNumber = seqNo
		db.lastWriteTime = db.currentTime
	}

	// Check for sequence number gaps
	expectedSeq := db.sequenceNumber
	if seqNo != expectedSeq {
		// Handle sequence gap - mark missing frames
		gap := int(seqNo) - int(expectedSeq)
		if gap < 0 {
			gap += 256 // Handle wrap-around
		}

		// Limit gap to reasonable size
		if gap > 20 {
			// Too large gap, reset
			db.sequenceNumber = seqNo
		} else {
			// Fill gap with missing frames
			for i := 0; i < gap; i++ {
				db.addMissingFrame(uint8(int(expectedSeq+uint8(i)) % 256))
			}
		}
	}

	// Store the actual frame
	db.storeFrame(data, seqNo, false)
	db.sequenceNumber = uint8((int(seqNo) + 1) % 256)
	db.lastWriteTime = db.currentTime

	return true
}

// GetData retrieves a data block from the buffer
// Equivalent to C++ CDelayBuffer::getData()
func (db *DelayBuffer) GetData(data []byte) protocol.DelayBufferStatus {
	if len(data) < db.blockSize {
		return protocol.BS_NO_DATA
	}

	if !db.running {
		return protocol.BS_NO_DATA
	}

	// Check if enough time has passed for jitter buffering
	timeSinceLastWrite := db.currentTime - db.lastWriteTime
	if timeSinceLastWrite < db.jitterTime {
		// Not enough data buffered yet
		if db.countBufferedFrames() < db.jitterSlots {
			return protocol.BS_NO_DATA
		}
	}

	// Check if data is available at read pointer
	if db.readPtr == db.writePtr {
		return protocol.BS_NO_DATA
	}

	// Copy data from buffer
	copy(data, db.buffer[db.readPtr])
	isMissing := db.missing[db.readPtr]

	// Advance read pointer
	db.readPtr = (db.readPtr + 1) % db.length

	if isMissing {
		return protocol.BS_MISSING
	}
	return protocol.BS_DATA
}

// Clock advances the buffer time
// Equivalent to C++ CDelayBuffer::clock()
func (db *DelayBuffer) Clock(ms int) {
	db.currentTime += ms
}

// Reset clears the buffer
// Equivalent to C++ CDelayBuffer::reset()
func (db *DelayBuffer) Reset() {
	db.readPtr = 0
	db.writePtr = 0
	db.currentTime = 0
	db.lastWriteTime = 0
	db.sequenceNumber = 0
	db.running = false

	// Clear all slots
	for i := range db.missing {
		db.missing[i] = false
	}
}

// IsRunning returns true if buffer is active
func (db *DelayBuffer) IsRunning() bool {
	return db.running
}

// GetJitterTime returns the jitter time in ms
func (db *DelayBuffer) GetJitterTime() int {
	return db.jitterTime
}

// SetJitterTime sets the jitter time in ms
func (db *DelayBuffer) SetJitterTime(jitterTime int) {
	db.jitterTime = jitterTime
	db.jitterSlots = jitterTime / db.blockTime
	if db.jitterSlots < 1 {
		db.jitterSlots = 1
	}
}

// storeFrame stores a frame in the buffer
func (db *DelayBuffer) storeFrame(data []byte, seqNo uint8, isMissing bool) {
	copy(db.buffer[db.writePtr], data)
	db.sequence[db.writePtr] = seqNo
	db.missing[db.writePtr] = isMissing

	// Advance write pointer
	db.writePtr = (db.writePtr + 1) % db.length

	// Handle buffer overflow
	if db.writePtr == db.readPtr {
		// Buffer full, advance read pointer
		db.readPtr = (db.readPtr + 1) % db.length
	}
}

// addMissingFrame adds a missing frame marker
func (db *DelayBuffer) addMissingFrame(seqNo uint8) {
	// Create empty data block
	emptyData := make([]byte, db.blockSize)
	db.storeFrame(emptyData, seqNo, true)
}

// countBufferedFrames returns number of frames currently buffered
func (db *DelayBuffer) countBufferedFrames() int {
	if db.writePtr >= db.readPtr {
		return db.writePtr - db.readPtr
	}
	return (db.length - db.readPtr) + db.writePtr
}

// GetStats returns buffer statistics for debugging
func (db *DelayBuffer) GetStats() (int, int, int, bool) {
	buffered := db.countBufferedFrames()
	return buffered, db.jitterSlots, db.currentTime, db.running
}