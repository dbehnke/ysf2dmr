package wiresx

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/correction"
)

// WiresX command patterns
var (
	DX_REQ   = []byte{0x5D, 0x71, 0x5F}
	CONN_REQ = []byte{0x5D, 0x23, 0x5F}
	DISC_REQ = []byte{0x5D, 0x2A, 0x5F}
	ALL_REQ  = []byte{0x5D, 0x66, 0x5F}
	CAT_REQ  = []byte{0x5D, 0x67, 0x5F}

	DX_RESP   = []byte{0x5D, 0x51, 0x5F, 0x26}
	CONN_RESP = []byte{0x5D, 0x41, 0x5F, 0x26}
	DISC_RESP = []byte{0x5D, 0x41, 0x5F, 0x26}
	ALL_RESP  = []byte{0x5D, 0x46, 0x5F, 0x26}

	DEFAULT_FICH = []byte{0x20, 0x00, 0x01, 0x00}
	NET_HEADER   = []byte("YSFD                    ALL      ")
)

// Status represents WiresX processing status
type Status int

const (
	StatusNone Status = iota
	StatusConnect
	StatusDisconnect
	StatusDX
	StatusAll
	StatusFail
)

// InternalStatus represents internal WiresX state
type InternalStatus int

const (
	InternalStatusNone InternalStatus = iota
	InternalStatusDX
	InternalStatusConnect
	InternalStatusDisconnect
	InternalStatusAll
	InternalStatusSearch
	InternalStatusCategory
)

// TalkGroup represents a talk group/reflector entry
type TalkGroup struct {
	ID   string // 7-digit ID with leading zeros
	Opt  string // Options
	Name string // Name (16 chars, space-padded)
	Desc string // Description (14 chars, space-padded)
}

// TalkGroupRegistry manages talk group lists
type TalkGroupRegistry struct {
	talkGroups []TalkGroup
	makeUpper  bool
}

// NewTalkGroupRegistry creates a new talk group registry
func NewTalkGroupRegistry(makeUpper bool) *TalkGroupRegistry {
	return &TalkGroupRegistry{
		talkGroups: make([]TalkGroup, 0),
		makeUpper:  makeUpper,
	}
}

// LoadFromString loads talk groups from string data (used for testing)
func (r *TalkGroupRegistry) LoadFromString(data string) error {
	scanner := bufio.NewScanner(strings.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		parts := strings.Split(line, ";")
		if len(parts) < 4 {
			continue
		}

		id := strings.TrimSpace(parts[0])
		opt := strings.TrimSpace(parts[1])
		name := strings.TrimSpace(parts[2])
		desc := strings.TrimSpace(parts[3])

		// Pad ID to 7 digits with leading zeros
		if len(id) < 7 {
			id = strings.Repeat("0", 7-len(id)) + id
		}

		// Process case conversion if requested
		if r.makeUpper {
			name = strings.ToUpper(name)
			desc = strings.ToUpper(desc)
		}

		// Pad name to 16 chars and desc to 14 chars
		if len(name) > 16 {
			name = name[:16]
		} else {
			name = name + strings.Repeat(" ", 16-len(name))
		}

		if len(desc) > 14 {
			desc = desc[:14]
		} else {
			desc = desc + strings.Repeat(" ", 14-len(desc))
		}

		tg := TalkGroup{
			ID:   id,
			Opt:  opt,
			Name: name,
			Desc: desc,
		}

		r.talkGroups = append(r.talkGroups, tg)
	}

	return scanner.Err()
}

// FindByID finds a talk group by numeric ID
func (r *TalkGroupRegistry) FindByID(id uint32) *TalkGroup {
	idStr := fmt.Sprintf("%07d", id)

	for i := range r.talkGroups {
		if r.talkGroups[i].ID == idStr {
			return &r.talkGroups[i]
		}
	}

	return nil
}

