package network

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

// DMRPacket represents a received DMR packet with metadata
type DMRPacket struct {
	Data     []byte
	Length   int
	FromAddr *net.UDPAddr
}

// DMRClient provides a goroutine-based DMR network client
type DMRClient struct {
	// Configuration
	config    *DMRConfig
	debug     bool

	// Network
	conn      *net.UDPConn
	serverAddr *net.UDPAddr

	// State
	status    protocol.DMRNetworkStatus
	salt      []byte

	// Channels for Go-native communication
	inbound   chan *DMRPacket    // Data packets for external processing
	outbound  chan []byte        // Packets to send to server
	events    chan string        // Status/event notifications
	shutdown  chan struct{}      // Shutdown signal
	authPackets chan []byte      // Internal authentication packets

	// Timers - using Go's native timing
	retryTimer    *time.Timer
	timeoutTimer  *time.Timer

	// Sync
	mu         sync.RWMutex
	running    bool
}

// DMRConfig holds DMR client configuration
type DMRConfig struct {
	ServerAddress string
	ServerPort    int
	LocalPort     int
	RepeaterID    uint32
	Password      string
	Callsign      string

	// Repeater info
	RxFrequency uint32
	TxFrequency uint32
	Power       uint32
	ColorCode   uint32
	Latitude    float32
	Longitude   float32
	Height      int
	Location    string
	Description string
	URL         string
	Options     string
}

// NewDMRClient creates a new goroutine-based DMR client
func NewDMRClient(config *DMRConfig, debug bool) (*DMRClient, error) {
	// Resolve server address
	serverAddr, err := net.ResolveUDPAddr("udp4",
		fmt.Sprintf("%s:%d", config.ServerAddress, config.ServerPort))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve DMR server: %v", err)
	}

	client := &DMRClient{
		config:     config,
		debug:      debug,
		serverAddr: serverAddr,
		status:     protocol.DMR_WAITING_CONNECT,
		salt:       make([]byte, protocol.DMR_SALT_LENGTH),

		// Buffered channels for smooth operation
		inbound:     make(chan *DMRPacket, 10),
		outbound:    make(chan []byte, 10),
		events:      make(chan string, 10),
		shutdown:    make(chan struct{}),
		authPackets: make(chan []byte, 10),
	}

	if debug {
		log.Printf("DMR Client created: server=%s, id=%d, localPort=%d",
			serverAddr.String(), config.RepeaterID, config.LocalPort)
	}

	return client, nil
}

// Start begins the DMR client goroutines
func (c *DMRClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("DMR client already running")
	}
	c.running = true
	c.mu.Unlock()

	// Bind UDP socket
	localAddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: c.config.LocalPort,
	}

	var err error
	c.conn, err = net.ListenUDP("udp4", localAddr)
	if err != nil {
		return fmt.Errorf("failed to bind DMR socket: %v", err)
	}

	if c.debug {
		log.Printf("DMR Client bound to %s", c.conn.LocalAddr().String())
	}

	// Start goroutines
	go c.networkReader(ctx)
	go c.networkWriter(ctx)
	go c.authenticationManager(ctx)

	return nil
}

// networkReader goroutine - handles incoming packets with blocking reads
func (c *DMRClient) networkReader(ctx context.Context) {
	defer c.conn.Close()

	buffer := make([]byte, 500)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		default:
			// Blocking read with reasonable timeout
			c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

			n, fromAddr, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is normal, continue reading
				}
				if c.debug {
					log.Printf("DMR read error: %v", err)
				}
				continue
			}

			// Validate source address
			if !fromAddr.IP.Equal(c.serverAddr.IP) || fromAddr.Port != c.serverAddr.Port {
				if c.debug {
					log.Printf("DMR: Ignoring packet from %s (expected %s)",
						fromAddr.String(), c.serverAddr.String())
				}
				continue
			}

			// Send packet to appropriate processing channel
			packetData := make([]byte, n)
			copy(packetData, buffer[:n])

			// Separate authentication packets from data packets
			isAuthPacket := c.isAuthenticationPacket(packetData)

			if isAuthPacket {
				// Authentication packets go to internal processing
				select {
				case c.authPackets <- packetData:
					if c.debug {
						log.Printf("DMR: Received auth packet %d bytes from %s", n, fromAddr.String())
					}
				default:
					if c.debug {
						log.Printf("DMR: Auth channel full, dropping packet")
					}
				}
			} else {
				// Data packets go to external processing
				packet := &DMRPacket{
					Data:     packetData,
					Length:   n,
					FromAddr: fromAddr,
				}

				select {
				case c.inbound <- packet:
					if c.debug {
						log.Printf("DMR: Received data packet %d bytes from %s", n, fromAddr.String())
					}
				default:
					if c.debug {
						log.Printf("DMR: Inbound channel full, dropping packet")
					}
				}
			}
		}
	}
}

