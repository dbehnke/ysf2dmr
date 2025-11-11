package network

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

// DMRNetwork provides DMR network communication equivalent to C++ CDMRNetwork
type DMRNetwork struct {
	// Network configuration
	address  net.IP
	port     int
	localId  uint32
	id       [4]byte // 4-byte repeater ID (big-endian)
	password string
	duplex   bool
	version  string
	debug    bool
	slot1    bool
	slot2    bool
	hwType   protocol.HWType
	enabled  bool

	// Network components
	socket       *UDPSocket
	buffer       []byte
	delayBuffers [3]*DelayBuffer // Index 0 unused, slots 1 and 2

	// State management
	status       protocol.DMRNetworkStatus
	retryTimer   *Timer
	timeoutTimer *Timer
	beacon       bool

	// Authentication
	salt []byte

	// Stream management
	streamId [3]uint32 // Index 0 unused, slots 1 and 2
	seqNo    uint8

	// Configuration data
	callsign     string
	rxFrequency  uint32
	txFrequency  uint32
	power        uint32
	colorCode    uint32
	latitude     float32
	longitude    float32
	height       int
	location     string
	description  string
	url          string
	options      string
}

// NewDMRNetwork creates a new DMR network instance
// Equivalent to C++ CDMRNetwork constructor
func NewDMRNetwork(address string, port int, localPort uint32, id uint32, password string,
	duplex bool, version string, debug bool, slot1, slot2 bool,
	hwType protocol.HWType, jitter int) (*DMRNetwork, error) {

	// Resolve address
	ip, err := Lookup(address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve DMR server address %s: %v", address, err)
	}

	// Use localPort for socket binding, or 0 if not specified
	bindPort := int(localPort)
	if localPort == 0 {
		bindPort = 0 // Use any available port
	}

	network := &DMRNetwork{
		address:   ip,
		port:      port,
		localId:   localPort, // Store the local port value for reference
		password:  password,
		duplex:    duplex,
		version:   version,
		debug:     debug,
		slot1:     slot1,
		slot2:     slot2,
		hwType:    hwType,
		enabled:   false,
		socket:    NewUDPSocket("", bindPort), // Bind to specified local port
		buffer:    make([]byte, 500),              // 500-byte receive buffer
		status:    protocol.DMR_WAITING_CONNECT,
		retryTimer: NewTimer(1000, 0, 0), // 1000 ticks per second
		timeoutTimer: NewTimer(1000, 0, 0),
		beacon:    false,
		salt:      make([]byte, protocol.DMR_SALT_LENGTH),
	}

	// Convert repeater ID to big-endian byte array
	binary.BigEndian.PutUint32(network.id[:], id)

	// Initialize delay buffers for each slot
	if slot1 {
		network.delayBuffers[1] = NewDelayBuffer(
			protocol.HOMEBREW_DATA_PACKET_LENGTH,
			protocol.DMR_SLOT_TIME,
			jitter)
	}
	if slot2 {
		network.delayBuffers[2] = NewDelayBuffer(
			protocol.HOMEBREW_DATA_PACKET_LENGTH,
			protocol.DMR_SLOT_TIME,
			jitter)
	}

	// Initialize random stream IDs
	rand.Seed(time.Now().UnixNano())
	network.streamId[1] = rand.Uint32()
	network.streamId[2] = rand.Uint32()

	if debug {
		log.Printf("DMR Network created: server=%s:%d, id=%d, localPort=%d, duplex=%v, slots=%v,%v",
			address, port, id, localPort, duplex, slot1, slot2)
	}

	return network, nil
}

// SetOptions sets the options string
// Equivalent to C++ CDMRNetwork::setOptions()
func (n *DMRNetwork) SetOptions(options string) {
	n.options = options
}

// SetConfig sets the repeater configuration
// Equivalent to C++ CDMRNetwork::setConfig()
func (n *DMRNetwork) SetConfig(callsign string, rxFrequency, txFrequency, power, colorCode uint32,
	latitude, longitude float32, height int, location, description, url string) {
	n.callsign = callsign
	n.rxFrequency = rxFrequency
	n.txFrequency = txFrequency
	n.power = power
	n.colorCode = colorCode
	n.latitude = latitude
	n.longitude = longitude
	n.height = height
	n.location = location
	n.description = description
	n.url = url

	if n.debug {
		log.Printf("DMR config: call=%s, freq=%d/%d, power=%d, cc=%d",
			callsign, rxFrequency, txFrequency, power, colorCode)
	}
}

