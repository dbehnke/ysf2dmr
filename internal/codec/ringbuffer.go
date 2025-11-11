package codec

import (
	"fmt"
)

// RingBuffer is a circular buffer implementation matching the C++ CRingBuffer interface
// This provides the same functionality as the C++ template CRingBuffer<unsigned char>
type RingBuffer struct {
	name   string
	buffer []uint8
	length uint32
	iPtr   uint32 // Input pointer (where new data is written)
	oPtr   uint32 // Output pointer (where data is read from)
}

// NewRingBuffer creates a new ring buffer with the specified length and name
// Equivalent to C++ CRingBuffer constructor
func NewRingBuffer(length uint32, name string) *RingBuffer {
	if length == 0 {
		panic("RingBuffer length must be > 0")
	}

	return &RingBuffer{
		name:   name,
		buffer: make([]uint8, length),
		length: length,
		iPtr:   0,
		oPtr:   0,
	}
}

// AddData adds data to the ring buffer
// Returns false if there's not enough space (buffer overflow)
// Equivalent to C++ addData()
func (rb *RingBuffer) AddData(data []uint8) bool {
	nSamples := uint32(len(data))

	if nSamples >= rb.FreeSpace() {
		fmt.Printf("%s buffer overflow, clearing the buffer. (%d >= %d)\n",
			rb.name, nSamples, rb.FreeSpace())
		rb.Clear()
		return false
	}

	for i := uint32(0); i < nSamples; i++ {
		rb.buffer[rb.iPtr] = data[i]
		rb.iPtr++

		if rb.iPtr == rb.length {
			rb.iPtr = 0
		}
	}

	return true
}

// AddSingleByte adds a single byte to the ring buffer
// Convenient for adding tags and single bytes
func (rb *RingBuffer) AddSingleByte(b uint8) bool {
	if rb.FreeSpace() == 0 {
		fmt.Printf("%s buffer overflow for single byte\n", rb.name)
		rb.Clear()
		return false
	}

	rb.buffer[rb.iPtr] = b
	rb.iPtr++

	if rb.iPtr == rb.length {
		rb.iPtr = 0
	}

	return true
}

// GetData retrieves data from the ring buffer
// Returns false if there's not enough data (buffer underflow)
// Equivalent to C++ getData()
func (rb *RingBuffer) GetData(nSamples uint32) ([]uint8, bool) {
	if rb.DataSize() < nSamples {
		fmt.Printf("**** Underflow in %s ring buffer, %d < %d\n",
			rb.name, rb.DataSize(), nSamples)
		return nil, false
	}

	result := make([]uint8, nSamples)

	for i := uint32(0); i < nSamples; i++ {
		result[i] = rb.buffer[rb.oPtr]
		rb.oPtr++

		if rb.oPtr == rb.length {
			rb.oPtr = 0
		}
	}

	return result, true
}

// Peek looks at data in the buffer without removing it
// Returns false if there's not enough data
// Equivalent to C++ peek()
func (rb *RingBuffer) Peek(nSamples uint32) ([]uint8, bool) {
	if rb.DataSize() < nSamples {
		fmt.Printf("**** Underflow peek in %s ring buffer, %d < %d\n",
			rb.name, rb.DataSize(), nSamples)
		return nil, false
	}

	result := make([]uint8, nSamples)
	ptr := rb.oPtr

	for i := uint32(0); i < nSamples; i++ {
		result[i] = rb.buffer[ptr]
		ptr++

		if ptr == rb.length {
			ptr = 0
		}
	}

	return result, true
}

// Clear clears all data from the buffer
// Equivalent to C++ clear()
func (rb *RingBuffer) Clear() {
	rb.iPtr = 0
	rb.oPtr = 0

	// Clear the buffer contents
	for i := range rb.buffer {
		rb.buffer[i] = 0
	}
}

// FreeSpace returns the amount of free space in the buffer
// Equivalent to C++ freeSpace()
func (rb *RingBuffer) FreeSpace() uint32 {
	length := rb.length

	if rb.oPtr > rb.iPtr {
		length = rb.oPtr - rb.iPtr
	} else if rb.iPtr > rb.oPtr {
		length = rb.length - (rb.iPtr - rb.oPtr)
	}

	if length > rb.length {
		length = 0
	}

	return length
}

// DataSize returns the amount of data currently in the buffer
// Equivalent to C++ dataSize()
func (rb *RingBuffer) DataSize() uint32 {
	return rb.length - rb.FreeSpace()
}

// HasSpace checks if there's at least the specified amount of space
// Equivalent to C++ hasSpace()
func (rb *RingBuffer) HasSpace(length uint32) bool {
	return rb.FreeSpace() > length
}

// HasData checks if there's any data in the buffer
// Equivalent to C++ hasData()
func (rb *RingBuffer) HasData() bool {
	return rb.oPtr != rb.iPtr
}

// IsEmpty checks if the buffer is empty
// Equivalent to C++ isEmpty()
func (rb *RingBuffer) IsEmpty() bool {
	return rb.oPtr == rb.iPtr
}

// GetName returns the buffer name for debugging
func (rb *RingBuffer) GetName() string {
	return rb.name
}

// GetLength returns the buffer capacity
func (rb *RingBuffer) GetLength() uint32 {
	return rb.length
}