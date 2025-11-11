package network

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/dbehnke/ysf2dmr/internal/protocol"
)

// processPacket handles incoming DMR network packets
func (n *DMRNetwork) processPacket(packet []byte) {
	if len(packet) < 4 {
		return // Invalid packet
	}

	magic := string(packet[:4])

	switch magic {
	case protocol.NETWORK_MAGIC_ACK:
		n.handleRPTACK(packet)
	case protocol.NETWORK_MAGIC_NAK:
		n.handleMSTNAK(packet)
	case protocol.NETWORK_MAGIC_PONG:
		n.handleMSTPONG(packet)
	case protocol.NETWORK_MAGIC_CLOSE_MASTER:
		n.handleMSTCL(packet)
	case protocol.NETWORK_MAGIC_BEACON:
		n.handleBeacon(packet)
	case protocol.NETWORK_MAGIC_DATA:
		n.handleDMRD(packet)
	default:
		if n.debug {
			log.Printf("DMR: Unknown packet type: %s (%d bytes)", magic, len(packet))
		}
	}
}

// handleRPTACK processes RPTACK acknowledgement packets
func (n *DMRNetwork) handleRPTACK(packet []byte) {
	if n.debug {
		log.Printf("DMR: Received RPTACK in state %d", n.status)
	}

	switch n.status {
	case protocol.DMR_WAITING_LOGIN:
		// Extract salt from packet
		if len(packet) >= 10 {
			copy(n.salt, packet[6:10])
			if n.debug {
				log.Printf("DMR: Received salt: %02X %02X %02X %02X",
					n.salt[0], n.salt[1], n.salt[2], n.salt[3])
			}
		}
		// Send authorization
		n.writeAuth()
		n.status = protocol.DMR_WAITING_AUTHORISATION

	case protocol.DMR_WAITING_AUTHORISATION:
		// Send configuration
		n.writeConfig()
		n.status = protocol.DMR_WAITING_CONFIG

	case protocol.DMR_WAITING_CONFIG:
		if len(n.options) > 0 {
			// Send options
			n.writeOptions()
			n.status = protocol.DMR_WAITING_OPTIONS
		} else {
			// Connected
			n.status = protocol.DMR_RUNNING
			n.timeoutTimer.Start(protocol.DMR_CONNECTION_TIMEOUT/1000, protocol.DMR_CONNECTION_TIMEOUT%1000)
			if n.debug {
				log.Printf("DMR: Connected and running")
			}
		}

	case protocol.DMR_WAITING_OPTIONS:
		// Connected
		n.status = protocol.DMR_RUNNING
		n.timeoutTimer.Start(protocol.DMR_CONNECTION_TIMEOUT/1000, protocol.DMR_CONNECTION_TIMEOUT%1000)
		if n.debug {
			log.Printf("DMR: Connected and running")
		}

	default:
		// Ignore RPTACK in other states
	}
}

// handleMSTNAK processes MSTNAK negative acknowledgement packets
func (n *DMRNetwork) handleMSTNAK(packet []byte) {
	if n.debug {
		log.Printf("DMR: Received MSTNAK - authentication failed")
	}

	// Reset to login state
	n.status = protocol.DMR_WAITING_LOGIN
	n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
}

// handleMSTPONG processes MSTPONG ping response packets
func (n *DMRNetwork) handleMSTPONG(packet []byte) {
	if n.debug {
		log.Printf("DMR: Received MSTPONG")
	}

	// Restart timeout timer
	n.timeoutTimer.Start(protocol.DMR_CONNECTION_TIMEOUT/1000, protocol.DMR_CONNECTION_TIMEOUT%1000)
}

// handleMSTCL processes master close packets
func (n *DMRNetwork) handleMSTCL(packet []byte) {
	if n.debug {
		log.Printf("DMR: Received MSTCL - master closing")
	}

	// Server is shutting down
	n.status = protocol.DMR_WAITING_CONNECT
	n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
}

// handleBeacon processes beacon request packets
func (n *DMRNetwork) handleBeacon(packet []byte) {
	if n.debug {
		log.Printf("DMR: Received beacon request")
	}

	n.beacon = true
}