// Search searches for talk groups by name
func (r *TalkGroupRegistry) Search(searchTerm string) []TalkGroup {
	searchTerm = strings.ToUpper(strings.TrimSpace(searchTerm))
	if len(searchTerm) == 0 {
		return nil
	}

	var results []TalkGroup

	for _, tg := range r.talkGroups {
		name := strings.ToUpper(strings.TrimSpace(tg.Name))
		if strings.HasPrefix(name, searchTerm) {
			results = append(results, tg)
		}
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.TrimSpace(results[i].Name) < strings.TrimSpace(results[j].Name)
	})

	return results
}

// GetAll returns all talk groups with pagination
func (r *TalkGroupRegistry) GetAll(start, count int) []TalkGroup {
	if start >= len(r.talkGroups) {
		return nil
	}

	end := start + count
	if end > len(r.talkGroups) {
		end = len(r.talkGroups)
	}

	return r.talkGroups[start:end]
}

// GetCount returns total number of talk groups
func (r *TalkGroupRegistry) GetCount() int {
	return len(r.talkGroups)
}

// WiresX represents the WiresX protocol handler
type WiresX struct {
	callsign      string
	node          string
	id            string
	name          string
	txFrequency   uint32
	rxFrequency   uint32
	dstID         uint32
	fullDstID     uint32
	network       NetworkWriter
	command       []byte
	timer         *time.Timer
	timerDuration time.Duration
	seqNo         uint8
	header        []byte
	csd1          []byte
	csd2          []byte
	csd3          []byte
	status        InternalStatus
	start         int
	search        string
	category      []TalkGroup
	registry      *TalkGroupRegistry
	bufferTX      [][]byte
	lastTX        time.Time
}

// NetworkWriter interface for writing network data
type NetworkWriter interface {
	Write(data []byte) error
}

// NewWiresX creates a new WiresX handler
func NewWiresX(callsign, suffix string, network NetworkWriter, tgFile string, makeUpper bool) *WiresX {
	wx := &WiresX{
		callsign:      callsign,
		network:       network,
		command:       make([]byte, 300),
		timerDuration: time.Second,
		header:        make([]byte, 34),
		csd1:          make([]byte, 20),
		csd2:          make([]byte, 20),
		csd3:          make([]byte, 20),
		status:        InternalStatusNone,
		registry:      NewTalkGroupRegistry(makeUpper),
		bufferTX:      make([][]byte, 0),
		lastTX:        time.Now(),
	}

	// Build node name from callsign and suffix
	wx.node = callsign
	if len(suffix) > 0 {
		wx.node += "-" + suffix
	}

	// Pad to 10 characters
	if len(wx.node) > 10 {
		wx.node = wx.node[:10]
	} else {
		wx.node = wx.node + strings.Repeat(" ", 10-len(wx.node))
	}

	// Pad callsign to 10 characters
	if len(wx.callsign) > 10 {
		wx.callsign = wx.callsign[:10]
	} else {
		wx.callsign = wx.callsign + strings.Repeat(" ", 10-len(wx.callsign))
	}

	return wx
}

// SetInfo sets the repeater information
func (wx *WiresX) SetInfo(name string, txFrequency, rxFrequency uint32, dstID uint32) {
	wx.name = name
	wx.txFrequency = txFrequency
	wx.rxFrequency = rxFrequency
	wx.dstID = dstID

	// Truncate/pad name to 14 characters
	if len(name) > 14 {
		wx.name = name[:14]
	} else {
		wx.name = name + strings.Repeat(" ", 14-len(name))
	}

	// Generate repeater ID using hash
	hasher := fnv.New32a()
	hasher.Write([]byte(name))
	hash := hasher.Sum32()
	wx.id = fmt.Sprintf("%05d", hash%100000)

	// Initialize CSD fields
	for i := range wx.csd1 {
		wx.csd1[i] = '*'
	}
	for i := range wx.csd2 {
		wx.csd2[i] = ' '
	}
	for i := range wx.csd3 {
		wx.csd3[i] = ' '
	}

	// Set node in CSD1
	copy(wx.csd1[10:], wx.node[:10])

	// Set callsign in CSD2
	copy(wx.csd2[0:], wx.callsign[:10])

	// Set ID in CSD3
	copy(wx.csd3[0:], wx.id[:5])
	copy(wx.csd3[15:], wx.id[:5])

	// Initialize header
	copy(wx.header, NET_HEADER)
	copy(wx.header[4:], wx.callsign[:10])
	copy(wx.header[14:], wx.node[:10])
}

