package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config represents the YSF2DMR configuration
type Config struct {
	filename string

	// Info section
	rxFrequency uint32
	txFrequency uint32
	power       uint32
	latitude    float64
	longitude   float64
	height      int32
	location    string
	description string
	url         string

	// YSF Network section
	callsign        string
	suffix          string
	dstAddress      string
	dstPort         uint32
	localAddress    string
	localPort       uint32
	enableWiresX    bool
	remoteGateway   bool
	hangTime        uint32
	wiresXMakeUpper bool
	fichCallSign    uint8
	fichCallMode    uint8
	fichFrameTotal  uint8
	fichMessageRoute uint8
	fichVOIP        uint8
	fichDataType    uint8
	fichSQLType     uint8
	fichSQLCode     uint8
	ysfDT1          []uint8
	ysfDT2          []uint8
	ysfRadioID      string
	daemon          bool
	ysfDebug        bool

	// DMR Network section
	dmrId                   uint32
	dmrXLXFile             string
	dmrXLXModule           string
	dmrXLXReflector        uint32
	dmrDstId               uint32
	dmrPC                  bool
	dmrNetworkAddress      string
	dmrNetworkPort         uint32
	dmrNetworkLocal        uint32
	dmrNetworkPassword     string
	dmrNetworkOptions      string
	dmrNetworkDebug        bool
	dmrNetworkJitterEnabled bool
	dmrNetworkJitter       uint32
	dmrNetworkEnableUnlink bool
	dmrNetworkIDUnlink     uint32
	dmrNetworkPCUnlink     bool
	dmrTGListFile          string

	// DMR Id Lookup section
	dmrIdLookupFile string
	dmrIdLookupTime uint32
	dmrDropUnknown  bool

	// Database section (for modern database-backed DMR ID lookup)
	databaseEnabled    bool
	databasePath       string
	databaseSyncHours  uint32
	databaseCacheSize  uint32
	databaseDebug      bool

	// Log section
	logDisplayLevel uint32
	logFileLevel    uint32
	logFilePath     string
	logFileRoot     string

	// APRS section
	aprsEnabled     bool
	aprsServer      string
	aprsPort        uint32
	aprsPassword    string
	aprsCallsign    string
	aprsAPIKey      string
	aprsRefresh     uint32
	aprsDescription string
}

// NewConfig creates a new configuration instance
func NewConfig(filename string) *Config {
	return &Config{
		filename: filename,
		// Set reasonable defaults
		dstPort:         42000,
		localPort:       42013,
		hangTime:        1000,
		dmrNetworkPort:  62031,
		dmrNetworkJitter: 500,
		dmrIdLookupTime: 24,
		aprsPort:        14580,
		aprsRefresh:     240,

		// Database defaults
		databaseEnabled:   false, // Disabled by default for backward compatibility
		databasePath:      "data/dmr_users.db",
		databaseSyncHours: 24, // Sync every 24 hours
		databaseCacheSize: 1000,
		databaseDebug:     false,
	}
}

// Load loads configuration from the specified file
func (c *Config) Load() error {
	file, err := os.Open(c.filename)
	if err != nil {
		return fmt.Errorf("failed to open config file %s: %v", c.filename, err)
	}
	defer file.Close()

	return c.parseINI(file)
}

// LoadFromString loads configuration from a string (useful for testing)
func (c *Config) LoadFromString(data string) error {
	return c.parseINIString(data)
}

func (c *Config) parseINI(file *os.File) error {
	scanner := bufio.NewScanner(file)
	return c.parseINIScanner(scanner)
}

func (c *Config) parseINIString(data string) error {
	scanner := bufio.NewScanner(strings.NewReader(data))
	return c.parseINIScanner(scanner)
}