// handleDMRD processes DMRD data packets
func (n *DMRNetwork) handleDMRD(packet []byte) {
	if !n.enabled || len(packet) != protocol.HOMEBREW_DATA_PACKET_LENGTH {
		return
	}

	// Extract slot number from packet
	slotNo := uint8(1)
	if len(packet) > 15 && (packet[15]&0x80) != 0 {
		slotNo = 2
	}

	// Check if slot is enabled
	if (slotNo == 1 && !n.slot1) || (slotNo == 2 && !n.slot2) {
		return
	}

	// Add to delay buffer
	if n.delayBuffers[slotNo] != nil {
		seqNo := packet[4] // Sequence number
		n.delayBuffers[slotNo].AddData(packet, seqNo)
	}
}

// handleRetryTimeout handles retry timer expiration
func (n *DMRNetwork) handleRetryTimeout() {
	switch n.status {
	case protocol.DMR_WAITING_CONNECT:
		// C++ behavior: open socket first, then login if successful
		err := n.socket.Open()
		if err != nil {
			if n.debug {
				log.Printf("DMR: Socket open failed: %v", err)
			}
			// Don't change state, will retry on next timer expiration
			return
		}

		if n.debug {
			log.Printf("DMR: Socket opened, sending login packet")
		}

		n.writeLogin()
		n.status = protocol.DMR_WAITING_LOGIN
		n.timeoutTimer.Start(protocol.DMR_CONNECTION_TIMEOUT/1000, protocol.DMR_CONNECTION_TIMEOUT%1000)

	case protocol.DMR_WAITING_LOGIN:
		n.writeLogin()

	case protocol.DMR_WAITING_AUTHORISATION:
		n.writeAuth()

	case protocol.DMR_WAITING_CONFIG:
		n.writeConfig()

	case protocol.DMR_WAITING_OPTIONS:
		n.writeOptions()

	case protocol.DMR_RUNNING:
		n.writePing()

	default:
		// Unknown state
		n.status = protocol.DMR_WAITING_CONNECT
	}

	// Restart retry timer
	n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
}

// handleConnectionTimeout handles connection timeout
func (n *DMRNetwork) handleConnectionTimeout() {
	if n.debug {
		log.Printf("DMR: Connection timeout")
	}

	// Connection lost, reconnect
	n.status = protocol.DMR_WAITING_CONNECT
	n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
}

// writeLogin sends login packet (RPTL)
func (n *DMRNetwork) writeLogin() {
	packet := make([]byte, protocol.NETWORK_LOGIN_LENGTH)
	copy(packet[0:4], protocol.NETWORK_MAGIC_LOGIN)
	copy(packet[4:8], n.id[:])

	n.writePacket(packet)

	if n.debug {
		log.Printf("DMR: Sent login packet to %s:%d", n.address.String(), n.port)
		log.Printf("DMR: Login packet (hex): %X", packet)
	}
}

// writeAuth sends authorization packet (RPTK)
func (n *DMRNetwork) writeAuth() {
	// Calculate SHA256(salt + password)
	hasher := sha256.New()
	hasher.Write(n.salt)
	hasher.Write([]byte(n.password))
	hash := hasher.Sum(nil)

	packet := make([]byte, protocol.NETWORK_AUTH_LENGTH)
	copy(packet[0:4], protocol.NETWORK_MAGIC_AUTH)
	copy(packet[4:8], n.id[:])
	copy(packet[8:40], hash[:32])

	n.writePacket(packet)

	if n.debug {
		log.Printf("DMR: Sent auth packet")
	}
}

