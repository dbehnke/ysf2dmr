package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

// YSFPacket represents a received YSF packet with metadata
type YSFPacket struct {
	Data     []byte
	Length   int
	FromAddr *net.UDPAddr
}

// YSFClient provides a goroutine-based YSF network client
type YSFClient struct {
	// Configuration
	config *YSFConfig
	debug  bool

	// Network
	conn       *net.UDPConn
	serverAddr *net.UDPAddr

	// Pre-built messages
	pollMsg   []byte
	unlinkMsg []byte

	// Channels for Go-native communication
	inbound  chan *YSFPacket    // Received packets from server
	outbound chan []byte        // Packets to send to server
	events   chan string        // Status/event notifications
	shutdown chan struct{}      // Shutdown signal

	// Timers
	pollTimer *time.Ticker

	// Sync
	mu      sync.RWMutex
	running bool
}

// YSFConfig holds YSF client configuration
type YSFConfig struct {
	ServerAddress string
	ServerPort    int
	LocalAddress  string
	LocalPort     int
	Callsign      string
}

// NewYSFClient creates a new goroutine-based YSF client
func NewYSFClient(config *YSFConfig, debug bool) (*YSFClient, error) {
	// Resolve server address
	serverAddr, err := net.ResolveUDPAddr("udp4",
		fmt.Sprintf("%s:%d", config.ServerAddress, config.ServerPort))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve YSF server: %v", err)
	}

	client := &YSFClient{
		config:     config,
		debug:      debug,
		serverAddr: serverAddr,

		// Buffered channels for smooth operation
		inbound:  make(chan *YSFPacket, 10),
		outbound: make(chan []byte, 10),
		events:   make(chan string, 10),
		shutdown: make(chan struct{}),
	}

	// Initialize pre-built messages
	client.initializeMessages()

	if debug {
		log.Printf("YSF Client created: server=%s, callsign=%s, localPort=%d",
			serverAddr.String(), config.Callsign, config.LocalPort)
	}

	return client, nil
}

// Start begins the YSF client goroutines
func (c *YSFClient) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("YSF client already running")
	}
	c.running = true
	c.mu.Unlock()

	// Bind UDP socket
	localAddr := &net.UDPAddr{
		IP:   net.ParseIP(c.config.LocalAddress),
		Port: c.config.LocalPort,
	}

	var err error
	c.conn, err = net.ListenUDP("udp4", localAddr)
	if err != nil {
		return fmt.Errorf("failed to bind YSF socket: %v", err)
	}

	if c.debug {
		log.Printf("YSF Client bound to %s", c.conn.LocalAddr().String())
	}

	// Start goroutines
	go c.networkReader(ctx)
	go c.networkWriter(ctx)
	go c.keepAliveManager(ctx)

	return nil
}

// networkReader goroutine - handles incoming packets with blocking reads
func (c *YSFClient) networkReader(ctx context.Context) {
	defer c.conn.Close()

	buffer := make([]byte, protocol.BUFFER_LENGTH)

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
					log.Printf("YSF read error: %v", err)
				}
				continue
			}

			// Validate source address (if we have a target)
			if c.serverAddr != nil {
				if !fromAddr.IP.Equal(c.serverAddr.IP) || fromAddr.Port != c.serverAddr.Port {
					if c.debug {
						log.Printf("YSF: Ignoring packet from %s (expected %s)",
							fromAddr.String(), c.serverAddr.String())
					}
					continue
				}
			}

			// Send packet to processing channel
			packetData := make([]byte, n)
			copy(packetData, buffer[:n])

			packet := &YSFPacket{
				Data:     packetData,
				Length:   n,
				FromAddr: fromAddr,
			}

			select {
			case c.inbound <- packet:
				if c.debug {
					log.Printf("YSF: Received %d bytes from %s", n, fromAddr.String())
				}
			default:
				if c.debug {
					log.Printf("YSF: Inbound channel full, dropping packet")
				}
			}
		}
	}
}

