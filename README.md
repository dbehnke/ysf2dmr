# YSF2DMR Gateway - Go Implementation

A modern Go implementation of the YSF2DMR gateway with enhanced features including database-backed DMR ID resolution, goroutine-based architecture, and automatic RadioID.net synchronization.

## 🚀 Features

### Core Gateway Functionality
- **YSF ↔ DMR Protocol Conversion**: Seamless bridging between Yaesu System Fusion and DMR networks
- **AMBE Audio Processing**: High-quality voice conversion with error correction
- **WiresX Integration**: Support for Yaesu WiresX room management
- **Real-time Operation**: Sub-millisecond audio frame processing

### Modern Architecture
- **Goroutine-based Concurrency**: Native Go concurrency instead of C++ polling patterns
- **Channel Communication**: Type-safe inter-component messaging
- **Context Cancellation**: Graceful shutdown and cancellation propagation
- **Performance Monitoring**: Built-in statistics and health monitoring

### Advanced DMR ID Resolution
- **Database-backed Lookup**: SQLite database with 290,000+ user records
- **Automatic RadioID.net Sync**: Daily updates from RadioID.net CSV feed
- **Pure Go SQLite**: No CGO dependencies using modernc.org/sqlite
- **Rich User Data**: Full names, locations, not just callsigns
- **Intelligent Caching**: 5-minute cache with configurable size limits
- **Backward Compatibility**: Drop-in replacement for file-based lookups

## 📦 Installation

### Prerequisites
- Go 1.21 or later
- No CGO dependencies (pure Go implementation)

### Build from Source
```bash
git clone https://github.com/dbehnke/ysf2dmr.git
cd ysf2dmr
go mod download
cd cmd/ysf2dmr
go build -o ysf2dmr
```

## ⚙️ Configuration

### Modern Database Mode (Recommended)
```ini
[Info]
Callsign=YOUR_CALL
Location=Your Location
Description=YSF2DMR Gateway

[YSF Network]
LocalPort=42013
DstAddress=ysf.example.com
DstPort=42001

[DMR Network]
Id=YOUR_DMR_ID
Address=dmr.example.com
Port=62031
Password=your_password

[Database]
Enabled=1
Path=data/dmr_users.db
SyncHours=24
CacheSize=1000
Debug=0
```

### Legacy File Mode
```ini
[Database]
Enabled=0

[DMR Id Lookup]
File=DMRIds.dat
Time=24
```

## 🚦 Usage

### Standard Operation
```bash
./ysf2dmr -config YSF2DMR.ini
```

### Goroutine-based Implementation
```bash
cd cmd/ysf2dmr
go run main_goroutine.go -config YSF2DMR.ini
```

## 🏗️ Architecture

### Package Structure
```
├── cmd/ysf2dmr/           # Main application
│   ├── main.go            # Timer-based implementation
│   └── main_goroutine.go  # Goroutine-based implementation
├── internal/
│   ├── database/          # SQLite database layer
│   ├── radioid/           # RadioID.net synchronization
│   ├── lookup/            # DMR ID resolution interfaces
│   ├── network/           # YSF/DMR network protocols
│   ├── protocol/          # Protocol definitions
│   ├── codec/             # AMBE audio processing
│   └── config/            # Configuration management
└── pkg/                   # Public API packages
```

### Key Components

#### Database Layer
- **Pure Go SQLite**: Using modernc.org/sqlite (no CGO)
- **GORM ORM**: Clean abstractions with auto-migration
- **Optimized Performance**: WAL mode, indexes, connection pooling

#### RadioID Synchronization
- **Automatic Downloads**: From https://radioid.net/static/user.csv
- **Batch Processing**: 1000 records per transaction
- **Error Recovery**: Retry logic with exponential backoff
- **Progress Logging**: Real-time import status

#### Network Protocols
- **YSF Client**: Goroutine-based with channel communication
- **DMR Client**: Homebrew protocol with authentication
- **UDP Socket Management**: IPv4-only with proper binding

## 📊 Performance

### Database Performance
- **Import Speed**: 290,000+ records in ~13 seconds
- **Lookup Speed**: Sub-millisecond with caching
- **Memory Usage**: ~50MB for full database
- **Storage**: ~25MB SQLite database

### Network Performance
- **Frame Processing**: <1ms latency
- **Concurrent Connections**: Multiple YSF/DMR networks
- **Throughput**: Full duplex audio with minimal buffering

## 🔧 Development

### Running Tests
```bash
go test ./...
```

### Building Variants
```bash
# Timer-based (compatible with original C++)
go build -o ysf2dmr cmd/ysf2dmr/main.go

# Goroutine-based (modern Go architecture)
go build -o ysf2dmr_goroutines cmd/ysf2dmr/main_goroutine.go
```

## 📈 Monitoring

### Built-in Statistics
- DMR ID lookup hit/miss rates
- Network connection status
- Audio frame processing metrics
- Database sync status

### Logging Levels
- Debug: Detailed protocol analysis
- Info: Normal operation status
- Warn: Non-critical issues
- Error: Critical failures

## 🔄 Migration from C++

This Go implementation provides:
- **100% API compatibility** with existing configurations
- **Enhanced performance** through modern concurrency
- **Automatic updates** via database synchronization
- **Cross-platform builds** with no dependencies

### Migration Steps
1. Update configuration to enable database mode
2. Replace binary with Go implementation
3. Verify connectivity and audio quality
4. Monitor automatic RadioID.net sync

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## 📄 License

This project maintains compatibility with the original YSF2DMR licensing terms.

## 🙏 Acknowledgments

- Original YSF2DMR C++ implementation
- dmr-nexus project for database architecture inspiration
- RadioID.net for providing the user database
- Go community for excellent tooling and libraries

---

**Status**: Production ready with comprehensive testing
**Compatibility**: Drop-in replacement for C++ implementation
**Performance**: Optimized for modern multi-core systems