// Process processes a WiresX command
func (wx *WiresX) Process(data []byte, source []byte, fi, dt, fn, ft uint8) Status {
	// Only process data FR mode communications frames
	if dt != 1 || fi != 1 { // YSF_DT_DATA_FR_MODE, YSF_FI_COMMUNICATIONS
		return StatusNone
	}

	if fn == 0 {
		return StatusNone
	}

	// Extract command data (simplified - real implementation would use YSFPayload)
	if fn == 1 {
		// First frame contains up to 20 bytes
		copyLen := 20
		if len(data) < copyLen {
			copyLen = len(data)
		}
		copy(wx.command[0:copyLen], data[:copyLen])
	} else {
		// Subsequent frames contain up to 40 bytes each
		offset := int(fn-2)*40 + 20
		copyLen := 40
		if len(data) < copyLen {
			copyLen = len(data)
		}
		if offset+copyLen <= len(wx.command) {
			copy(wx.command[offset:offset+copyLen], data[:copyLen])
		}
	}

	// Check if this is the final frame
	if fn == ft {
		// Find the end marker (0x03)
		cmdLen := int(fn-1)*40 + 20
		valid := false

		for i := cmdLen; i > 0; i-- {
			if i < len(wx.command) && wx.command[i] == 0x03 {
				// Verify CRC (simplified - just check if CRC byte exists)
				if i+1 < len(wx.command) {
					// For now, accept any CRC value - real implementation would verify
					valid = true
				}
				break
			}
		}

		if !valid {
			return StatusNone
		}

		// Process different command types
		if len(wx.command) >= 4 {
			cmd := wx.command[1:4]

			if bytesEqual(cmd, DX_REQ) {
				wx.processDX(source)
				return StatusDX
			} else if bytesEqual(cmd, ALL_REQ) {
				wx.processAll(source, wx.command[5:])
				return StatusAll
			} else if bytesEqual(cmd, CONN_REQ) {
				return wx.processConnect(source, wx.command[4:])
			} else if bytesEqual(cmd, DISC_REQ) {
				wx.processDisconnect(source)
				return StatusDisconnect
			} else if bytesEqual(cmd, CAT_REQ) {
				wx.processCategory(source, wx.command[5:])
				return StatusNone
			}
		}

		return StatusFail
	}

	return StatusNone
}

// GetDstID returns the current destination ID
func (wx *WiresX) GetDstID() uint32 {
	return wx.dstID
}

// GetOpt returns the option value for a given ID
func (wx *WiresX) GetOpt(id uint32) uint32 {
	tg := wx.registry.FindByID(id)
	if tg != nil {
		opt, _ := strconv.ParseUint(tg.Opt, 10, 32)
		idFull, _ := strconv.ParseUint(tg.ID, 10, 32)
		wx.fullDstID = uint32(idFull)
		return uint32(opt)
	}

	wx.fullDstID = id
	return 0
}

// GetFullDstID returns the full destination ID
func (wx *WiresX) GetFullDstID() uint32 {
	return wx.fullDstID
}

// GetRepeaterID returns the repeater ID
func (wx *WiresX) GetRepeaterID() string {
	return wx.id
}

// ProcessConnect handles external connect requests
func (wx *WiresX) ProcessConnect(reflector uint32) {
	wx.dstID = reflector
	wx.status = InternalStatusConnect
	wx.startTimer()
}

// ProcessDisconnect handles external disconnect requests
func (wx *WiresX) ProcessDisconnect() {
	wx.status = InternalStatusDisconnect
	wx.startTimer()
}