// networkWriter goroutine - handles outgoing packets
func (c *YSFClient) networkWriter(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case packet := <-c.outbound:
			if c.serverAddr != nil {
				_, err := c.conn.WriteToUDP(packet, c.serverAddr)
				if err != nil {
					if c.debug {
						log.Printf("YSF write error: %v", err)
					}
					// Signal connection problem
					c.events <- "WRITE_ERROR"
				}
			}
		}
	}
}

// keepAliveManager goroutine - handles YSF poll messages
func (c *YSFClient) keepAliveManager(ctx context.Context) {
	c.pollTimer = time.NewTicker(5 * time.Second)
	defer c.pollTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-c.pollTimer.C:
			// Send poll message
			c.sendPoll()
		}
	}
}

// GetInbound returns the inbound packet channel for external processing
func (c *YSFClient) GetInbound() <-chan *YSFPacket {
	return c.inbound
}

// GetEvents returns the events channel for status monitoring
func (c *YSFClient) GetEvents() <-chan string {
	return c.events
}

// Stop gracefully shuts down the YSF client
func (c *YSFClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.shutdown)
	c.running = false

	if c.pollTimer != nil {
		c.pollTimer.Stop()
	}
}

// sendPacket queues a packet for transmission
func (c *YSFClient) sendPacket(data []byte) {
	if len(data) != protocol.YSF_FRAME_LENGTH {
		if c.debug {
			log.Printf("YSF: Invalid frame length: %d", len(data))
		}
		return
	}

	select {
	case c.outbound <- data:
		// Packet queued successfully
	default:
		if c.debug {
			log.Printf("YSF: Outbound channel full, dropping packet")
		}
	}
}

// sendPoll sends a keep-alive poll message
func (c *YSFClient) sendPoll() {
	select {
	case c.outbound <- c.pollMsg:
		if c.debug {
			log.Printf("YSF: Poll sent to %s", c.serverAddr.String())
		}
	default:
		if c.debug {
			log.Printf("YSF: Outbound channel full, dropping poll")
		}
	}
}

// sendUnlink sends an unlink message
func (c *YSFClient) sendUnlink() {
	select {
	case c.outbound <- c.unlinkMsg:
		if c.debug {
			log.Printf("YSF: Unlink sent to %s", c.serverAddr.String())
		}
	default:
		if c.debug {
			log.Printf("YSF: Outbound channel full, dropping unlink")
		}
	}
}

// initializeMessages creates pre-built poll and unlink messages
func (c *YSFClient) initializeMessages() {
	// Pad callsign to YSF_CALLSIGN_LENGTH bytes (left-aligned, right-padded)
	callsign := c.config.Callsign
	if len(callsign) > protocol.YSF_CALLSIGN_LENGTH {
		callsign = callsign[:protocol.YSF_CALLSIGN_LENGTH]
	}
	paddedCallsign := make([]byte, protocol.YSF_CALLSIGN_LENGTH)
	copy(paddedCallsign, callsign)
	for i := len(callsign); i < protocol.YSF_CALLSIGN_LENGTH; i++ {
		paddedCallsign[i] = ' '
	}

	// Poll message: "YSFP" + 10-byte callsign
	c.pollMsg = make([]byte, protocol.YSF_POLL_MESSAGE_LENGTH)
	copy(c.pollMsg[0:], "YSFP")
	copy(c.pollMsg[4:], paddedCallsign)

	// Unlink message: "YSFU" + 10-byte callsign
	c.unlinkMsg = make([]byte, protocol.YSF_UNLINK_MESSAGE_LENGTH)
	copy(c.unlinkMsg[0:], "YSFU")
	copy(c.unlinkMsg[4:], paddedCallsign)
}

// WriteData queues YSF data for transmission
func (c *YSFClient) WriteData(data []byte) error {
	if len(data) != protocol.YSF_FRAME_LENGTH {
		return fmt.Errorf("invalid YSF frame length: expected %d, got %d",
			protocol.YSF_FRAME_LENGTH, len(data))
	}

	c.sendPacket(data)
	return nil
}