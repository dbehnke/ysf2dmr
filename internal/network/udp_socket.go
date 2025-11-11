package network

import (
	"fmt"
	"log"
	"net"
	"time"
)

// UDPSocket provides non-blocking UDP I/O operations equivalent to C++ CUDPSocket
type UDPSocket struct {
	conn      *net.UDPConn
	address   string
	port      int
	localAddr *net.UDPAddr
}

// NewUDPSocket creates a UDP socket with specific address and port (client mode)
func NewUDPSocket(address string, port int) *UDPSocket {
	return &UDPSocket{
		address: address,
		port:    port,
	}
}

// NewUDPSocketServer creates a UDP socket for server mode (any address, specific port)
func NewUDPSocketServer(port int) *UDPSocket {
	return &UDPSocket{
		address: "",
		port:    port,
	}
}

// Open creates the UDP socket with C++ equivalent binding behavior
// Equivalent to C++ CUDPSocket::open()
func (s *UDPSocket) Open() error {
	var err error

	// C++ behavior: Only bind if port > 0, otherwise create unbound socket
	if s.port > 0 {
		// Bind to specific port (server mode or configured client)
		if s.address == "" {
			// Bind to any address (server mode)
			s.localAddr = &net.UDPAddr{
				IP:   net.IPv4zero,
				Port: s.port,
			}
		} else {
			// Bind to specific address and port
			s.localAddr = &net.UDPAddr{
				IP:   net.ParseIP(s.address),
				Port: s.port,
			}
			if s.localAddr.IP == nil {
				return fmt.Errorf("invalid address: %s", s.address)
			}
		}

		// Create bound UDP socket with SO_REUSEADDR equivalent, force IPv4
		s.conn, err = net.ListenUDP("udp4", s.localAddr)
		if err != nil {
			log.Printf("Error opening bound UDP socket: %v", err)
			return err
		}

		log.Printf("UDP socket bound to %s", s.conn.LocalAddr().String())
	} else {
		// Create unbound socket (client mode with ephemeral port)
		// This matches C++ behavior when m_port == 0
		s.localAddr = &net.UDPAddr{
			IP:   net.IPv4zero,
			Port: 0, // Let OS assign ephemeral port on first send
		}

		s.conn, err = net.ListenUDP("udp4", s.localAddr)
		if err != nil {
			log.Printf("Error opening unbound UDP socket: %v", err)
			return err
		}

		log.Printf("UDP socket created (unbound) on %s", s.conn.LocalAddr().String())
	}

	// Set non-blocking mode by using read timeout
	err = s.conn.SetReadDeadline(time.Now())
	if err != nil {
		s.conn.Close()
		return err
	}

	return nil
}

// Read performs non-blocking read operation
// Equivalent to C++ CUDPSocket::read() with select() and zero timeout
// Returns: bytes read (>0), 0 if no data available, -1 on error
func (s *UDPSocket) Read(buffer []byte) (int, *net.UDPAddr, error) {
	if s.conn == nil {
		return -1, nil, fmt.Errorf("socket not open")
	}

	// Set immediate timeout for non-blocking behavior
	s.conn.SetReadDeadline(time.Now())

	n, addr, err := s.conn.ReadFromUDP(buffer)
	if err != nil {
		// Check if it's a timeout (no data available)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return 0, nil, nil // No data available (equivalent to C++ select() timeout)
		}
		log.Printf("UDP read error: %v", err)
		return -1, nil, err
	}

	return n, addr, nil
}

// Write sends data to specified address and port
// Equivalent to C++ CUDPSocket::write()
func (s *UDPSocket) Write(buffer []byte, addr *net.UDPAddr) error {
	if s.conn == nil {
		return fmt.Errorf("socket not open")
	}

	_, err := s.conn.WriteToUDP(buffer, addr)
	if err != nil {
		log.Printf("UDP write error: %v", err)
		return err
	}

	return nil
}

// Close closes the UDP socket
// Equivalent to C++ CUDPSocket::close()
func (s *UDPSocket) Close() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
		log.Printf("UDP socket closed")
	}
}

// Lookup resolves hostname to IP address
// Equivalent to C++ CUDPSocket::lookup()
func Lookup(hostname string) (net.IP, error) {
	// Try to parse as IP address first
	if ip := net.ParseIP(hostname); ip != nil {
		return ip, nil
	}

	// Resolve hostname using DNS
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	// Return first IPv4 address found
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no IPv4 address found for %s", hostname)
}

// ParseUDPAddr convenience function to parse address:port strings
func ParseUDPAddr(address string, port int) (*net.UDPAddr, error) {
	ip, err := Lookup(address)
	if err != nil {
		return nil, err
	}

	return &net.UDPAddr{
		IP:   ip,
		Port: port,
	}, nil
}