// Clock updates the WiresX timer and processes pending responses
func (wx *WiresX) Clock(ms uint32) {
	// Check timer expiration
	if wx.timer != nil {
		select {
		case <-wx.timer.C:
			wx.handleTimerExpiry()
		default:
		}
	}

	// Handle TX buffer with rate limiting
	if time.Since(wx.lastTX) > 90*time.Millisecond && len(wx.bufferTX) > 0 {
		frame := wx.bufferTX[0]
		wx.bufferTX = wx.bufferTX[1:]

		if wx.network != nil {
			wx.network.Write(frame)
		}

		wx.lastTX = time.Now()
	}
}

// Private methods

func (wx *WiresX) processDX(source []byte) {
	wx.status = InternalStatusDX
	wx.startTimer()
}

func (wx *WiresX) processAll(source []byte, data []byte) {
	if len(data) < 5 {
		return
	}

	if data[0] == '0' && data[1] == '1' {
		// ALL request
		startStr := string(data[2:5])
		start, _ := strconv.Atoi(startStr)
		if start > 0 {
			start--
		}
		wx.start = start
		wx.status = InternalStatusAll
		wx.startTimer()
	} else if data[0] == '1' && data[1] == '1' {
		// SEARCH request
		startStr := string(data[2:5])
		start, _ := strconv.Atoi(startStr)
		if start > 0 {
			start--
		}
		wx.start = start

		if len(data) >= 21 {
			wx.search = string(data[5:21])
		}

		wx.status = InternalStatusSearch
		wx.startTimer()
	}
}

func (wx *WiresX) processConnect(source []byte, data []byte) Status {
	if len(data) < 6 {
		return StatusNone
	}

	idStr := string(data[:6])
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		return StatusNone
	}

	wx.dstID = uint32(id)
	wx.status = InternalStatusConnect
	wx.startTimer()

	return StatusConnect
}

func (wx *WiresX) processDisconnect(source []byte) {
	wx.status = InternalStatusDisconnect
	wx.startTimer()
}

func (wx *WiresX) processCategory(source []byte, data []byte) {
	// Category processing (simplified)
	wx.status = InternalStatusCategory
	wx.startTimer()
}

func (wx *WiresX) startTimer() {
	if wx.timer != nil {
		wx.timer.Stop()
	}
	wx.timer = time.NewTimer(wx.timerDuration)
}

func (wx *WiresX) handleTimerExpiry() {
	switch wx.status {
	case InternalStatusDX:
		wx.sendDXReply()
	case InternalStatusAll:
		wx.sendAllReply()
	case InternalStatusSearch:
		wx.sendSearchReply()
	case InternalStatusConnect:
		// Connect response is handled externally
	case InternalStatusDisconnect:
		// Disconnect response is handled externally
	case InternalStatusCategory:
		wx.sendCategoryReply()
	}

	wx.status = InternalStatusNone
	wx.timer = nil
}

func (wx *WiresX) sendDXReply() {
	data := wx.createDXResponse()
	wx.createReply(data)
	wx.seqNo++
}

func (wx *WiresX) sendAllReply() {
	data := wx.createAllResponse()
	wx.createReply(data)
	wx.seqNo++
}

func (wx *WiresX) sendSearchReply() {
	if len(wx.search) == 0 {
		wx.sendSearchNotFoundReply()
		return
	}

	results := wx.registry.Search(wx.search)
	if len(results) == 0 {
		wx.sendSearchNotFoundReply()
		return
	}

	data := wx.createSearchResponse(results)
	wx.createReply(data)
	wx.seqNo++
}

func (wx *WiresX) sendSearchNotFoundReply() {
	data := wx.createSearchNotFoundResponse()
	wx.createReply(data)
	wx.seqNo++
}

func (wx *WiresX) sendCategoryReply() {
	data := wx.createCategoryResponse()
	wx.createReply(data)
	wx.seqNo++
}

// SendConnectReply sends a connect response
func (wx *WiresX) SendConnectReply(dstID uint32) {
	wx.dstID = dstID
	data := wx.createConnectResponse(dstID)
	wx.createReply(data)
	wx.seqNo++
}

// SendDisconnectReply sends a disconnect response
func (wx *WiresX) SendDisconnectReply() {
	data := wx.createDisconnectResponse()
	wx.createReply(data)
	wx.seqNo++
}

