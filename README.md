# Cerera Blockchain

[![Go](https://github.com/MVerseZ/cerera/actions/workflows/go.yml/badge.svg)](https://github.com/MVerseZ/cerera/actions/workflows/go.yml)
[![Go Version](https://img.shields.io/badge/go-1.23-blue?logo=go)](https://golang.org/dl/)
[![Go Report Card](https://goreportcard.com/badge/github.com/MVerseZ/cerera)](https://goreportcard.com/report/github.com/MVerseZ/cerera)
[![License: GPL v2](https://img.shields.io/badge/license-GPL--v2-blue)](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html)
[![GitHub stars](https://img.shields.io/github/stars/MVerseZ/cerera?style=flat)](https://github.com/MVerseZ/cerera/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/MVerseZ/cerera?style=flat)](https://github.com/MVerseZ/cerera/network/members)
[![GitHub issues](https://img.shields.io/github/issues/MVerseZ/cerera)](https://github.com/MVerseZ/cerera/issues)
[![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)](https://github.com/MVerseZ/cerera/issues)
[![HitCount](https://hits.dwyl.com/MVerseZ/cerera.svg)](https://hits.dwyl.com/MVerseZ/cerera)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20windows%20%7C%20macos-lightgrey)

Cerera is a high-performance blockchain platform built in Go, designed for scalability, security, and developer-friendly features.

## 🚀 Features

- **High Performance**: Optimized for high transaction throughput
- **Secure Transactions**: Advanced cryptographic security with ECDSA signatures
- **P2P Network**: Decentralized peer-to-peer networking using libp2p
- **Faucet System**: Built-in faucet with rate limiting and security controls
- **Account Management**: Comprehensive account system with HD wallet support
- **Storage Layer**: Efficient storage with Merkle trees and trie structures

## 🏗️ Architecture

```
cerera/
├── cmd/                    # Main applications
│   ├── cerera/            # Main blockchain node
│   └── cereractl/         # Command-line tools
├── internal/              # Internal packages
│   ├── cerera/           # Core blockchain logic
│   │   ├── block/        # Block structure and validation
│   │   ├── chain/        # Blockchain management
│   │   ├── consensus/    # Consensus mechanisms
│   │   ├── crypto/       # Cryptographic functions
│   │   ├── miner/        # Mining implementation
│   │   ├── network/      # P2P networking
│   │   ├── storage/      # Storage layer (vault)
│   │   ├── types/        # Core data types
│   │   └── validator/    # Transaction validation
│   ├── coinbase/         # Coinbase and faucet logic
│   └── gigea/           # Additional consensus components
├── build/                # Pre-built libraries
└── tests/               # Test scripts and tools
```

## 🛠️ Installation

### Prerequisites

- Go 1.23.0 or later
- Git

### Build from Source

```bash
# Clone the repository
git clone https://github.com/cerera/cerera.git
cd cerera

# Build the main node
go build ./cmd/cerera

# Build the CLI tools
go build ./cmd/cereractl
```

## 🚀 Quick Start

### Running a Node

```bash
# Start a Cerera node with default settings
./cerera

# Start with custom configuration
./cerera -config=./configs/custom.json
```

### Command Line Flags

Cerera supports various command line flags for configuration:

```bash
# Basic usage
./cerera [flags]

# Available flags:
./cerera -addr=31000 -key=/path/to/key.pem -mode=server -http=8080 -miner=true -mem=true
```

#### Flag Reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-addr` | string | `"31000"` | P2P address for network connection |
| `-key` | string | `""` | Path to PEM private key file |
| `-mode` | string | `"server"` | Operation mode: `server`, `client`, or `p2p` |
| `-http` | int | `8080` | HTTP server port for API endpoints |
| `-miner` | bool | `true` | Enable/disable block mining |
| `-mem` | bool | `true` | Storage mode: `true` for in-memory, `false` for disk |

#### Examples

```bash
# Start as P2P node with custom port
./cerera -mode=p2p -addr=31001

# Start with disk storage and disabled mining
./cerera -mem=false -miner=false

# Start with custom HTTP port and key file
./cerera -http=9090 -key=./keys/node.pem

# Start as client mode
./cerera -mode=client -addr=127.0.0.1:31000
```

## 🔧 Configuration

Cerera uses JSON configuration files. Example configuration:

```json
{
  "Chain": {
    "ChainID": 12345,
    "Path": "./chain.dat"
  },
  "NetCfg": {
    "ADDR": "0x...",
    "PRIV": "-----BEGIN PRIVATE KEY-----..."
  },
  "IN_MEM": false,
  "AUTOGEN": true
}
```

## 💰 Faucet System

Cerera includes a built-in faucet system for testing and development:

- **Rate Limiting**: Maximum 1 request per hour per address
- **Amount Limits**: 1-1000 tokens per request
- **Security**: Built-in validation and cooldown mechanisms

### Using the Faucet

```bash
# Request tokens from faucet
curl -X POST http://localhost:1337/app \
  -H "Content-Type: application/json" \
  -d '{
    "method": "faucet",
    "jsonrpc": "2.0",
    "id": 1,
    "params": ["0x...", 10]
  }'
```

## 🧪 Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/cerera/types
go test ./internal/cerera/storage
go test ./internal/coinbase

# Run with verbose output
go test -v ./...
```

## 📊 Performance

- **Transaction Throughput**: High TPS with optimized validation
- **Block Time**: Configurable block generation intervals
- **Memory Usage**: Efficient memory management with in-memory and persistent modes
- **Network**: Low-latency P2P communication

## 🔒 Security Features

- **Cryptographic Security**: ECDSA signatures and secure hash functions
- **Input Validation**: Comprehensive validation of all inputs
- **Rate Limiting**: Built-in protection against spam and abuse
- **Account Security**: HD wallet support with BIP32/BIP39 standards

## 🌐 Network Protocol

Cerera uses libp2p for peer-to-peer networking:

- **Discovery**: Automatic peer discovery
- **Gossip Protocol**: Efficient message propagation
- **DHT**: Distributed hash table for peer management

## 📝 API Reference

### JSON-RPC Methods

- `getBalance(address)` - Get account balance
- `sendTransaction(from, to, amount)` - Send transaction
- `faucet(address, amount)` - Request tokens from faucet
- `getBlock(height)` - Get block by height
- `getTransaction(hash)` - Get transaction by hash

### HTTP Endpoints

- `POST /app` - JSON-RPC endpoint
- `GET /status` - Node status

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

### Development Setup

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./cmd/cerera
```

## 📄 License

This project is licensed under the GNU General Public License v2.0 - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: Check the `/docs` directory
- **Issues**: Report bugs and request features on GitHub
- **Discussions**: Join our community discussions

## 🔗 Links

- **Website**: [cerera](https://cerera-сhain.ru)
- **Documentation**: [docs.cerera](https://docs.cerera-сhain.ru)

---

**Cerera** - Building the future of decentralized applications 🚀