// networkWriter goroutine - handles outgoing packets
func (c *DMRClient) networkWriter(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case packet := <-c.outbound:
			_, err := c.conn.WriteToUDP(packet, c.serverAddr)
			if err != nil {
				if c.debug {
					log.Printf("DMR write error: %v", err)
				}
				// Signal connection problem
				c.events <- "WRITE_ERROR"
			}
		}
	}
}

// isAuthenticationPacket determines if a packet is authentication-related
func (c *DMRClient) isAuthenticationPacket(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// Check for authentication packet types
	if len(data) >= 6 && string(data[:6]) == "RPTACK" {
		return true
	}
	if len(data) >= 6 && string(data[:6]) == "MSTNAK" {
		return true
	}
	if len(data) >= 7 && string(data[:7]) == "MSTPONG" {
		return true
	}
	if len(data) >= 5 && string(data[:5]) == "MSTCL" {
		return true
	}

	return false // Data packets (DMRD, etc.)
}

// authenticationManager goroutine - handles DMR authentication state machine
func (c *DMRClient) authenticationManager(ctx context.Context) {
	// Start with connection delay
	c.retryTimer = time.NewTimer(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-c.retryTimer.C:
			c.handleRetryTimeout()
		case packet := <-c.authPackets:
			c.processPacket(packet)
		}
	}
}

// GetInbound returns the inbound packet channel for external processing
func (c *DMRClient) GetInbound() <-chan *DMRPacket {
	return c.inbound
}

// GetEvents returns the events channel for status monitoring
func (c *DMRClient) GetEvents() <-chan string {
	return c.events
}

// IsConnected returns true if authenticated and running
func (c *DMRClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status == protocol.DMR_RUNNING
}

// GetStatus returns current authentication status
func (c *DMRClient) GetStatus() protocol.DMRNetworkStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Stop gracefully shuts down the DMR client
func (c *DMRClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.shutdown)
	c.running = false

	if c.retryTimer != nil {
		c.retryTimer.Stop()
	}
	if c.timeoutTimer != nil {
		c.timeoutTimer.Stop()
	}
}

// sendPacket queues a packet for transmission
func (c *DMRClient) sendPacket(data []byte) {
	select {
	case c.outbound <- data:
		// Packet queued successfully
	default:
		if c.debug {
			log.Printf("DMR: Outbound channel full, dropping packet")
		}
	}
}

// handleRetryTimeout implements the authentication state machine
func (c *DMRClient) handleRetryTimeout() {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.status {
	case protocol.DMR_WAITING_CONNECT:
		if c.debug {
			log.Printf("DMR: Starting authentication - sending login")
		}
		c.sendLogin()
		c.status = protocol.DMR_WAITING_LOGIN
		c.timeoutTimer = time.NewTimer(60 * time.Second) // Connection timeout

	case protocol.DMR_WAITING_LOGIN:
		if c.debug {
			log.Printf("DMR: Retrying login")
		}
		c.sendLogin()

	case protocol.DMR_WAITING_AUTHORISATION:
		if c.debug {
			log.Printf("DMR: Retrying authorization")
		}
		c.sendAuth()

	case protocol.DMR_WAITING_CONFIG:
		if c.debug {
			log.Printf("DMR: Retrying configuration")
		}
		c.sendConfig()

	case protocol.DMR_RUNNING:
		if c.debug {
			log.Printf("DMR: Sending ping")
		}
		c.sendPing()

	default:
		if c.debug {
			log.Printf("DMR: Unknown state, resetting to WAITING_CONNECT")
		}
		c.status = protocol.DMR_WAITING_CONNECT
	}

	// Reset retry timer for next attempt
	c.retryTimer.Reset(10 * time.Second)
}

