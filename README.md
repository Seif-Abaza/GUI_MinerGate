# MinerGate Dashboard v1.0.3

Professional Mining Dashboard GUI for cryptocurrency mining operations.

## Features

- **Real-time Monitoring**: Live hashrate, efficiency, and worker status
- **Interactive Charts**: Hashrate, Temperature & Power charts with 24h history
- **Multi-device Support**: Monitor multiple ASIC miners simultaneously
- **FRP Integration**: Built-in FRP client for remote access
- **GoASIC Integration**: Automatic network scanning for ASIC devices
- **Auto-refresh**: Configurable automatic data updates
- **Secure API**: Encrypted API communication with retry logic
- **Plugin System**: Extensible plugin architecture
- **Auto-update**: Automatic update checking and installation

## Supported Languages

- English (en)
- Arabic (ar)

## Requirements

- Go 1.21 or higher
- Fyne dependencies (see below)

### Fyne Dependencies

**Linux:**
```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev
```

**macOS:**
```bash
xcode-select --install
```

**Windows:**
```bash
# Install MinGW-w64 or Visual Studio Build Tools
```

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/Seif-Abaza/MinerGate_GUI_Dashboard.git
cd MinerGate_GUI_Dashboard

# Install dependencies
make deps

# Build
make build

# Run
./build/bin/minergate
```

### Quick Start

```bash
make run
```

## Configuration

Configuration is stored in `config.json`:

```json
{
    "language": "en",
    "auto_refresh": true,
    "refresh_rate": 10,
    "theme": "dark",
    "api_endpoint": "http://localhost:8080/api/v1",
    "api_target_device": "http://localhost:8080/api/v1/devices/report",
    "frp_enabled": false,
    "goasic_enabled": true,
    "update_auto_check": true
}
```

## Project Structure

```
minergate/
├── cmd/minergate/main.go        # Entry point
├── internal/
│   ├── api/client.go            # API client
│   ├── charts/charts.go         # Interactive charts
│   ├── config/config.go         # Configuration
│   ├── gui/dashboard.go         # GUI
│   ├── models/models.go         # Data models
│   ├── plugins/manager.go       # Plugin manager
│   ├── update/updater.go        # Update system
│   ├── frp/client.go            # FRP client
│   └── goasic/manager.go        # ASIC manager
├── config.json                  # Configuration file
├── go.mod                       # Go module
└── Makefile                     # Build commands
```

## Security Features

- Encrypted API key storage (AES-256-GCM)
- TLS 1.2+ with secure cipher suites
- Rate limiting for API requests
- SHA256 checksum verification for updates
- Input validation and sanitization
- Secure HTTP client with timeout

## Available Make Commands

| Command | Description |
|---------|-------------|
| `make all` | Install dependencies and build |
| `make deps` | Install dependencies |
| `make build` | Build the application |
| `make run` | Run the application |
| `make test` | Run tests |
| `make clean` | Clean build artifacts |
| `make package` | Create distribution package |

## Supported ASIC Miners

- Bitmain Antminer (S19, S19 Pro, S19 XP, S19j Pro, L7, etc.)
- MicroBT Whatsminer (M30S, M30S++, M50, etc.)
- Canaan AvalonMiner
- IceRiver miners

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/farms/:id` | GET | Get farm data |
| `/farms/:id/miners` | GET | Get miners list |
| `/miners/:id/stats` | GET | Get miner stats |
| `/miners/:id/action` | POST | Execute action |
| `/miners/:id/history` | GET | Get historical data |

## Actions

- Restart miner
- Get errors
- Get fan status
- Get pool configuration
- Get power information

## License

MIT License

## Author

Seif Abaza

## Version History

- **v1.0.3**: FRP integration, GoASIC integration, interactive charts
- **v1.0.2**: Plugin system, auto-update
- **v1.0.1**: Basic monitoring
- **v1.0.0**: Initial release
