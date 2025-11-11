package network

import (
	"fmt"
	"log"
)

// RingBuffer implements a thread-safe circular buffer equivalent to C++ CRingBuffer
type RingBuffer struct {
	buffer   []byte
	head     int
	tail     int
	size     int
	capacity int
	name     string
}

// NewRingBuffer creates a new ring buffer with specified capacity
// Equivalent to C++ CRingBuffer<T>(length, name)
func NewRingBuffer(capacity int, name string) *RingBuffer {
	return &RingBuffer{
		buffer:   make([]byte, capacity+1), // +1 to distinguish full from empty
		capacity: capacity,
		name:     name,
	}
}

// AddData adds data to the ring buffer
// Equivalent to C++ CRingBuffer::addData()
// Returns false if insufficient space
func (rb *RingBuffer) AddData(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// Check if we have enough space
	if !rb.HasSpace(len(data)) {
		log.Printf("RingBuffer[%s]: Insufficient space for %d bytes (free: %d)",
			rb.name, len(data), rb.FreeSpace())
		return false
	}

	// Add data byte by byte to handle wrap-around
	for _, b := range data {
		rb.buffer[rb.head] = b
		rb.head = (rb.head + 1) % len(rb.buffer)
		rb.size++
	}

	return true
}

// GetData retrieves data from the ring buffer
// Equivalent to C++ CRingBuffer::getData()
// Returns false if insufficient data available
func (rb *RingBuffer) GetData(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// Check if we have enough data
	if rb.size < len(data) {
		return false
	}

	// Read data byte by byte to handle wrap-around
	for i := range data {
		data[i] = rb.buffer[rb.tail]
		rb.tail = (rb.tail + 1) % len(rb.buffer)
		rb.size--
	}

	return true
}

// Peek looks at data without removing it from the buffer
// Equivalent to C++ CRingBuffer::peek()
func (rb *RingBuffer) Peek(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	// Check if we have enough data
	if rb.size < len(data) {
		return false
	}

	// Read data without modifying tail
	tempTail := rb.tail
	for i := range data {
		data[i] = rb.buffer[tempTail]
		tempTail = (tempTail + 1) % len(rb.buffer)
	}

	return true
}

// Clear empties the ring buffer
// Equivalent to C++ CRingBuffer::clear()
func (rb *RingBuffer) Clear() {
	rb.head = 0
	rb.tail = 0
	rb.size = 0
}

// FreeSpace returns available space in bytes
// Equivalent to C++ CRingBuffer::freeSpace()
func (rb *RingBuffer) FreeSpace() int {
	return rb.capacity - rb.size
}

// DataSize returns amount of data in buffer
// Equivalent to C++ CRingBuffer::dataSize()
func (rb *RingBuffer) DataSize() int {
	return rb.size
}

// HasSpace checks if buffer has space for specified amount
// Equivalent to C++ CRingBuffer::hasSpace()
func (rb *RingBuffer) HasSpace(length int) bool {
	return rb.FreeSpace() >= length
}

// HasData returns true if buffer contains data
// Equivalent to C++ CRingBuffer::hasData()
func (rb *RingBuffer) HasData() bool {
	return rb.size > 0
}

// IsEmpty returns true if buffer is empty
// Equivalent to C++ CRingBuffer::isEmpty()
func (rb *RingBuffer) IsEmpty() bool {
	return rb.size == 0
}

// GetName returns the buffer name for debugging
func (rb *RingBuffer) GetName() string {
	return rb.name
}

// String returns a string representation for debugging
func (rb *RingBuffer) String() string {
	return fmt.Sprintf("RingBuffer[%s]: size=%d, capacity=%d, head=%d, tail=%d",
		rb.name, rb.size, rb.capacity, rb.head, rb.tail)
}

// AddLength stores a length prefix followed by data
// This matches the C++ YSFNetwork pattern of storing length + data
func (rb *RingBuffer) AddLength(data []byte) bool {
	length := len(data)

	// Store as 2-byte length prefix (big-endian) + data
	lengthBytes := []byte{byte(length >> 8), byte(length & 0xFF)}

	if !rb.AddData(lengthBytes) {
		return false
	}

	if length > 0 {
		return rb.AddData(data)
	}

	return true
}

// GetLength retrieves length prefix and data
// Returns the data length and fills the provided buffer
func (rb *RingBuffer) GetLength(data []byte) (int, bool) {
	// Need at least 2 bytes for length prefix
	if rb.size < 2 {
		return 0, false
	}

	// Peek at length prefix
	lengthBytes := make([]byte, 2)
	if !rb.Peek(lengthBytes) {
		return 0, false
	}

	length := (int(lengthBytes[0]) << 8) | int(lengthBytes[1])

	// Check if we have the complete packet
	if rb.size < 2 + length {
		return 0, false
	}

	// Remove length prefix
	rb.GetData(lengthBytes)

	// Check buffer size
	if len(data) < length {
		return 0, false
	}

	// Get actual data
	if length > 0 {
		actualData := data[:length]
		if !rb.GetData(actualData) {
			return 0, false
		}
	}

	return length, true
}