func (c *Config) parseINIScanner(scanner *bufio.Scanner) error {
	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Check for section header
		if line[0] == '[' && line[len(line)-1] == ']' {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Parse based on current section
		switch currentSection {
		case "Info":
			c.parseInfoSection(key, value)
		case "YSF Network":
			c.parseYSFNetworkSection(key, value)
		case "DMR Network":
			c.parseDMRNetworkSection(key, value)
		case "DMR Id Lookup":
			c.parseDMRIdLookupSection(key, value)
		case "Database":
			c.parseDatabaseSection(key, value)
		case "Log":
			c.parseLogSection(key, value)
		case "aprs.fi":
			c.parseAPRSSection(key, value)
		}
	}

	return scanner.Err()
}

func (c *Config) parseInfoSection(key, value string) {
	switch key {
	case "RXFrequency":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.rxFrequency = uint32(v)
		}
	case "TXFrequency":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.txFrequency = uint32(v)
		}
	case "Power":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.power = uint32(v)
		}
	case "Latitude":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			c.latitude = v
		}
	case "Longitude":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			c.longitude = v
		}
	case "Height":
		if v, err := strconv.ParseInt(value, 10, 32); err == nil {
			c.height = int32(v)
		}
	case "Location":
		c.location = value
	case "Description":
		c.description = value
	case "URL":
		c.url = value
	}
}

func (c *Config) parseYSFNetworkSection(key, value string) {
	switch key {
	case "Callsign":
		c.callsign = value
	case "Suffix":
		c.suffix = value
	case "DstAddress":
		c.dstAddress = value
	case "DstPort":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dstPort = uint32(v)
		}
	case "LocalAddress":
		c.localAddress = value
	case "LocalPort":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.localPort = uint32(v)
		}
	case "EnableWiresX":
		c.enableWiresX = c.parseBool(value)
	case "RemoteGateway":
		c.remoteGateway = c.parseBool(value)
	case "HangTime":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.hangTime = uint32(v)
		}
	case "WiresXMakeUpper":
		c.wiresXMakeUpper = c.parseBool(value)
	case "FICHCallsign":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichCallSign = uint8(v)
		}
	case "FICHCallMode":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichCallMode = uint8(v)
		}
	case "FICHFrameTotal":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichFrameTotal = uint8(v)
		}
	case "FICHMessageRoute":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichMessageRoute = uint8(v)
		}
	case "FICHVOIP":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichVOIP = uint8(v)
		}
	case "FICHDataType":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichDataType = uint8(v)
		}
	case "FICHSQLType":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichSQLType = uint8(v)
		}
	case "FICHSQLCode":
		if v, err := strconv.ParseUint(value, 10, 8); err == nil {
			c.fichSQLCode = uint8(v)
		}
	case "DT1":
		c.ysfDT1 = c.parseByteArray(value)
	case "DT2":
		c.ysfDT2 = c.parseByteArray(value)
	case "RadioID":
		c.ysfRadioID = value
	case "Daemon":
		c.daemon = c.parseBool(value)
	case "Debug":
		c.ysfDebug = c.parseBool(value)
	}
}

func (c *Config) parseDMRNetworkSection(key, value string) {
	switch key {
	case "Id":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrId = uint32(v)
		}
	case "XLXFile":
		c.dmrXLXFile = value
	case "XLXModule":
		c.dmrXLXModule = value
	case "XLXReflector":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrXLXReflector = uint32(v)
		}
	case "StartupDstId":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrDstId = uint32(v)
		}
	case "StartupPC":
		c.dmrPC = c.parseBool(value)
	case "Address":
		c.dmrNetworkAddress = value
	case "Port":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrNetworkPort = uint32(v)
		}
	case "Local":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrNetworkLocal = uint32(v)
		}
	case "Password":
		c.dmrNetworkPassword = value
	case "Options":
		c.dmrNetworkOptions = value
	case "Debug":
		c.dmrNetworkDebug = c.parseBool(value)
	case "Jitter":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrNetworkJitter = uint32(v)
		}
	case "EnableUnlink":
		c.dmrNetworkEnableUnlink = c.parseBool(value)
	case "TGUnlink":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrNetworkIDUnlink = uint32(v)
		}
	case "PCUnlink":
		c.dmrNetworkPCUnlink = c.parseBool(value)
	case "TGListFile":
		c.dmrTGListFile = value
	}
}