func (wx *WiresX) createReply(data []byte) {
	// Simplified reply creation - real implementation would properly encode YSF frames
	// For now, just add to TX buffer
	frame := make([]byte, len(data))
	copy(frame, data)
	wx.bufferTX = append(wx.bufferTX, frame)
}

// Response creation methods
func (wx *WiresX) createDXResponse() []byte {
	data := make([]byte, 129)

	// Initialize with spaces
	for i := 0; i < 128; i++ {
		data[i] = ' '
	}

	data[0] = wx.seqNo

	// Response type
	copy(data[1:], DX_RESP)

	// Repeater ID
	copy(data[5:], wx.id[:5])

	// Node
	copy(data[10:], wx.node[:10])

	// Name
	copy(data[20:], wx.name[:14])

	if wx.dstID == 0 {
		data[34] = '1'
		data[35] = '2'
		copy(data[57:], "000")
	} else {
		data[34] = '1'
		data[35] = '5'

		dstIDStr := fmt.Sprintf("%05d", wx.dstID)
		copy(data[36:], dstIDStr)

		var name string
		if wx.dstID == 9 {
			name = "LOCAL"
		} else if wx.dstID == 9990 {
			name = "PARROT"
		} else if wx.dstID == 4000 {
			name = "UNLINK"
		} else {
			name = fmt.Sprintf("TG %d", wx.dstID)
		}

		if len(name) < 16 {
			name = name + strings.Repeat(" ", 16-len(name))
		}

		copy(data[41:], name[:16])
		copy(data[57:], "000")
		copy(data[70:], "Descripcion   ")
	}

	// Frequency information
	var offset uint32
	var sign byte
	if wx.txFrequency >= wx.rxFrequency {
		offset = wx.txFrequency - wx.rxFrequency
		sign = '-'
	} else {
		offset = wx.rxFrequency - wx.txFrequency
		sign = '+'
	}

	freqHz := wx.txFrequency % 1000000
	freqkHz := (freqHz + 500) / 1000

	freq := fmt.Sprintf("%05d.%03d000%c%03d.%06d",
		wx.txFrequency/1000000, freqkHz, sign,
		offset/1000000, offset%1000000)

	copy(data[84:], freq[:23])

	data[127] = 0x03 // End marker
	data[128] = correction.AddCRC(data[:128])

	return data
}

func (wx *WiresX) createConnectResponse(dstID uint32) []byte {
	data := make([]byte, 91)

	// Initialize with spaces
	for i := 0; i < 90; i++ {
		data[i] = ' '
	}

	data[0] = wx.seqNo
	copy(data[1:], CONN_RESP)
	copy(data[5:], wx.id[:5])
	copy(data[10:], wx.node[:10])
	copy(data[20:], wx.name[:14])

	data[34] = '1'
	data[35] = '5'

	dstIDStr := fmt.Sprintf("%05d", dstID)
	copy(data[36:], dstIDStr)

	var name string
	if dstID == 9 {
		name = "LOCAL"
	} else if dstID == 9990 {
		name = "PARROT"
	} else if dstID == 4000 {
		name = "UNLINK"
	} else {
		name = fmt.Sprintf("TG %d", dstID)
	}

	if len(name) < 16 {
		name = name + strings.Repeat(" ", 16-len(name))
	}

	copy(data[41:], name[:16])
	copy(data[57:], "000")
	copy(data[70:], "Descripcion   ")
	copy(data[84:], "00000")

	data[89] = 0x03 // End marker
	data[90] = correction.AddCRC(data[:90])

	return data
}

func (wx *WiresX) createDisconnectResponse() []byte {
	data := make([]byte, 91)

	// Initialize with spaces
	for i := 0; i < 90; i++ {
		data[i] = ' '
	}

	data[0] = wx.seqNo
	copy(data[1:], DISC_RESP)
	copy(data[5:], wx.id[:5])
	copy(data[10:], wx.node[:10])
	copy(data[20:], wx.name[:14])

	data[34] = '1'
	data[35] = '2'
	copy(data[57:], "000")

	data[89] = 0x03 // End marker
	data[90] = correction.AddCRC(data[:90])

	return data
}