// writeConfig sends configuration packet (RPTC)
func (n *DMRNetwork) writeConfig() {
	packet := make([]byte, protocol.NETWORK_CONFIG_LENGTH)

	// Magic and ID
	copy(packet[0:4], protocol.NETWORK_MAGIC_CONFIG)
	copy(packet[4:8], n.id[:])

	// Callsign (8 bytes, left-aligned with right padding, matching C++ %-8.8s)
	callsign := strings.ToUpper(n.callsign)
	if len(callsign) > 8 {
		callsign = callsign[:8]
	}
	callsignBytes := make([]byte, 8)
	// Left-align: copy callsign to start of buffer, pad with spaces on right
	copy(callsignBytes, callsign)
	for i := len(callsign); i < 8; i++ {
		callsignBytes[i] = ' '
	}
	copy(packet[8:16], callsignBytes)

	// Frequencies (9 bytes each, zero-padded)
	rxFreqStr := fmt.Sprintf("%09d", n.rxFrequency)
	txFreqStr := fmt.Sprintf("%09d", n.txFrequency)
	copy(packet[16:25], rxFreqStr)
	copy(packet[25:34], txFreqStr)

	// Power (2 bytes, zero-padded)
	powerStr := fmt.Sprintf("%02d", n.power)
	copy(packet[34:36], powerStr)

	// Color Code (2 bytes, zero-padded)
	ccStr := fmt.Sprintf("%02d", n.colorCode)
	copy(packet[36:38], ccStr)

	// Latitude (8 bytes) - match C++ %08f then truncate to 8 chars
	latStr := fmt.Sprintf("%08f", n.latitude)
	if len(latStr) > 8 {
		latStr = latStr[:8]
	}
	copy(packet[38:46], latStr)

	// Longitude (9 bytes) - match C++ %09f then truncate to 9 chars
	lngStr := fmt.Sprintf("%09f", n.longitude)
	if len(lngStr) > 9 {
		lngStr = lngStr[:9]
	}
	copy(packet[46:55], lngStr)

	// Height (3 bytes)
	heightStr := fmt.Sprintf("%03d", n.height)
	copy(packet[55:58], heightStr)

	// Location (20 bytes)
	location := n.location
	if len(location) > 20 {
		location = location[:20]
	}
	copy(packet[58:78], location)

	// Description (19 bytes)
	description := n.description
	if len(description) > 19 {
		description = description[:19]
	}
	copy(packet[78:97], description)

	// Slots configuration
	slotConfig := byte('0')
	if n.slot1 && n.slot2 {
		slotConfig = '3' // Both slots
	} else if n.slot1 {
		slotConfig = '1' // Slot 1 only
	} else if n.slot2 {
		slotConfig = '2' // Slot 2 only
	}
	packet[97] = slotConfig

	// URL (124 bytes)
	url := n.url
	if len(url) > 124 {
		url = url[:124]
	}
	copy(packet[98:222], url)

	// Version (40 bytes)
	version := n.version
	if len(version) > 40 {
		version = version[:40]
	}
	copy(packet[222:262], version)

	// Software type (40 bytes)
	hwTypeStr := n.hwType.String()
	if len(hwTypeStr) > 40 {
		hwTypeStr = hwTypeStr[:40]
	}
	copy(packet[262:302], hwTypeStr)

	n.writePacket(packet)

	if n.debug {
		log.Printf("DMR: Sent config packet")
	}
}

// writeOptions sends options packet (RPTO)
func (n *DMRNetwork) writeOptions() {
	packet := make([]byte, 8+len(n.options)+1) // +1 for null terminator
	copy(packet[0:4], protocol.NETWORK_MAGIC_OPTIONS)
	copy(packet[4:8], n.id[:])
	copy(packet[8:], n.options)
	packet[len(packet)-1] = 0 // Null terminator

	n.writePacket(packet)

	if n.debug {
		log.Printf("DMR: Sent options packet")
	}
}

// writePing sends ping packet (RPTPING)
func (n *DMRNetwork) writePing() {
	packet := make([]byte, protocol.NETWORK_PING_LENGTH)
	copy(packet[0:7], protocol.NETWORK_MAGIC_PING)
	copy(packet[7:11], n.id[:])

	n.writePacket(packet)

	if n.debug {
		log.Printf("DMR: Sent ping packet")
	}
}

// writeClose sends close packet (RPTCL)
func (n *DMRNetwork) writeClose() {
	packet := make([]byte, protocol.NETWORK_CLOSE_LENGTH)
	copy(packet[0:5], protocol.NETWORK_MAGIC_CLOSE)
	copy(packet[5:9], n.id[:])

	n.writePacket(packet)

	if n.debug {
		log.Printf("DMR: Sent close packet")
	}
}

// writePacket sends a packet to the DMR server
func (n *DMRNetwork) writePacket(packet []byte) {
	addr := &net.UDPAddr{
		IP:   n.address,
		Port: n.port,
	}

	err := n.socket.Write(packet, addr)
	if err != nil {
		if n.debug {
			log.Printf("DMR: Write error: %v", err)
		}
		// Trigger reconnection
		n.status = protocol.DMR_WAITING_CONNECT
		n.retryTimer.Start(protocol.DMR_RETRY_TIMEOUT/1000, protocol.DMR_RETRY_TIMEOUT%1000)
	}
}