// Open initiates the network connection
// Equivalent to C++ CDMRNetwork::open()
func (n *DMRNetwork) Open() error {
	if n.debug {
		log.Printf("Opening DMR network connection to %s:%d", n.address.String(), n.port)
	}

	// C++ behavior: don't open socket immediately, wait for retry timer
	n.status = protocol.DMR_WAITING_CONNECT
	n.timeoutTimer.Stop()
	n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)

	if n.debug {
		log.Printf("DMR: Waiting %d seconds before initial connection attempt", protocol.DMR_RETRY_TIMEOUT/1000)
	}

	return nil
}

// Enable enables or disables data reception
// Equivalent to C++ CDMRNetwork::enable()
func (n *DMRNetwork) Enable(enabled bool) {
	n.enabled = enabled
	if n.debug {
		log.Printf("DMR network enabled: %v", enabled)
	}
}

// IsConnected returns true if connected and authenticated
// Equivalent to C++ CDMRNetwork::isConnected()
func (n *DMRNetwork) IsConnected() bool {
	return n.status == protocol.DMR_RUNNING
}

// GetStatusString returns the current authentication status for debugging
func (n *DMRNetwork) GetStatusString() string {
	switch n.status {
	case protocol.DMR_WAITING_CONNECT:
		return "WAITING_CONNECT"
	case protocol.DMR_WAITING_LOGIN:
		return "WAITING_LOGIN"
	case protocol.DMR_WAITING_AUTHORISATION:
		return "WAITING_AUTHORISATION"
	case protocol.DMR_WAITING_CONFIG:
		return "WAITING_CONFIG"
	case protocol.DMR_WAITING_OPTIONS:
		return "WAITING_OPTIONS"
	case protocol.DMR_RUNNING:
		return "RUNNING"
	default:
		return "UNKNOWN"
	}
}

// Close closes the network connection
// Equivalent to C++ CDMRNetwork::close()
func (n *DMRNetwork) Close() {
	if n.debug {
		log.Printf("Closing DMR network connection")
	}

	if n.status == protocol.DMR_RUNNING {
		n.writeClose()
	}

	n.socket.Close()
	n.retryTimer.Stop()
	n.timeoutTimer.Stop()
	n.status = protocol.DMR_WAITING_CONNECT
}

// Read retrieves a DMR data frame
// Equivalent to C++ CDMRNetwork::read()
func (n *DMRNetwork) Read(data *protocol.DMRData) bool {
	if !n.enabled || n.status != protocol.DMR_RUNNING {
		return false
	}

	// Check each enabled slot's delay buffer
	for slotNo := 1; slotNo <= 2; slotNo++ {
		if (slotNo == 1 && !n.slot1) || (slotNo == 2 && !n.slot2) {
			continue
		}

		buffer := n.delayBuffers[slotNo]
		if buffer == nil {
			continue
		}

		tempBuffer := make([]byte, protocol.HOMEBREW_DATA_PACKET_LENGTH)
		status := buffer.GetData(tempBuffer)

		if status == protocol.BS_NO_DATA {
			continue
		}

		// Parse DMRD packet
		if !n.parseDMRDPacket(tempBuffer, data) {
			continue
		}

		// Set missing flag
		data.SetMissing(status == protocol.BS_MISSING)

		if n.debug && !data.IsMissing() {
			log.Printf("DMR Read: %s", data.String())
		}

		return true
	}

	return false
}

// Write sends a DMR data frame
// Equivalent to C++ CDMRNetwork::write()
func (n *DMRNetwork) Write(data *protocol.DMRData) error {
	if n.status != protocol.DMR_RUNNING {
		return fmt.Errorf("DMR network not running")
	}

	if !n.enabled {
		return nil // Silently ignore when disabled
	}

	// Check slot validity
	slotNo := data.GetSlotNo()
	if (slotNo == 1 && !n.slot1) || (slotNo == 2 && !n.slot2) {
		return nil // Silently ignore disabled slots
	}

	// Build DMRD packet
	packet := n.buildDMRDPacket(data)

	// Send packet
	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	err := n.socket.Write(packet, addr)
	if err != nil {
		if n.debug {
			log.Printf("DMR write error: %v", err)
		}
		n.status = protocol.DMR_WAITING_CONNECT
		n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
		return err
	}

	if n.debug {
		log.Printf("DMR Write: %s", data.String())
	}

	// Special handling for voice LC headers (send twice)
	if data.GetDataType() == protocol.DT_VOICE_LC_HEADER {
		time.Sleep(5 * time.Millisecond) // Small delay
		n.socket.Write(packet, addr)     // Send again
	}

	return nil
}

