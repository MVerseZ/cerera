# Cerera Blockchain API Documentation

## Overview

Cerera blockchain provides a JSON-RPC API for interacting with the blockchain network. All API calls are made through the Service Registry, which routes requests to the appropriate service based on the method name.

## API Endpoint

```
POST /app
Content-Type: application/json
```

## Request Format

All requests follow the JSON-RPC 2.0 specification:

```json
{
  "jsonrpc": "2.0",
  "method": "cerera.<service>.<method>",
  "params": [...],
  "id": 1
}
```

## Response Format

```json
{
  "jsonrpc": "2.0",
  "result": {...},
  "id": 1
}
```

## Account Service (`cerera.account.*`)

The Account Service (Vault) manages accounts, balances, and wallet operations.

### Methods

#### `getAll`

Returns all accounts in the vault.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.getAll",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "0x...": 1000000000000000000,
    "0x...": 500000000000000000
  },
  "id": 1
}
```

**Note:** Returns a map where keys are addresses (as hex strings) and values are balances (as float64).

#### `getCount`

Returns the total number of accounts.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.getCount",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": 5,
  "id": 1
}
```

#### `create`

Creates a new account with a passphrase.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.create",
  "params": ["your-passphrase"],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "address": "0x...",
    "priv": "-----BEGIN PRIVATE KEY-----...",
    "pub": "-----BEGIN PUBLIC KEY-----...",
    "mnemonic": "word1 word2 ... word24"
  },
  "id": 1
}
```

#### `restore`

Restores an account from a mnemonic phrase.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.restore",
  "params": ["word1 word2 ... word24", "passphrase"],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "addr": "0x...",
    "priv": "-----BEGIN PRIVATE KEY-----...",
    "pub": "-----BEGIN PUBLIC KEY-----..."
  },
  "id": 1
}
```

#### `verify`

Verifies account credentials.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.verify",
  "params": ["0x...", "passphrase"],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": true,
  "id": 1
}
```

#### `getBalance`

Gets the balance of an account.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.getBalance",
  "params": ["0x..."],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": 1000000000000000000,
  "id": 1
}
```

**Note:** Returns balance as a float64 number (in CER).

#### `faucet`

Requests tokens from the faucet.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.faucet",
  "params": ["0x...", "10.5"],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": "Faucet successful",
  "id": 1
}
```

**Note:** Amount can be provided as a decimal string (e.g., `"10.5"`) or as a float number.

#### `inputs`

Gets transaction inputs for an account.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.account.inputs",
  "params": ["0x..."],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": [...],
  "id": 1
}
```

## Chain Service (`cerera.chain.*`)

The Chain Service provides access to blockchain data and information.

### Methods

#### `getInfo`

Returns comprehensive blockchain status information.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.chain.getInfo",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "total": 100,
    "chainWork": 12345,
    "latest": "0x...",
    "size": 1024000,
    "avgTime": 30.5,
    "txs": 500,
    "gas": 1000000
  },
  "id": 1
}
```

**Note:** The `gasPrice` field is not included in the response. Only `gas` (total gas used) is returned.

#### `height`

Returns the current blockchain height (number of blocks).

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.chain.height",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": 100,
  "id": 1
}
```

#### `getBlockByIndex`

Gets a block by its index (height).

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.chain.getBlockByIndex",
  "params": [0],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "header": {
      "ctx": 0,
      "difficulty": 1,
      "extraData": "...",
      "gasLimit": 1000000,
      "gasUsed": 21000,
      "height": 0,
      "index": 0,
      "node": "0x...",
      "chainId": 12345,
      "prevHash": "0x...",
      "stateRoot": "0x...",
      "size": 1024,
      "timestamp": 1234567890,
      "version": "...",
      "nonce": 0
    },
    "transactions": [...],
    "confirmations": 0,
    "hash": "0x..."
  },
  "id": 1
}
```

#### `getBlock`

Gets a block by its hash.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.chain.getBlock",
  "params": ["0x..."],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "header": {
      "ctx": 0,
      "difficulty": 1,
      "extraData": "...",
      "gasLimit": 1000000,
      "gasUsed": 21000,
      "height": 0,
      "index": 0,
      "node": "0x...",
      "chainId": 12345,
      "prevHash": "0x...",
      "stateRoot": "0x...",
      "size": 1024,
      "timestamp": 1234567890,
      "version": "...",
      "nonce": 0
    },
    "transactions": [...],
    "confirmations": 0,
    "hash": "0x..."
  },
  "id": 1
}
```

#### `getBlockHeader`

Gets only the header of a block by its hash.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.chain.getBlockHeader",
  "params": ["0x..."],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "ctx": 0,
    "difficulty": 1,
    "extraData": "...",
    "gasLimit": 1000000,
    "gasUsed": 21000,
    "height": 0,
    "index": 0,
    "node": "0x...",
    "chainId": 12345,
    "prevHash": "0x...",
    "stateRoot": "0x...",
    "size": 1024,
    "timestamp": 1234567890,
    "version": "...",
    "nonce": 0
  },
  "id": 1
}
```

## Transaction Service (`cerera.transaction.*`)

The Transaction Service handles transaction creation, signing, and retrieval.

### Methods

#### `create`

Creates a new transaction (without broadcasting).

**Request (Typed DTO):**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.transaction.create",
  "params": [{
    "key": "xpub...",
    "nonce": 0,
    "to": "0x...",
    "amount": "1.5",
    "gas": 21000,
    "msg": "Hello"
  }],
  "id": 1
}
```