func (c *Config) parseDMRIdLookupSection(key, value string) {
	switch key {
	case "File":
		c.dmrIdLookupFile = value
	case "Time":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.dmrIdLookupTime = uint32(v)
		}
	case "DropUnknown":
		c.dmrDropUnknown = c.parseBool(value)
	}
}

func (c *Config) parseDatabaseSection(key, value string) {
	switch key {
	case "Enabled":
		c.databaseEnabled = c.parseBool(value)
	case "Path":
		c.databasePath = value
	case "SyncHours":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.databaseSyncHours = uint32(v)
		}
	case "CacheSize":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.databaseCacheSize = uint32(v)
		}
	case "Debug":
		c.databaseDebug = c.parseBool(value)
	}
}

func (c *Config) parseLogSection(key, value string) {
	switch key {
	case "DisplayLevel":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.logDisplayLevel = uint32(v)
		}
	case "FileLevel":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.logFileLevel = uint32(v)
		}
	case "FilePath":
		c.logFilePath = value
	case "FileRoot":
		c.logFileRoot = value
	}
}

func (c *Config) parseAPRSSection(key, value string) {
	switch key {
	case "Enable":
		c.aprsEnabled = c.parseBool(value)
	case "Server":
		c.aprsServer = value
	case "Port":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.aprsPort = uint32(v)
		}
	case "Password":
		c.aprsPassword = value
	case "AprsCallsign":
		c.aprsCallsign = value
	case "APIKey":
		c.aprsAPIKey = value
	case "Refresh":
		if v, err := strconv.ParseUint(value, 10, 32); err == nil {
			c.aprsRefresh = uint32(v)
		}
	case "Description":
		c.aprsDescription = value
	}
}

func (c *Config) parseBool(value string) bool {
	return value == "1" || strings.ToLower(value) == "true" || strings.ToLower(value) == "yes"
}

func (c *Config) parseByteArray(value string) []uint8 {
	parts := strings.Split(value, ",")
	result := make([]uint8, 0, len(parts))

	for _, part := range parts {
		if v, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8); err == nil {
			result = append(result, uint8(v))
		}
	}

	return result
}

// Getter methods for Info section
func (c *Config) GetRxFrequency() uint32  { return c.rxFrequency }
func (c *Config) GetTxFrequency() uint32  { return c.txFrequency }
func (c *Config) GetPower() uint32        { return c.power }
func (c *Config) GetLatitude() float64    { return c.latitude }
func (c *Config) GetLongitude() float64   { return c.longitude }
func (c *Config) GetHeight() int32        { return c.height }
func (c *Config) GetLocation() string     { return c.location }
func (c *Config) GetDescription() string  { return c.description }
func (c *Config) GetURL() string          { return c.url }

// Getter methods for YSF Network section
func (c *Config) GetCallsign() string        { return c.callsign }
func (c *Config) GetSuffix() string          { return c.suffix }
func (c *Config) GetDstAddress() string      { return c.dstAddress }
func (c *Config) GetDstPort() uint32         { return c.dstPort }
func (c *Config) GetLocalAddress() string    { return c.localAddress }
func (c *Config) GetLocalPort() uint32       { return c.localPort }
func (c *Config) GetEnableWiresX() bool      { return c.enableWiresX }
func (c *Config) GetRemoteGateway() bool     { return c.remoteGateway }
func (c *Config) GetHangTime() uint32        { return c.hangTime }
func (c *Config) GetWiresXMakeUpper() bool   { return c.wiresXMakeUpper }
func (c *Config) GetFICHCallSign() uint8     { return c.fichCallSign }
func (c *Config) GetFICHCallMode() uint8     { return c.fichCallMode }
func (c *Config) GetFICHFrameTotal() uint8   { return c.fichFrameTotal }
func (c *Config) GetFICHMessageRoute() uint8 { return c.fichMessageRoute }
func (c *Config) GetFICHVOIP() uint8         { return c.fichVOIP }
func (c *Config) GetFICHDataType() uint8     { return c.fichDataType }
func (c *Config) GetFICHSQLType() uint8      { return c.fichSQLType }
func (c *Config) GetFICHSQLCode() uint8      { return c.fichSQLCode }
func (c *Config) GetYsfDT1() []uint8         { return c.ysfDT1 }
func (c *Config) GetYsfDT2() []uint8         { return c.ysfDT2 }
func (c *Config) GetYsfRadioID() string      { return c.ysfRadioID }
func (c *Config) GetDaemon() bool            { return c.daemon }
func (c *Config) GetYSFDebug() bool          { return c.ysfDebug }

