package network

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

// YSFNetwork provides YSF network communication equivalent to C++ CYSFNetwork
type YSFNetwork struct {
	callsign    string        // 10-byte callsign (space-padded)
	socket      *UDPSocket    // UDP socket instance
	debug       bool          // Debug flag for logging
	address     net.IP        // Destination IP address
	port        int           // Destination port
	pollMsg     []byte        // Pre-built 14-byte poll message
	unlinkMsg   []byte        // Pre-built 14-byte unlink message
	buffer      *RingBuffer   // Circular buffer for incoming data
	tempBuffer  []byte        // Temporary buffer for UDP reads
}

// NewYSFNetworkClient creates a YSF network client that connects to a remote address/port
// Equivalent to C++ CYSFNetwork(const std::string& address, unsigned int port, const std::string& callsign, bool debug)
func NewYSFNetworkClient(address string, port int, callsign string, debug bool) (*YSFNetwork, error) {
	network := &YSFNetwork{
		callsign:   padCallsign(callsign),
		socket:     NewUDPSocket("", 0), // Bind to any local address/port
		debug:      debug,
		port:       port,
		buffer:     NewRingBuffer(protocol.RING_BUFFER_LENGTH, "YSFNetwork"),
		tempBuffer: make([]byte, protocol.BUFFER_LENGTH),
	}

	// Parse destination address
	ip := net.ParseIP(address)
	if ip == nil {
		// Try to resolve hostname
		var err error
		ip, err = Lookup(address)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve address %s: %v", address, err)
		}
	}
	network.address = ip

	// Initialize poll and unlink messages
	network.initializeMessages()

	if debug {
		log.Printf("YSF Network Client created: callsign=%s, destination=%s:%d",
			network.callsign, address, port)
	}

	return network, nil
}

// NewYSFNetworkServer creates a YSF network server that listens on a local address and port
// Equivalent to C++ CYSFNetwork(const std::string& localAddress, unsigned int localPort, const std::string& callsign, bool debug)
func NewYSFNetworkServer(localAddress string, port int, callsign string, debug bool) *YSFNetwork {
	network := &YSFNetwork{
		callsign:   padCallsign(callsign),
		socket:     NewUDPSocket(localAddress, port),
		debug:      debug,
		port:       0, // No destination initially
		buffer:     NewRingBuffer(protocol.RING_BUFFER_LENGTH, "YSFNetwork"),
		tempBuffer: make([]byte, protocol.BUFFER_LENGTH),
	}

	// Initialize poll and unlink messages
	network.initializeMessages()

	if debug {
		log.Printf("YSF Network Server created: callsign=%s, listen_address=%s:%d",
			network.callsign, localAddress, port)
	}

	return network
}

// GetCallsign returns the configured callsign
// Equivalent to C++ CYSFNetwork::getCallsign()
func (n *YSFNetwork) GetCallsign() string {
	return strings.TrimSpace(n.callsign) // Remove padding spaces
}

// Open creates and binds the UDP socket
// Equivalent to C++ CYSFNetwork::open()
func (n *YSFNetwork) Open() error {
	if n.debug {
		log.Printf("Opening YSF network connection")
	}
	return n.socket.Open()
}

// SetDestination stores destination address and port for outbound packets
// Equivalent to C++ CYSFNetwork::setDestination()
func (n *YSFNetwork) SetDestination(address net.IP, port int) {
	n.address = address
	n.port = port

	if n.debug {
		log.Printf("YSF destination set to %s:%d", address.String(), port)
	}
}

// SetDestinationByString parses address string and sets destination
func (n *YSFNetwork) SetDestinationByString(address string, port int) error {
	ip := net.ParseIP(address)
	if ip == nil {
		// Try to resolve hostname
		var err error
		ip, err = Lookup(address)
		if err != nil {
			return fmt.Errorf("failed to resolve address %s: %v", address, err)
		}
	}
	n.SetDestination(ip, port)
	return nil
}

// ClearDestination disables outbound packets
// Equivalent to C++ CYSFNetwork::clearDestination()
func (n *YSFNetwork) ClearDestination() {
	n.address = nil
	n.port = 0

	if n.debug {
		log.Printf("YSF destination cleared")
	}
}

// Write sends 155-byte YSF data frame to destination
// Equivalent to C++ CYSFNetwork::write()
func (n *YSFNetwork) Write(data []byte) error {
	if n.port == 0 {
		return nil // No destination set
	}

	if len(data) != protocol.YSF_FRAME_LENGTH {
		return fmt.Errorf("invalid YSF frame length: expected %d, got %d",
			protocol.YSF_FRAME_LENGTH, len(data))
	}

	if n.debug {
		log.Printf("YSF Network write: %d bytes to %s:%d", len(data), n.address.String(), n.port)
		// Could add hex dump here like C++ CUtils::dump()
	}

	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	return n.socket.Write(data, addr)
}