// processPacket handles incoming DMR packets
func (c *DMRClient) processPacket(data []byte) {
	if len(data) < 4 {
		return // Invalid packet
	}

	if c.debug {
		maxLen := len(data)
		if maxLen > 16 {
			maxLen = 16
		}
		log.Printf("DMR: Raw packet (%d bytes): %02X | String: %q", len(data), data[:maxLen], string(data[:min(6, len(data))]))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for 6-byte RPTACK first (most common)
	if len(data) >= 6 && string(data[:6]) == "RPTACK" {
		if c.debug {
			log.Printf("DMR: Processing RPTACK packet (%d bytes): %02X", len(data), data)
		}
		c.handleRPTACK(data)
		return
	}

	// Check for 4-byte magic packets
	if len(data) >= 4 {
		magic := string(data[:4])
		switch magic {
		case "MSTN": // MSTNAK
			if len(data) >= 6 && string(data[:6]) == "MSTNAK" {
				c.handleMSTNAK(data)
			}
		case "MSTP": // MSTPONG
			if len(data) >= 7 && string(data[:7]) == "MSTPONG" {
				c.handleMSTPONG(data)
			}
		case "MSTC": // MSTCL
			if len(data) >= 5 && string(data[:5]) == "MSTCL" {
				c.handleMSTCL(data)
			}
		default:
			if c.debug {
				// Show more packet info for debugging
				maxLen := len(data)
				if maxLen > 16 {
					maxLen = 16
				}
				log.Printf("DMR: Unknown packet type with data (%d bytes): %02X", len(data), data[:maxLen])
			}
		}
	}
}

// Helper function for Go versions that don't have min built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Authentication packet handlers
func (c *DMRClient) handleRPTACK(packet []byte) {
	if c.debug {
		log.Printf("DMR: Received RPTACK in state %s, packet length %d", c.getStatusString(), len(packet))
		log.Printf("DMR: RPTACK packet hex: %02X", packet)
	}

	switch c.status {
	case protocol.DMR_WAITING_LOGIN:
		// Extract salt
		if len(packet) >= 10 {
			copy(c.salt, packet[6:10])
			if c.debug {
				log.Printf("DMR: Received salt: %02X", c.salt)
			}
		}
		c.sendAuth()
		c.status = protocol.DMR_WAITING_AUTHORISATION

	case protocol.DMR_WAITING_AUTHORISATION:
		c.sendConfig()
		c.status = protocol.DMR_WAITING_CONFIG

	case protocol.DMR_WAITING_CONFIG:
		if c.config.Options != "" {
			c.sendOptions()
			c.status = protocol.DMR_WAITING_OPTIONS
		} else {
			c.status = protocol.DMR_RUNNING
			c.timeoutTimer.Reset(60 * time.Second)
			if c.debug {
				log.Printf("DMR: Authentication complete - RUNNING")
			}
			c.events <- "AUTHENTICATED"
		}

	case protocol.DMR_WAITING_OPTIONS:
		c.status = protocol.DMR_RUNNING
		c.timeoutTimer.Reset(60 * time.Second)
		if c.debug {
			log.Printf("DMR: Authentication complete - RUNNING")
		}
		c.events <- "AUTHENTICATED"
	}
}

func (c *DMRClient) handleMSTNAK(packet []byte) {
	if c.debug {
		log.Printf("DMR: Received MSTNAK - authentication failed")
	}
	c.status = protocol.DMR_WAITING_LOGIN
	c.retryTimer.Reset(10 * time.Second)
	c.events <- "AUTH_FAILED"
}

func (c *DMRClient) handleMSTPONG(packet []byte) {
	if c.debug {
		log.Printf("DMR: Received MSTPONG - connection alive")
	}
	c.timeoutTimer.Reset(60 * time.Second)
}

func (c *DMRClient) handleMSTCL(packet []byte) {
	if c.debug {
		log.Printf("DMR: Received MSTCL - master closing")
	}
	c.status = protocol.DMR_WAITING_CONNECT
	c.retryTimer.Reset(10 * time.Second)
	c.events <- "DISCONNECTED"
}

func (c *DMRClient) getStatusString() string {
	switch c.status {
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

// Packet building methods
func (c *DMRClient) sendLogin() {
	packet := make([]byte, protocol.NETWORK_LOGIN_LENGTH)
	copy(packet[0:4], "RPTL")

	// Convert repeater ID to big-endian bytes
	packet[4] = byte(c.config.RepeaterID >> 24)
	packet[5] = byte(c.config.RepeaterID >> 16)
	packet[6] = byte(c.config.RepeaterID >> 8)
	packet[7] = byte(c.config.RepeaterID)

	c.sendPacket(packet)
	if c.debug {
		log.Printf("DMR: Sent login packet (ID: %d)", c.config.RepeaterID)
	}
}

func (c *DMRClient) sendAuth() {
	// Calculate SHA256(salt + password)
	hasher := sha256.New()
	hasher.Write(c.salt)
	hasher.Write([]byte(c.config.Password))
	hash := hasher.Sum(nil)

	packet := make([]byte, protocol.NETWORK_AUTH_LENGTH)
	copy(packet[0:4], "RPTK")

	// Repeater ID
	packet[4] = byte(c.config.RepeaterID >> 24)
	packet[5] = byte(c.config.RepeaterID >> 16)
	packet[6] = byte(c.config.RepeaterID >> 8)
	packet[7] = byte(c.config.RepeaterID)

	// SHA256 hash
	copy(packet[8:40], hash[:32])

	c.sendPacket(packet)
	if c.debug {
		log.Printf("DMR: Sent auth packet")
	}
}

func (c *DMRClient) sendConfig() {
	packet := make([]byte, protocol.NETWORK_CONFIG_LENGTH)

	// Magic and ID
	copy(packet[0:4], "RPTC")
	packet[4] = byte(c.config.RepeaterID >> 24)
	packet[5] = byte(c.config.RepeaterID >> 16)
	packet[6] = byte(c.config.RepeaterID >> 8)
	packet[7] = byte(c.config.RepeaterID)

	// Callsign (8 bytes, left-aligned with right padding)
	callsign := strings.ToUpper(c.config.Callsign)
	if len(callsign) > 8 {
		callsign = callsign[:8]
	}
	copy(packet[8:], callsign)
	for i := len(callsign); i < 8; i++ {
		packet[8+i] = ' '
	}

	// Frequencies, power, color code, location data, etc.
	// Use the same formatting as the fixed packet format
	rxFreqStr := fmt.Sprintf("%09d", c.config.RxFrequency)
	txFreqStr := fmt.Sprintf("%09d", c.config.TxFrequency)
	copy(packet[16:25], rxFreqStr)
	copy(packet[25:34], txFreqStr)

	powerStr := fmt.Sprintf("%02d", c.config.Power)
	copy(packet[34:36], powerStr)

	ccStr := fmt.Sprintf("%02d", c.config.ColorCode)
	copy(packet[36:38], ccStr)

	latStr := fmt.Sprintf("%08f", c.config.Latitude)
	if len(latStr) > 8 {
		latStr = latStr[:8]
	}
	copy(packet[38:46], latStr)

	lngStr := fmt.Sprintf("%09f", c.config.Longitude)
	if len(lngStr) > 9 {
		lngStr = lngStr[:9]
	}
	copy(packet[46:55], lngStr)

	heightStr := fmt.Sprintf("%03d", c.config.Height)
	copy(packet[55:58], heightStr)

	// Location, description, URL, version strings
	// Truncate if too long
	location := c.config.Location
	if len(location) > 20 {
		location = location[:20]
	}
	copy(packet[58:78], location)

	description := c.config.Description
	if len(description) > 19 {
		description = description[:19]
	}
	copy(packet[78:97], description)

	packet[97] = '3' // Both slots enabled

	url := c.config.URL
	if len(url) > 124 {
		url = url[:124]
	}
	copy(packet[98:222], url)

	copy(packet[222:262], "1.0.0-go-goroutines") // Version

	copy(packet[262:302], "HOMEBREW") // Hardware type

	c.sendPacket(packet)
	if c.debug {
		log.Printf("DMR: Sent config packet")
	}
}

func (c *DMRClient) sendOptions() {
	packet := make([]byte, 8+len(c.config.Options)+1)
	copy(packet[0:4], "RPTO")
	packet[4] = byte(c.config.RepeaterID >> 24)
	packet[5] = byte(c.config.RepeaterID >> 16)
	packet[6] = byte(c.config.RepeaterID >> 8)
	packet[7] = byte(c.config.RepeaterID)
	copy(packet[8:], c.config.Options)
	packet[len(packet)-1] = 0 // Null terminator

	c.sendPacket(packet)
	if c.debug {
		log.Printf("DMR: Sent options packet")
	}
}

func (c *DMRClient) sendPing() {
	packet := make([]byte, protocol.NETWORK_PING_LENGTH)
	copy(packet[0:7], "RPTPING")
	packet[7] = byte(c.config.RepeaterID >> 24)
	packet[8] = byte(c.config.RepeaterID >> 16)
	packet[9] = byte(c.config.RepeaterID >> 8)
	packet[10] = byte(c.config.RepeaterID)

	c.sendPacket(packet)
	if c.debug {
		log.Printf("DMR: Sent ping packet")
	}
}