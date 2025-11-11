package config

import (
	"os"
	"testing"
)

func TestConfig_LoadFromFile(t *testing.T) {
	// Create a temporary config file for testing
	testConfig := `[Info]
RXFrequency=435000000
TXFrequency=435000000
Power=1
Latitude=40.7128
Longitude=-74.0060
Height=10
Location=New York City
Description=Test Repeater
URL=https://example.com

[YSF Network]
Callsign=G4KLX
Suffix=RPT
DstAddress=127.0.0.1
DstPort=42000
LocalAddress=127.0.0.1
LocalPort=42013
EnableWiresX=1
RemoteGateway=0
HangTime=1000
WiresXMakeUpper=1
DT1=1,34,97,95,43,3,17,0,0,0
DT2=0,0,0,0,108,32,28,32,3,8
Daemon=0

[DMR Network]
Id=1234567
StartupDstId=9990
StartupPC=1
Address=44.131.4.1
Port=62031
Jitter=500
EnableUnlink=1
TGUnlink=4000
PCUnlink=0
Password=PASSWORD
TGListFile=TGList-DMR.txt
Debug=0

[DMR Id Lookup]
File=DMRIds.dat
Time=24
DropUnknown=0

[Log]
DisplayLevel=1
FileLevel=1
FilePath=.
FileRoot=YSF2DMR

[aprs.fi]
Enable=0
AprsCallsign=G4KLX
Server=euro.aprs2.net
Port=14580
Password=9999
APIKey=TestAPIKey
Refresh=240
Description=APRS Description`

	// Create temporary file
	tmpfile, err := os.CreateTemp("", "test_config_*.ini")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testConfig)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Test loading the config
	config := NewConfig(tmpfile.Name())
	err = config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test Info section
	if config.GetRxFrequency() != 435000000 {
		t.Errorf("GetRxFrequency() = %d, want 435000000", config.GetRxFrequency())
	}
	if config.GetTxFrequency() != 435000000 {
		t.Errorf("GetTxFrequency() = %d, want 435000000", config.GetTxFrequency())
	}
	if config.GetLatitude() != 40.7128 {
		t.Errorf("GetLatitude() = %f, want 40.7128", config.GetLatitude())
	}
	if config.GetLocation() != "New York City" {
		t.Errorf("GetLocation() = %q, want %q", config.GetLocation(), "New York City")
	}

	// Test YSF Network section
	if config.GetCallsign() != "G4KLX" {
		t.Errorf("GetCallsign() = %q, want %q", config.GetCallsign(), "G4KLX")
	}
	if config.GetSuffix() != "RPT" {
		t.Errorf("GetSuffix() = %q, want %q", config.GetSuffix(), "RPT")
	}
	if config.GetDstPort() != 42000 {
		t.Errorf("GetDstPort() = %d, want 42000", config.GetDstPort())
	}
	if !config.GetEnableWiresX() {
		t.Error("GetEnableWiresX() = false, want true")
	}

	// Test DMR Network section
	if config.GetDMRId() != 1234567 {
		t.Errorf("GetDMRId() = %d, want 1234567", config.GetDMRId())
	}
	if config.GetDMRDstId() != 9990 {
		t.Errorf("GetDMRDstId() = %d, want 9990", config.GetDMRDstId())
	}
	if config.GetDMRNetworkAddress() != "44.131.4.1" {
		t.Errorf("GetDMRNetworkAddress() = %q, want %q", config.GetDMRNetworkAddress(), "44.131.4.1")
	}

	// Test Log section
	if config.GetLogDisplayLevel() != 1 {
		t.Errorf("GetLogDisplayLevel() = %d, want 1", config.GetLogDisplayLevel())
	}
	if config.GetLogFileRoot() != "YSF2DMR" {
		t.Errorf("GetLogFileRoot() = %q, want %q", config.GetLogFileRoot(), "YSF2DMR")
	}
}