// WritePoll sends 14-byte poll message to destination
// Equivalent to C++ CYSFNetwork::writePoll()
func (n *YSFNetwork) WritePoll() error {
	if n.port == 0 {
		return nil // No destination set
	}

	if n.debug {
		log.Printf("YSF Network poll sent to %s:%d", n.address.String(), n.port)
	}

	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	return n.socket.Write(n.pollMsg, addr)
}

// WriteUnlink sends 14-byte unlink message to destination
// Equivalent to C++ CYSFNetwork::writeUnlink()
func (n *YSFNetwork) WriteUnlink() error {
	if n.port == 0 {
		return nil // No destination set
	}

	if n.debug {
		log.Printf("YSF Network unlink sent to %s:%d", n.address.String(), n.port)
	}

	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	return n.socket.Write(n.unlinkMsg, addr)
}

// Read retrieves data from the ring buffer
// Equivalent to C++ CYSFNetwork::read()
// Returns number of bytes read (0 if buffer empty)
func (n *YSFNetwork) Read(data []byte) int {
	// Get length-prefixed data from ring buffer
	length, ok := n.buffer.GetLength(data)
	if !ok {
		return 0 // No data available
	}

	if n.debug && length > 0 {
		log.Printf("YSF Network read: %d bytes", length)
	}

	return length
}

// Clock processes incoming UDP packets and stores them in the ring buffer
// Equivalent to C++ CYSFNetwork::clock()
func (n *YSFNetwork) Clock(ms int) {
	// Poll UDP socket for incoming data
	for {
		bytesRead, fromAddr, err := n.socket.Read(n.tempBuffer)
		if err != nil {
			if n.debug {
				log.Printf("YSF Network clock error: %v", err)
			}
			break
		}

		if bytesRead == 0 {
			break // No more data available
		}

		// Validate sender if destination is set (for client mode)
		if n.port != 0 && n.address != nil {
			if !fromAddr.IP.Equal(n.address) || fromAddr.Port != n.port {
				if n.debug {
					log.Printf("YSF Network: packet from unexpected source %s:%d (expected %s:%d)",
						fromAddr.IP.String(), fromAddr.Port, n.address.String(), n.port)
				}
				continue // Ignore packet from wrong source
			}
		}

		if n.debug {
			log.Printf("YSF Network received: %d bytes from %s:%d",
				bytesRead, fromAddr.IP.String(), fromAddr.Port)
		}

		// Store in ring buffer with length prefix
		packetData := n.tempBuffer[:bytesRead]
		if !n.buffer.AddLength(packetData) {
			if n.debug {
				log.Printf("YSF Network: ring buffer full, dropping packet")
			}
		}
	}
}

// Close closes the UDP socket
// Equivalent to C++ CYSFNetwork::close()
func (n *YSFNetwork) Close() {
	if n.debug {
		log.Printf("Closing YSF network connection")
	}
	n.socket.Close()
}

// padCallsign pads callsign to YSF_CALLSIGN_LENGTH bytes with spaces
func padCallsign(callsign string) string {
	if len(callsign) >= protocol.YSF_CALLSIGN_LENGTH {
		return callsign[:protocol.YSF_CALLSIGN_LENGTH]
	}
	return callsign + strings.Repeat(" ", protocol.YSF_CALLSIGN_LENGTH-len(callsign))
}

// initializeMessages creates pre-built poll and unlink messages
func (n *YSFNetwork) initializeMessages() {
	// Poll message: "YSFP" + 10-byte callsign
	n.pollMsg = make([]byte, protocol.YSF_POLL_MESSAGE_LENGTH)
	copy(n.pollMsg[0:], "YSFP")
	copy(n.pollMsg[4:], n.callsign)

	// Unlink message: "YSFU" + 10-byte callsign
	n.unlinkMsg = make([]byte, protocol.YSF_UNLINK_MESSAGE_LENGTH)
	copy(n.unlinkMsg[0:], "YSFU")
	copy(n.unlinkMsg[4:], n.callsign)
}

// HasData returns true if ring buffer contains data
func (n *YSFNetwork) HasData() bool {
	return n.buffer.HasData()
}

// String returns string representation for debugging
func (n *YSFNetwork) String() string {
	if n.port == 0 {
		return fmt.Sprintf("YSFNetwork[%s]: server mode", strings.TrimSpace(n.callsign))
	}
	return fmt.Sprintf("YSFNetwork[%s]: client mode -> %s:%d",
		strings.TrimSpace(n.callsign), n.address.String(), n.port)
}