// buildDMRDPacket builds a DMRD data packet
func (n *DMRNetwork) buildDMRDPacket(data *protocol.DMRData) []byte {
	packet := make([]byte, protocol.HOMEBREW_DATA_PACKET_LENGTH)

	// Magic
	copy(packet[0:4], protocol.NETWORK_MAGIC_DATA)

	// Sequence number (increment per packet)
	packet[4] = n.seqNo
	n.seqNo++

	// Source ID (3 bytes, big-endian)
	srcId := data.GetSrcId()
	packet[5] = byte(srcId >> 16)
	packet[6] = byte(srcId >> 8)
	packet[7] = byte(srcId)

	// Destination ID (3 bytes, big-endian)
	dstId := data.GetDstId()
	packet[8] = byte(dstId >> 16)
	packet[9] = byte(dstId >> 8)
	packet[10] = byte(dstId)

	// Repeater ID
	copy(packet[11:15], n.id[:])

	// Build flags byte
	flags := byte(0)
	slotNo := data.GetSlotNo()
	if slotNo == 2 {
		flags |= 0x80 // Slot 2
	}
	if data.GetFLCO() == protocol.FLCO_USER_USER {
		flags |= 0x40 // Private call
	}
	if data.IsDataSync() {
		flags |= 0x20 // Data sync
	}
	if data.IsVoiceSync() {
		flags |= 0x10 // Voice sync
	}

	// Add data type or N value
	if data.IsVoice() {
		flags |= data.GetN() & 0x0F
	} else {
		flags |= data.GetDataType() & 0x0F
	}
	packet[15] = flags

	// Stream ID (use per-slot stream ID)
	streamId := n.streamId[slotNo]
	binary.BigEndian.PutUint32(packet[16:20], streamId)

	// DMR data (33 bytes)
	dmrData := data.GetData()
	copy(packet[20:53], dmrData[:])

	// BER and RSSI
	packet[53] = data.GetBER()
	packet[54] = data.GetRSSI()

	return packet
}

// parseDMRDPacket parses a DMRD packet into DMRData
func (n *DMRNetwork) parseDMRDPacket(packet []byte, data *protocol.DMRData) bool {
	if len(packet) != protocol.HOMEBREW_DATA_PACKET_LENGTH {
		return false
	}

	if !bytes.Equal(packet[0:4], []byte(protocol.NETWORK_MAGIC_DATA)) {
		return false
	}

	// Parse sequence number
	data.SetSeqNo(packet[4])

	// Parse source ID (24-bit big-endian)
	srcId := (uint32(packet[5]) << 16) | (uint32(packet[6]) << 8) | uint32(packet[7])
	data.SetSrcId(srcId)

	// Parse destination ID (24-bit big-endian)
	dstId := (uint32(packet[8]) << 16) | (uint32(packet[9]) << 8) | uint32(packet[10])
	data.SetDstId(dstId)

	// Parse flags
	flags := packet[15]

	// Extract slot number
	if (flags & 0x80) != 0 {
		data.SetSlotNo(2)
	} else {
		data.SetSlotNo(1)
	}

	// Extract FLCO
	if (flags & 0x40) != 0 {
		data.SetFLCO(protocol.FLCO_USER_USER) // Private call
	} else {
		data.SetFLCO(protocol.FLCO_GROUP) // Group call
	}

	// Extract data type and N value
	dataSync := (flags & 0x20) != 0
	voiceSync := (flags & 0x10) != 0
	nValue := flags & 0x0F

	if voiceSync {
		data.SetDataType(protocol.DT_VOICE_SYNC)
		data.SetN(nValue)
	} else if dataSync {
		// Determine specific data type based on N value
		switch nValue {
		case 1:
			data.SetDataType(protocol.DT_VOICE_LC_HEADER)
		case 2:
			data.SetDataType(protocol.DT_TERMINATOR_WITH_LC)
		case 6:
			data.SetDataType(protocol.DT_DATA_HEADER)
		case 7:
			data.SetDataType(protocol.DT_RATE_12_DATA)
		case 8:
			data.SetDataType(protocol.DT_RATE_34_DATA)
		case 10:
			data.SetDataType(protocol.DT_RATE_1_DATA)
		default:
			data.SetDataType(nValue)
		}
		data.SetN(0)
	} else {
		data.SetDataType(protocol.DT_VOICE)
		data.SetN(nValue)
	}

	// Parse stream ID
	streamId := binary.BigEndian.Uint32(packet[16:20])
	data.SetStreamId(streamId)

	// Parse DMR data
	var dmrData [33]byte
	copy(dmrData[:], packet[20:53])
	data.SetData(dmrData[:])

	// Parse BER and RSSI
	data.SetBER(packet[53])
	data.SetRSSI(packet[54])

	return true
}