func (wx *WiresX) createAllResponse() []byte {
	total := wx.registry.GetCount()
	if total > 999 {
		total = 999
	}

	n := total - wx.start
	if n > 20 {
		n = 20
	}

	talkGroups := wx.registry.GetAll(wx.start, n)

	// Calculate response size
	size := 29 + n*50 + (1029-29-n*50) + 2
	data := make([]byte, size)

	data[0] = wx.seqNo
	copy(data[1:], ALL_RESP)
	data[5] = '2'
	data[6] = '1'
	copy(data[7:], wx.id[:5])
	copy(data[12:], wx.node[:10])

	countStr := fmt.Sprintf("%03d%03d", n, total)
	copy(data[22:], countStr)
	data[28] = 0x0D

	offset := 29
	for _, tg := range talkGroups {
		// Initialize with spaces
		for j := 0; j < 50; j++ {
			data[offset+j] = ' '
		}

		data[offset] = '5'
		copy(data[offset+1:], tg.ID[2:7]) // Use last 5 digits
		copy(data[offset+6:], tg.Name)
		copy(data[offset+22:], "000")
		copy(data[offset+35:], tg.Desc)
		data[offset+49] = 0x0D

		offset += 50
	}

	// Pad to 1029
	for i := offset; i < 1029; i++ {
		data[i] = 0x20
	}
	offset = 1029

	data[offset] = 0x03 // End marker
	data[offset+1] = correction.AddCRC(data[:offset+1])

	return data[:offset+2]
}

func (wx *WiresX) createSearchResponse(results []TalkGroup) []byte {
	total := len(results)
	if total > 999 {
		total = 999
	}

	n := len(results) - wx.start
	if n > 20 {
		n = 20
	}

	if wx.start < len(results) {
		results = results[wx.start:]
	} else {
		results = nil
		n = 0
	}

	if n > len(results) {
		n = len(results)
	}

	// Calculate response size
	size := 29 + n*50 + (1029-29-n*50) + 2
	data := make([]byte, size)

	data[0] = wx.seqNo
	copy(data[1:], ALL_RESP)
	data[5] = '0'
	data[6] = '2'
	copy(data[7:], wx.id[:5])
	copy(data[12:], wx.node[:10])
	data[22] = '1'

	countStr := fmt.Sprintf("%02d%03d", n, total)
	copy(data[23:], countStr)
	data[28] = 0x0D

	offset := 29
	for i := 0; i < n; i++ {
		tg := results[i]

		// Initialize with spaces
		for j := 0; j < 50; j++ {
			data[offset+j] = ' '
		}

		data[offset] = '1'
		copy(data[offset+1:], tg.ID[2:7]) // Use last 5 digits
		copy(data[offset+6:], strings.ToUpper(tg.Name))
		copy(data[offset+22:], "000")
		copy(data[offset+35:], tg.Desc)
		data[offset+49] = 0x0D

		offset += 50
	}

	// Pad to 1029
	for i := offset; i < 1029; i++ {
		data[i] = 0x20
	}
	offset = 1029

	data[offset] = 0x03 // End marker
	data[offset+1] = correction.AddCRC(data[:offset+1])

	return data[:offset+2]
}

func (wx *WiresX) createSearchNotFoundResponse() []byte {
	data := make([]byte, 31)

	data[0] = wx.seqNo
	copy(data[1:], ALL_RESP)
	data[5] = '0'
	data[6] = '1'
	copy(data[7:], wx.id[:5])
	copy(data[12:], wx.node[:10])
	data[22] = '1'
	copy(data[23:], "00000")
	data[28] = 0x0D
	data[29] = 0x03 // End marker
	data[30] = correction.AddCRC(data[:30])

	return data
}

func (wx *WiresX) createCategoryResponse() []byte {
	// Simplified category response
	return wx.createAllResponse()
}

// Utility function
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}