// WritePosition sends a position packet
// Equivalent to C++ CDMRNetwork::writePosition()
func (n *DMRNetwork) WritePosition(id uint32, gpsData []byte) error {
	if n.status != protocol.DMR_RUNNING {
		return fmt.Errorf("DMR network not running")
	}

	if len(gpsData) < 7 {
		return fmt.Errorf("invalid GPS data length: %d", len(gpsData))
	}

	packet := make([]byte, protocol.NETWORK_POSITION_LENGTH)
	copy(packet[0:4], protocol.NETWORK_MAGIC_POSITION)
	copy(packet[4:8], n.id[:])
	packet[8] = byte(id >> 16)
	packet[9] = byte(id >> 8)
	packet[10] = byte(id)
	copy(packet[11:18], gpsData[:7])

	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	return n.socket.Write(packet, addr)
}

// WriteTalkerAlias sends a talker alias packet
// Equivalent to C++ CDMRNetwork::writeTalkerAlias()
func (n *DMRNetwork) WriteTalkerAlias(id uint32, aliasType uint8, aliasData []byte) error {
	if n.status != protocol.DMR_RUNNING {
		return fmt.Errorf("DMR network not running")
	}

	if len(aliasData) < 7 {
		return fmt.Errorf("invalid alias data length: %d", len(aliasData))
	}

	packet := make([]byte, protocol.NETWORK_TALKERALIAS_LENGTH)
	copy(packet[0:4], protocol.NETWORK_MAGIC_TALKERALIAS)
	copy(packet[4:8], n.id[:])
	packet[8] = byte(id >> 16)
	packet[9] = byte(id >> 8)
	packet[10] = byte(id)
	packet[11] = aliasType
	copy(packet[12:19], aliasData[:7])

	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	return n.socket.Write(packet, addr)
}

// WantsBeacon returns and clears the beacon flag
// Equivalent to C++ CDMRNetwork::wantsBeacon()
func (n *DMRNetwork) WantsBeacon() bool {
	beacon := n.beacon
	n.beacon = false
	return beacon
}

// Reset resets the delay buffer for a specific slot
// Equivalent to C++ CDMRNetwork::reset()
func (n *DMRNetwork) Reset(slotNo uint8) {
	if slotNo >= 1 && slotNo <= 2 && n.delayBuffers[slotNo] != nil {
		n.delayBuffers[slotNo].Reset()
		n.streamId[slotNo] = rand.Uint32()
		if n.debug {
			log.Printf("DMR slot %d reset, new stream ID: 0x%08X", slotNo, n.streamId[slotNo])
		}
	}
}

// Clock processes network events and timers
// Equivalent to C++ CDMRNetwork::clock()
func (n *DMRNetwork) Clock(ms int) {
	// Update timers
	n.retryTimer.Clock(ms)
	n.timeoutTimer.Clock(ms)

	// Update delay buffers
	for i := 1; i <= 2; i++ {
		if n.delayBuffers[i] != nil {
			n.delayBuffers[i].Clock(ms)
		}
	}

	// Handle timer events
	if n.retryTimer.HasExpired() {
		n.retryTimer.Stop()
		n.handleRetryTimeout()
	}

	if n.timeoutTimer.HasExpired() {
		n.timeoutTimer.Stop()
		n.handleConnectionTimeout()
	}

	// Process incoming packets
	n.processIncomingPackets()
}

// processIncomingPackets handles incoming UDP packets
func (n *DMRNetwork) processIncomingPackets() {
	// Only process packets if socket should be open (after initial connection attempt)
	if n.status == protocol.DMR_WAITING_CONNECT {
		return // Socket not open yet, wait for retry timer
	}

	for {
		bytesRead, fromAddr, err := n.socket.Read(n.buffer)
		if err != nil {
			if n.debug && err.Error() != "socket not open" {
				log.Printf("DMR socket read error: %v", err)
			}
			// If socket not open, don't change status (wait for timer)
			if err.Error() == "socket not open" {
				return
			}
			n.status = protocol.DMR_WAITING_CONNECT
			n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
			return
		}

		if bytesRead == 0 {
			break // No more data
		}

		// Debug: Log ALL received packets
		if n.debug {
			log.Printf("DMR: Received %d bytes from %s:%d (expecting %s:%d)",
				bytesRead, fromAddr.IP, fromAddr.Port, n.address.String(), n.port)
		}

		// Validate source address
		if !fromAddr.IP.Equal(n.address) || fromAddr.Port != n.port {
			if n.debug {
				log.Printf("DMR: Ignoring packet from unexpected source: %s:%d (expected %s:%d)",
					fromAddr.IP, fromAddr.Port, n.address.String(), n.port)
			}
			continue
		}

		packet := n.buffer[:bytesRead]
		if n.debug {
			log.Printf("DMR: Processing valid packet: %d bytes", bytesRead)
		}
		n.processPacket(packet)
	}
}

// Continue in next part due to length...