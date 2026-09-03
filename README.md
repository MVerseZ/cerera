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
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.faucet",
    "params": ["0x...", "10"],
    "id": 1
  }'
```

## JSON-RPC API

Cerera exposes a [JSON-RPC 2.0](https://www.jsonrpc.org/specification) API over HTTP. Methods are routed through an internal service registry using the namespace `cerera.<service>.<method>`.

### Endpoint

```
POST http://localhost:<http-port>/
Content-Type: application/json
```

The HTTP server listens on the port from `-http` (default `8080`). In Docker Compose stacks, node1 is usually mapped to port `1337`. Integration tests often call `http://localhost:1337/app`; the server registers a catch-all handler on `/`, so both `/` and `/app` work.

WebSocket ping/pong is available at `GET /ws`.

Prometheus metrics: `GET /metrics`.

### Request format

```json
{
  "jsonrpc": "2.0",
  "method": "cerera.chain.height",
  "params": [],
  "id": 1
}
```

### Response format

Success:

```json
{
  "jsonrpc": "2.0",
  "result": 42,
  "id": 1
}
```

Errors are returned as HTTP 500 with a plain-text body, or inside `result` for business-logic failures (e.g. `"service account not found"`).

### Services overview

| Namespace | Internal service | Purpose |
|-----------|------------------|---------|
| `cerera.account.*` | Vault | Wallets, balances, faucet |
| `cerera.chain.*` | Chain | Blocks, height, chain info |
| `cerera.transaction.*` | Validator | Send and query transactions |
| `cerera.pool.*` | Mempool | Pending transactions, gas limits |

**Chain vs account data:** `cerera.chain.*` reflects the canonical blockchain and should match on synced nodes. `cerera.account.*` reads the local vault (each node has its own identity address plus merged peer accounts). For multi-node checks, compare `chain.height` and head block hash; use `account.getBalance` for a specific address rather than comparing full `getAll` snapshots.

Full request/response examples: [API.md](API.md).

---

### Account service — `cerera.account.*`

Local vault operations. Amounts are in **CER** (decimal strings recommended for faucet).

| Method | Params | Returns | Description |
|--------|--------|---------|-------------|
| `getAll` | `[]` | `map[address]balance` | All accounts in the local vault |
| `getCount` | `[]` | `int` | Number of accounts |
| `getTotalSupply` | `[]` | `*big.Int` | Sum of vault balances |
| `getBalance` | `[address]` | `*big.Int` | Balance for one address (`0` if missing) |
| `create` | `[passphrase]` | `{address, priv, pub, mnemonic}` | Create a new HD wallet account |
| `restore` | `[mnemonic, passphrase]` | `{address, priv, pub}` | Restore from mnemonic |
| `verify` | `[address, passphrase]` | `bool` | Check address + passphrase |
| `faucet` | `[address, amount]` | `"Faucet successful"` | Credit tokens (string `"10.5"` or number) |
| `inputs` | `[address]` | `map[hash]amount` | UTXO-style inputs for an account |

**Example — create account and check balance:**

```bash
curl -s http://localhost:8080/ -H "Content-Type: application/json" -d '{
  "jsonrpc":"2.0","method":"cerera.account.create","params":["secret"],"id":1
}'

curl -s http://localhost:8080/ -H "Content-Type: application/json" -d '{
  "jsonrpc":"2.0","method":"cerera.account.getBalance","params":["0x..."],"id":2
}'
```

---

### Chain service — `cerera.chain.*`

Read-only blockchain data (same on all synced nodes).

| Method | Params | Returns | Description |
|--------|--------|---------|-------------|
| `height` | `[]` | `int` | Current block height |
| `getCurrentHeight` | `[]` | `int` | Same as `height` |
| `getChainId` | `[]` | `int` | Chain ID |
| `getInfo` | `[]` | `BlockChainStatus` | `{total, latest, size, avgTime, txs, gas, gasPrice, difficulty, chainWork}` |
| `getLatestBlock` | `[]` | `Block` | Current head block |
| `getBlockByIndex` | `[height]` | `Block` | Block by index (0-based height) |
| `getBlockByHash` | `[hash]` | `Block` | Block by hash hex string |