func TestConfig_LoadFromString(t *testing.T) {
	testConfig := `[YSF Network]
Callsign=TEST
DstPort=12345
EnableWiresX=0

[DMR Network]
Id=7654321
StartupDstId=91`

	config := NewConfig("")
	err := config.LoadFromString(testConfig)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	if config.GetCallsign() != "TEST" {
		t.Errorf("GetCallsign() = %q, want %q", config.GetCallsign(), "TEST")
	}
	if config.GetDstPort() != 12345 {
		t.Errorf("GetDstPort() = %d, want 12345", config.GetDstPort())
	}
	if config.GetEnableWiresX() {
		t.Error("GetEnableWiresX() = true, want false")
	}
	if config.GetDMRId() != 7654321 {
		t.Errorf("GetDMRId() = %d, want 7654321", config.GetDMRId())
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	config := NewConfig("")

	// Test default values
	if config.GetCallsign() != "" {
		t.Errorf("GetCallsign() default = %q, want empty string", config.GetCallsign())
	}
	if config.GetDstPort() != 42000 { // Default value set in NewConfig
		t.Errorf("GetDstPort() default = %d, want 42000", config.GetDstPort())
	}
	if config.GetEnableWiresX() {
		t.Error("GetEnableWiresX() default = true, want false")
	}
	if config.GetLogDisplayLevel() != 0 {
		t.Errorf("GetLogDisplayLevel() default = %d, want 0", config.GetLogDisplayLevel())
	}
}

func TestConfig_InvalidFile(t *testing.T) {
	config := NewConfig("/nonexistent/file.ini")
	err := config.Load()
	if err == nil {
		t.Error("Load() with nonexistent file should return error")
	}
}

func TestConfig_YSFDataTypes(t *testing.T) {
	testConfig := `[YSF Network]
DT1=1,34,97,95,43,3,17,0,0,0
DT2=0,0,0,0,108,32,28,32,3,8`

	config := NewConfig("")
	err := config.LoadFromString(testConfig)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	dt1 := config.GetYsfDT1()
	expectedDT1 := []uint8{1, 34, 97, 95, 43, 3, 17, 0, 0, 0}
	if len(dt1) != len(expectedDT1) {
		t.Errorf("GetYsfDT1() length = %d, want %d", len(dt1), len(expectedDT1))
	} else {
		for i, v := range expectedDT1 {
			if dt1[i] != v {
				t.Errorf("GetYsfDT1()[%d] = %d, want %d", i, dt1[i], v)
			}
		}
	}

	dt2 := config.GetYsfDT2()
	expectedDT2 := []uint8{0, 0, 0, 0, 108, 32, 28, 32, 3, 8}
	if len(dt2) != len(expectedDT2) {
		t.Errorf("GetYsfDT2() length = %d, want %d", len(dt2), len(expectedDT2))
	} else {
		for i, v := range expectedDT2 {
			if dt2[i] != v {
				t.Errorf("GetYsfDT2()[%d] = %d, want %d", i, dt2[i], v)
			}
		}
	}
}

func TestConfig_BooleanValues(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		getValue func(*Config) bool
		want     bool
	}{
		{
			name:     "EnableWiresX true with 1",
			config:   "[YSF Network]\nEnableWiresX=1",
			getValue: func(c *Config) bool { return c.GetEnableWiresX() },
			want:     true,
		},
		{
			name:     "EnableWiresX false with 0",
			config:   "[YSF Network]\nEnableWiresX=0",
			getValue: func(c *Config) bool { return c.GetEnableWiresX() },
			want:     false,
		},
		{
			name:     "DMR PC true",
			config:   "[DMR Network]\nStartupPC=1",
			getValue: func(c *Config) bool { return c.GetDMRPC() },
			want:     true,
		},
		{
			name:     "DMR Debug false",
			config:   "[DMR Network]\nDebug=0",
			getValue: func(c *Config) bool { return c.GetDMRNetworkDebug() },
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewConfig("")
			err := config.LoadFromString(tt.config)
			if err != nil {
				t.Fatalf("LoadFromString() error = %v", err)
			}

			got := tt.getValue(config)
			if got != tt.want {
				t.Errorf("getValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_NumericValues(t *testing.T) {
	testConfig := `[Info]
RXFrequency=145800000
TXFrequency=145200000
Power=25
Latitude=40.7128
Longitude=-74.0060
Height=100

[YSF Network]
DstPort=42000
LocalPort=42013
HangTime=1500

[DMR Network]
Id=1234567
Port=62031
Jitter=300`

	config := NewConfig("")
	err := config.LoadFromString(testConfig)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	// Test unsigned integers
	if config.GetRxFrequency() != 145800000 {
		t.Errorf("GetRxFrequency() = %d, want 145800000", config.GetRxFrequency())
	}
	if config.GetPower() != 25 {
		t.Errorf("GetPower() = %d, want 25", config.GetPower())
	}

	// Test floats
	if config.GetLatitude() != 40.7128 {
		t.Errorf("GetLatitude() = %f, want 40.7128", config.GetLatitude())
	}
	if config.GetLongitude() != -74.0060 {
		t.Errorf("GetLongitude() = %f, want -74.0060", config.GetLongitude())
	}

	// Test signed integers
	if config.GetHeight() != 100 {
		t.Errorf("GetHeight() = %d, want 100", config.GetHeight())
	}
}

func TestConfig_CommentedLines(t *testing.T) {
	testConfig := `[YSF Network]
Callsign=G4KLX
# This is a comment
#Suffix=COMMENTED
Suffix=ACTIVE
# Another comment
DstPort=42000`

	config := NewConfig("")
	err := config.LoadFromString(testConfig)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	if config.GetCallsign() != "G4KLX" {
		t.Errorf("GetCallsign() = %q, want %q", config.GetCallsign(), "G4KLX")
	}
	if config.GetSuffix() != "ACTIVE" {
		t.Errorf("GetSuffix() = %q, want %q", config.GetSuffix(), "ACTIVE")
	}
	if config.GetDstPort() != 42000 {
		t.Errorf("GetDstPort() = %d, want 42000", config.GetDstPort())
	}
}

func TestConfig_MissingSection(t *testing.T) {
	testConfig := `[Nonexistent Section]
SomeKey=SomeValue`

	config := NewConfig("")
	err := config.LoadFromString(testConfig)
	if err != nil {
		t.Fatalf("LoadFromString() error = %v", err)
	}

	// Should return default values for missing sections
	if config.GetCallsign() != "" {
		t.Errorf("GetCallsign() with missing section = %q, want empty string", config.GetCallsign())
	}
}

// Benchmark tests
func BenchmarkConfig_Load(b *testing.B) {
	// Create a temporary config file
	testConfig := `[YSF Network]
Callsign=G4KLX
DstPort=42000
EnableWiresX=1

[DMR Network]
Id=1234567
StartupDstId=9990`

	tmpfile, err := os.CreateTemp("", "bench_config_*.ini")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testConfig)); err != nil {
		b.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatalf("Failed to close temp file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := NewConfig(tmpfile.Name())
		config.Load()
	}
}

func BenchmarkConfig_GetValues(b *testing.B) {
	config := NewConfig("")
	testConfig := `[YSF Network]
Callsign=G4KLX
DstPort=42000
EnableWiresX=1`

	config.LoadFromString(testConfig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.GetCallsign()
		_ = config.GetDstPort()
		_ = config.GetEnableWiresX()
	}
}