**Request (Legacy - Positional Parameters):**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.transaction.create",
  "params": [
    "xpub...",
    0,
    "0x...",
    1.5,
    21000,
    "Hello"
  ],
  "id": 1
}
```

**Note:** The `key` parameter should be a public key (B58 serialized format, e.g., `xpub...`), which can be obtained from the `pub` field when creating an account via `cerera.account.create`.

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "hash": "0x...",
    "from": "0x...",
    "to": "0x...",
    "value": "1500000000000000000",
    "gas": 21000,
    "data": "Hello",
    ...
  },
  "id": 1
}
```

**Note:** 
- `amount` should be provided as a decimal string (e.g., `"1.5"`) for precision
- `to` can be an address object or hex string
- `gas` must be non-negative
- `key` should be a public key (B58 serialized format, e.g., `xpub...`), obtained from the `pub` field when creating an account

#### `send`

Creates, signs, and queues a transaction for mining.

**Request (Typed DTO):**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.transaction.send",
  "params": [{
    "key": "xpub...",
    "toHex": "0x...",
    "amount": "1.5",
    "gas": 21000,
    "msg": "Hello"
  }],
  "id": 1
}
```

**Request (Legacy - Positional Parameters):**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.transaction.send",
  "params": [
    "xpub...",
    "0x...",
    1.5,
    21000,
    "Hello"
  ],
  "id": 1
}
```

**Note:** The `key` parameter should be a public key (B58 serialized format, e.g., `xpub...`), which can be obtained from the `pub` field when creating an account via `cerera.account.create`.

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": "0x...",
  "id": 1
}
```

**Note:**
- Returns the transaction hash
- Transaction is automatically queued in the mempool
- Nonce is automatically incremented
- Message length is limited to 1024 characters
- `key` should be a public key (B58 serialized format, e.g., `xpub...`), obtained from the `pub` field when creating an account

#### `get`

Retrieves a transaction by its hash.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.transaction.get",
  "params": ["0x..."],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "hash": "0x...",
    "from": "0x...",
    "to": "0x...",
    "value": "1500000000000000000",
    "gas": 21000,
    "data": "Hello"
  },
  "id": 1
}
```

**Note:** 
- The `to` field may be `null` for contract creation transactions.
- The `block` and `index` fields are not included in the response.

## Pool Service (`cerera.pool.*`)

The Pool Service provides information about the transaction mempool.

### Methods

#### `getInfo`

Returns mempool information.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.pool.getInfo",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "size": 10,
    "bytes": 1024,
    "usage": 2048,
    "maxMempool": 1000,
    "mempoolminfee": 1,
    "unbroadCastCount": 2,
    "hashes": ["0x...", "0x..."],
    "txs": [...]
  },
  "id": 1
}
```

#### `minGas`

Returns the minimum gas price required for transactions.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "cerera.pool.minGas",
  "params": [],
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": 0.000000001,
  "id": 1
}
```

## Examples

### Complete Workflow: Create Account, Get Balance, Send Transaction

```bash
# 1. Create a new account
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.create",
    "params": ["my-secure-passphrase"],
    "id": 1
  }'

# 2. Get account balance
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.getBalance",
    "params": ["0x..."],
    "id": 2
  }'

# 3. Request tokens from faucet
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.faucet",
    "params": ["0x...", "100"],
    "id": 3
  }'

# 4. Send a transaction
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.transaction.send",
    "params": [{
      "key": "xpub...",
      "toHex": "0x...",
      "amount": "10.5",
      "gas": 21000,
      "msg": "Payment"
    }],
    "id": 4
  }'

# 5. Get transaction details
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.transaction.get",
    "params": ["0x..."],
    "id": 5
  }'

# 6. Get blockchain info
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.chain.getInfo",
    "params": [],
    "id": 6
  }'
```

## Error Handling

Errors are returned in the `result` field as error strings or error objects:

```json
{
  "jsonrpc": "2.0",
  "result": "service account not found",
  "id": 1
}
```

Common error messages:
- `"service <name> not found"` - Service not registered
- `"invalid parameters for <method>"` - Parameter validation failed
- `"parameter type mismatch for <method>"` - Type assertion failed
- `"negative gas or value"` - Invalid gas or amount value
- `"message too long"` - Message exceeds 1024 characters
- `"Error while verify"` - Account verification failed
- `"Faucet successful"` - Faucet operation succeeded (not an error)

## Service Registry Implementation

The Service Registry (`internal/cerera/service/registry.go`) provides the following functionality:

- **Service Registration**: Services register themselves with a unique name
- **Service Resolution**: Aliases are mapped to internal service names
- **Method Parsing**: Methods in format `cerera.<service>.<method>` are parsed and routed
- **Thread-Safe Access**: All operations are protected by mutex locks

### Internal Service Names

- Account Service: `D5_VAULT_CERERA_001_1_7`
- Chain Service: `CHAIN_CERERA_001_1_7`
- Pool Service: `POOL_CERERA_001_1_3`
- Transaction Service: `CERERA_VALIDATOR_54013.10.25`

## Notes

- All amounts should be provided as decimal strings (e.g., `"1.5"`) when using typed DTOs for precision
- Addresses can be provided as hex strings (e.g., `"0x..."`) or address objects
- Gas values are in CER (Cerera's native token)
- Transaction nonces are automatically managed for `send` operations
- The API supports both typed DTOs and legacy positional parameters for backward compatibility