**Example — get height and head block:**

```bash
curl -s http://localhost:8080/ -H "Content-Type: application/json" -d '{
  "jsonrpc":"2.0","method":"cerera.chain.height","params":[],"id":1
}'

curl -s http://localhost:8080/ -H "Content-Type: application/json" -d '{
  "jsonrpc":"2.0","method":"cerera.chain.getBlockByIndex","params":[0],"id":2
}'
```

---

### Transaction service — `cerera.transaction.*`

Handled by the validator. Transactions are signed locally and queued in the mempool.

| Method | Params | Returns | Description |
|--------|--------|---------|-------------|
| `send` | `[privKey, toHex, amount, gas, msg]` | `txHash` | Sign with PEM private key, enqueue for mining |
| `get` | `[txHash]` | `{hash, from, to, value, gas, data}` | Look up a confirmed transaction |
| `stake` | — | `null` | Reserved (not implemented) |

**`send` parameters:**

1. `privKey` — PEM-encoded private key (`priv` from `account.create` / `account.restore`)
2. `toHex` — recipient address hex string
3. `amount` — transfer amount (float, in CER)
4. `gas` — gas limit (float)
5. `msg` — optional message string (max 1024 chars)

Nonce is assigned automatically. Minimum gas price comes from `cerera.pool.minGas`.

**Example — send a transfer:**

```bash
curl -s http://localhost:8080/ -H "Content-Type: application/json" -d '{
  "jsonrpc":"2.0",
  "method":"cerera.transaction.send",
  "params":["-----BEGIN PRIVATE KEY-----\n...","0xRecipient...",1.0,21000,"hello"],
  "id":1
}'
```

---

### Pool service — `cerera.pool.*`

Mempool inspection and mining helpers.

| Method | Params | Returns | Description |
|--------|--------|---------|-------------|
| `getInfo` | `[]` | `MemPoolInfo` | `{size, bytes, usage, maxMempool, mempoolminfee, unbroadCastCount, hashes, txs}` |
| `minGas` | `[]` | `float64` | Minimum gas price |
| `getPendingTransactions` | `[]` | `[GTransaction, ...]` | All pending txs |
| `getTransaction` | `[txHash]` | `GTransaction` | Pending tx by hash |
| `getTopN` | `[n]` | `[GTransaction, ...]` | Top-N txs by fee |
| `getMiningPackage` | `[n]` | `[GTransaction, ...]` | CPFP-aware package for block assembly |
| `removeFromPool` | `[txHash]` | `bool` | Remove a pending tx |

---

### Typical workflow

```bash
# 1. Create account
cerera.account.create ["passphrase"]

# 2. Fund via faucet
cerera.account.faucet ["0x...", "100"]

# 3. Send transaction
cerera.transaction.send [privKey, "0x...", 10.0, 21000, "payment"]

# 4. Wait for mining, then verify on chain
cerera.chain.height []
cerera.transaction.get ["0xTxHash..."]

# 5. Check mempool while waiting
cerera.pool.getInfo []
```

### Error messages

| Message | Meaning |
|---------|---------|
| `service <name> not found` | Unknown namespace or service not registered |
| `method <name> not found in <service>` | Method name typo or not exposed |
| `invalid parameters for create` | Wrong number/types of params |
| `negative gas or value` | Invalid gas or amount |
| `message too long` | Message exceeds 1024 characters |
| `Faucet successful` | Faucet succeeded (not an error) |

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

- [API reference (detailed examples)](API.md)
- [Multi-node smoke tests](deployments/CONSENSUS_SMOKE.md)
- [Pallada VM](pallada/README.md)

---

**Cerera** — Building the future of decentralized applications.