// Getter methods for DMR Network section
func (c *Config) GetDMRId() uint32                   { return c.dmrId }
func (c *Config) GetDMRXLXFile() string             { return c.dmrXLXFile }
func (c *Config) GetDMRXLXModule() string           { return c.dmrXLXModule }
func (c *Config) GetDMRXLXReflector() uint32        { return c.dmrXLXReflector }
func (c *Config) GetDMRDstId() uint32               { return c.dmrDstId }
func (c *Config) GetDMRPC() bool                    { return c.dmrPC }
func (c *Config) GetDMRNetworkAddress() string      { return c.dmrNetworkAddress }
func (c *Config) GetDMRNetworkPort() uint32         { return c.dmrNetworkPort }
func (c *Config) GetDMRNetworkLocal() uint32        { return c.dmrNetworkLocal }
func (c *Config) GetDMRNetworkPassword() string     { return c.dmrNetworkPassword }
func (c *Config) GetDMRNetworkOptions() string      { return c.dmrNetworkOptions }
func (c *Config) GetDMRNetworkDebug() bool          { return c.dmrNetworkDebug }
func (c *Config) GetDMRNetworkJitterEnabled() bool  { return c.dmrNetworkJitterEnabled }
func (c *Config) GetDMRNetworkJitter() uint32       { return c.dmrNetworkJitter }
func (c *Config) GetDMRNetworkEnableUnlink() bool   { return c.dmrNetworkEnableUnlink }
func (c *Config) GetDMRNetworkIDUnlink() uint32     { return c.dmrNetworkIDUnlink }
func (c *Config) GetDMRNetworkPCUnlink() bool       { return c.dmrNetworkPCUnlink }
func (c *Config) GetDMRTGListFile() string          { return c.dmrTGListFile }

// Getter methods for DMR Id Lookup section
func (c *Config) GetDMRIdLookupFile() string { return c.dmrIdLookupFile }
func (c *Config) GetDMRIdLookupTime() uint32 { return c.dmrIdLookupTime }
func (c *Config) GetDMRDropUnknown() bool    { return c.dmrDropUnknown }

// Getter methods for Log section
func (c *Config) GetLogDisplayLevel() uint32 { return c.logDisplayLevel }
func (c *Config) GetLogFileLevel() uint32    { return c.logFileLevel }
func (c *Config) GetLogFilePath() string     { return c.logFilePath }
func (c *Config) GetLogFileRoot() string     { return c.logFileRoot }

// Getter methods for APRS section
func (c *Config) GetAPRSEnabled() bool        { return c.aprsEnabled }
func (c *Config) GetAPRSServer() string       { return c.aprsServer }
func (c *Config) GetAPRSPort() uint32         { return c.aprsPort }
func (c *Config) GetAPRSPassword() string     { return c.aprsPassword }
func (c *Config) GetAPRSCallsign() string     { return c.aprsCallsign }
func (c *Config) GetAPRSAPIKey() string       { return c.aprsAPIKey }
func (c *Config) GetAPRSRefresh() uint32      { return c.aprsRefresh }
func (c *Config) GetAPRSDescription() string  { return c.aprsDescription }

// Getter methods for Database section
func (c *Config) GetDatabaseEnabled() bool    { return c.databaseEnabled }
func (c *Config) GetDatabasePath() string     { return c.databasePath }
func (c *Config) GetDatabaseSyncHours() uint32 { return c.databaseSyncHours }
func (c *Config) GetDatabaseCacheSize() uint32 { return c.databaseCacheSize }
func (c *Config) GetDatabaseDebug() bool      { return c.databaseDebug }