# Cerera Blockchain

[![Go](https://github.com/MVerseZ/cerera/actions/workflows/go.yml/badge.svg)](https://github.com/MVerseZ/cerera/actions/workflows/go.yml)
[![Go Version](https://img.shields.io/badge/go-1.25-blue?logo=go)](https://golang.org/dl/)
[![Go Report Card](https://goreportcard.com/badge/github.com/MVerseZ/cerera)](https://goreportcard.com/report/github.com/MVerseZ/cerera)
[![License: GPL v2](https://img.shields.io/badge/license-GPL--v2-blue)](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html)
[![GitHub stars](https://img.shields.io/github/stars/MVerseZ/cerera?style=flat)](https://github.com/MVerseZ/cerera/stargazers)
[![GitHub forks](https://img.shields.io/github/forks/MVerseZ/cerera?style=flat)](https://github.com/MVerseZ/cerera/network/members)
[![GitHub issues](https://img.shields.io/github/issues/MVerseZ/cerera)](https://github.com/MVerseZ/cerera/issues)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20windows%20%7C%20macos-lightgrey)

Cerera is a high-performance blockchain platform built in Go, designed for scalability, security, and developer-friendly features.

## Features

- **High Performance**: Optimized for high transaction throughput with Prometheus metrics
- **Secure Transactions**: Advanced cryptographic security with ECDSA signatures and BIP32/BIP39 HD wallet support
- **ICENet Protocol**: Custom P2P networking layer built on libp2p with DHT, gossip, and block sync
- **Pallada VM**: Stack-based virtual machine for smart contract execution (EVM-compatible opcodes)
- **Faucet System**: Built-in faucet with rate limiting and security controls
- **Account Management**: Comprehensive account system with mnemonic restore support
- **Storage Layer**: Efficient storage with Merkle tries and persistent/in-memory modes

## Architecture

```
cerera/
├── cmd/
│   ├── cerera/            # Main blockchain node entry point
│   └── cereractl/         # Command-line management tool
├── core/                  # Core blockchain primitives
│   ├── account/           # Account model
│   ├── address/           # Address type and helpers
│   ├── block/             # Block structure, genesis, validation
│   ├── chain/             # Blockchain management
│   ├── common/            # Shared types and math utilities
│   ├── crypto/            # Cryptographic functions
│   ├── pool/              # Transaction mempool
│   ├── storage/           # Persistent storage (vault/bitcask)
│   └── types/             # Core data types (transactions, packets)
├── icenet/                # ICENet P2P protocol layer
│   ├── consensus/         # Distributed consensus (voting, state)
│   ├── metrics/           # Network-level Prometheus metrics
│   ├── peers/             # Peer manager
│   ├── protocol/          # Wire protocol and messages
│   └── sync/              # Block synchronization
├── internal/              # Internal implementation packages
│   ├── coinbase/          # Coinbase, faucet, and staking logic
│   ├── consensus/         # Local consensus algorithm
│   ├── mesh/              # libp2p node, DHT, static peers
│   ├── miner/             # PoW mining worker
│   ├── network/           # HTTP API server
│   ├── observer/          # Event observer/bus
│   ├── service/           # Service registry and provider interface
│   └── validator/         # Transaction and block validation
├── pallada/               # Pallada smart contract VM
│   └── examples/          # VM usage examples
├── gigea/                 # Consensus event bus components
├── config/                # Node configuration
├── deployments/           # Docker Compose, Prometheus, Grafana configs
├── docs/                  # Architecture diagrams
├── grafana/               # Grafana provisioning
└── tests/                 # Integration test scripts (Python)
```

## Installation

### Prerequisites

- Go 1.25.0 or later
- Git

### Build from Source

```bash
# Clone the repository
git clone https://github.com/cerera/cerera.git
cd cerera

# Build the main node
go build ./cmd/cerera

# Build the CLI tool
go build ./cmd/cereractl
```

## Quick Start

### Running a Node

```bash
# Start with default settings
./cerera

# Start with custom port and HTTP API on port 9090
./cerera -port=31001 -http=9090

# Start with TLS enabled and disk storage
./cerera -s -mem=false

# Start without mining
./cerera -miner=false
```

### Command Line Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-port` | string | `"31000"` | P2P port for ICENet connection |
| `-key` | string | `""` | Path to PEM private key file |
| `-mode` | string | `"server"` | Operation mode: `server`, `client`, or `p2p` |
| `-http` | int | `8080` | HTTP server port for API endpoints |
| `-miner` | bool | `true` | Enable/disable block mining |
| `-mem` | bool | `true` | Storage mode: `true` for in-memory, `false` for disk |
| `-s` | bool | `false` | Enable HTTPS (TLS) |

## Configuration

Cerera generates configuration automatically on startup. You can also provide a JSON config file:

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
  "POOL": {
    "MaxSize": 1024
  },
  "IN_MEM": false,
  "AUTOGEN": true,
  "SEC": {
    "HTTP": {
      "TLS": false
    }
  }
}
```

## Faucet System

Cerera includes a built-in faucet for testing and development:

- **Rate Limiting**: Maximum 1 request per hour per address
- **Amount Limits**: 1–1000 tokens per request
- **Security**: Built-in validation and cooldown mechanisms

```bash
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "method": "faucet",
    "jsonrpc": "2.0",
    "id": 1,
    "params": ["0x...", 10]
  }'
```

## Pallada VM

Cerera includes **Pallada** — a stack-based virtual machine for smart contract execution. It implements EVM-compatible opcodes with a custom gas model.

Key features:
- 256-bit stack (max depth 1024)
- Linear byte-addressable memory (up to 1 MB)
- Persistent contract storage via `SLOAD`/`SSTORE`
- Inter-contract calls via `CALL`
- Gas metering for every operation

See [pallada/README.md](pallada/README.md) for full documentation.

## ICENet Protocol

**ICENet** is the custom P2P networking layer built on libp2p. It handles:

- Peer discovery via Kademlia DHT
- Block broadcasting via gossip pubsub
- Block synchronization between nodes
- Distributed voting-based consensus

## Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./core/block/...
go test ./core/pool/...
go test ./internal/coinbase/...
go test ./pallada/...
go test ./icenet/consensus/...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Python integration test scripts are available in the `tests/` directory.

## Monitoring

Cerera exposes Prometheus metrics. Pre-built Grafana dashboards are available in `deployments/grafana/dashboards/`:

- Chain height and block size
- Pool size and transaction throughput
- ICENet connections, block sync, consensus rounds
- Miner hash rate and difficulty
- Validator signing success/error rates

## Security Features

- **Cryptographic Security**: ECDSA signatures over secp256k1
- **Input Validation**: Comprehensive validation at the validator layer
- **Rate Limiting**: Built-in protection against spam and abuse
- **HD Wallets**: BIP32/BIP39 mnemonic generation and restore

## API Reference

### JSON-RPC Methods (POST /app)

| Method | Parameters | Description |
|--------|------------|-------------|
| `getBalance` | `address` | Get account balance |
| `faucet` | `address, amount` | Request tokens from faucet |
| `getBlock` | `height` | Get block by height |

### HTTP Endpoints

- `POST /app` — JSON-RPC endpoint
- `GET /status` — Node status and chain info

## Deployment

Docker Compose configurations for multi-node setups are available in `deployments/`

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a pull request

## License

This project is licensed under the GNU General Public License v2.0 — see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: Report bugs and request features on GitHub
- **Documentation**: Check the `docs/` and `deployments/` directories

## Links

- **Website**: [cerera-chain.ru](https://cerera-сhain.ru)
- **Documentation**: [docs.cerera-chain.ru](https://docs.cerera-сhain.ru)

---

**Cerera** — Building the future of decentralized applications.
