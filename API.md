# Cerera Blockchain API Documentation

## Overview

Cerera blockchain provides a JSON-RPC API for interacting with the blockchain network. All API calls are made through the Service Registry, which routes requests to the appropriate service based on the method name.

## API Endpoint

Default HTTP port is **`8080`** (CLI flag `-http`). All JSON-RPC calls go to the root path:

```
POST http://<host>:<port>/
Content-Type: application/json
```

WebSocket (ping/pong only, not JSON-RPC):

```
WS ws://<host>:<port>/ws
```

CORS is enabled (`Access-Control-Allow-Origin: *`), so a browser app on another origin (e.g. `http://localhost:5173`) can call the node directly in development.

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

**Notes:**
- `avgTime` — average interval between **mined** blocks (height ≥ 1), in **seconds** (`float64`). The genesis→block#1 gap is excluded. If there is only genesis or a single mined block, the value is `0` and may be omitted from JSON (`omitempty`).
- `latest` — hash of the chain tip (hex string).
- `total` — block count including genesis.
- The `gasPrice` field is not included in the response. Only `gas` (total gas used) is returned.

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
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.create",
    "params": ["my-secure-passphrase"],
    "id": 1
  }'

# 2. Get account balance
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.getBalance",
    "params": ["0x..."],
    "id": 2
  }'

# 3. Request tokens from faucet
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.account.faucet",
    "params": ["0x...", "100"],
    "id": 3
  }'

# 4. Send a transaction
curl -X POST http://localhost:8080/ \
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
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.transaction.get",
    "params": ["0x..."],
    "id": 5
  }'

# 6. Get blockchain info
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "cerera.chain.getInfo",
    "params": [],
    "id": 6
  }'
```

## Frontend Integration Guide

This section describes how to integrate a web frontend (React, Vue, Svelte, etc.) with the Cerera JSON-RPC API.

### Architecture overview

```
Browser UI  ──fetch POST /──►  Cerera node (:8080)
     │                              │
     │                              ├─ cerera.chain.*
     │                              ├─ cerera.account.*
     │                              ├─ cerera.transaction.*
     │                              └─ cerera.pool.*
     │
     └── optional: Vite dev proxy (/api → node) in development
```

- **Transport:** HTTP `POST` with JSON body (JSON-RPC 2.0 shape).
- **No batch RPC:** send one method per request.
- **WebSocket `/ws`:** ping/pong only; block/tx events are **not** streamed over WS yet — use polling for dashboards.
- **Server timeout:** 5 seconds per request (`internal/network`).

### Environment variables

```env
# .env.local (Vite / Next.js)
VITE_CERERA_RPC_URL=http://127.0.0.1:8080/
# or behind dev proxy:
# VITE_CERERA_RPC_URL=/api/rpc
```

Never put private keys or mnemonics in frontend env vars. Signing stays on the backend or in a wallet extension.

### TypeScript RPC client

Minimal client with typed helpers for common calls:

```typescript
// src/lib/cerera-rpc.ts

export type JsonRpcRequest = {
  jsonrpc: "2.0";
  method: string;
  params: unknown[];
  id: number;
};

export type JsonRpcResponse<T = unknown> = {
  jsonrpc: "2.0";
  id: number;
  result?: T;
  error?: { code: number; message: string };
};

export type ChainInfo = {
  total: number;
  chainWork: number;
  latest: string;
  size: number;
  avgTime?: number; // seconds between mined blocks; 0 = omit
  txs: number;
  gas: number;
  difficulty?: number;
};

export type BlockHeader = {
  height: number;
  index: number;
  timestamp: number; // Unix ms — divide by 1000 for Date
  gasLimit: number;
  gasUsed: number;
  node: string;
  chainId: number;
  prevHash: string;
  stateRoot: string;
  difficulty: number;
  nonce: number;
};

let nextId = 1;

export function createCereraClient(baseUrl: string) {
  async function call<T>(method: string, params: unknown[] = []): Promise<T> {
    const body: JsonRpcRequest = {
      jsonrpc: "2.0",
      method,
      params,
      id: nextId++,
    };

    const res = await fetch(baseUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
      // Execute() failures return HTTP 500 with plain text body
      const text = await res.text();
      throw new Error(`HTTP ${res.status}: ${text}`);
    }

    const json = (await res.json()) as JsonRpcResponse<T>;
    if (json.error) {
      throw new Error(json.error.message);
    }
    return json.result as T;
  }

  return {
    chain: {
      height: () => call<number>("cerera.chain.height"),
      getInfo: () => call<ChainInfo>("cerera.chain.getInfo"),
      getBlockByIndex: (index: number) =>
        call<{ header: BlockHeader; hash: string; transactions: unknown[] }>(
          "cerera.chain.getBlockByIndex",
          [index]
        ),
    },
    account: {
      getBalance: (address: string) =>
        call<number>("cerera.account.getBalance", [address]),
      getCount: () => call<number>("cerera.account.getCount"),
      faucet: (address: string, amount: string) =>
        call<string>("cerera.account.faucet", [address, amount]),
    },
    pool: {
      getInfo: () => call<Record<string, unknown>>("cerera.pool.getInfo"),
      minGas: () => call<number>("cerera.pool.minGas"),
    },
    transaction: {
      send: (dto: {
        key: string;
        toHex: string;
        amount: string;
        gas: number;
        msg?: string;
      }) => call<string>("cerera.transaction.send", [dto]),
      get: (hash: string) =>
        call<Record<string, unknown>>("cerera.transaction.get", [hash]),
    },
    raw: call,
  };
}
```

### React: dashboard with polling

Typical explorer/dashboard needs chain height, info, and latest block header:

```tsx
// src/hooks/useCereraDashboard.ts
import { useEffect, useState } from "react";
import { createCereraClient, type ChainInfo, type BlockHeader } from "../lib/cerera-rpc";

const rpc = createCereraClient(import.meta.env.VITE_CERERA_RPC_URL);

export function useCereraDashboard(pollMs = 5000) {
  const [height, setHeight] = useState(0);
  const [info, setInfo] = useState<ChainInfo | null>(null);
  const [head, setHead] = useState<BlockHeader | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function refresh() {
      try {
        const h = await rpc.chain.height();
        const i = await rpc.chain.getInfo();
        const block = h > 0 ? await rpc.chain.getBlockByIndex(h) : null;
        if (cancelled) return;
        setHeight(h);
        setInfo(i);
        setHead(block?.header ?? null);
        setError(null);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      }
    }

    refresh();
    const t = setInterval(refresh, pollMs);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [pollMs]);

  return { height, info, head, error };
}
```

**Display helpers:**

```typescript
// avgTime: seconds → human label
export function formatAvgBlockTime(sec?: number) {
  if (!sec || sec <= 0) return "—";
  if (sec < 60) return `${sec.toFixed(1)} s`;
  return `${(sec / 60).toFixed(1)} min`;
}

// block header timestamp is milliseconds
export function formatBlockTime(tsMs: number) {
  return new Date(tsMs).toLocaleString();
}
```

### Vite dev proxy (optional)

Avoid CORS and hard-coded host in development:

```typescript
// vite.config.ts
export default {
  server: {
    proxy: {
      "/api/rpc": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/rpc/, "/"),
      },
    },
  },
};
```

Then set `VITE_CERERA_RPC_URL=/api/rpc`.

### Recommended UI flows

| Screen | RPC calls | Notes |
|--------|-----------|-------|
| Network status | `chain.height`, `chain.getInfo`, `pool.getInfo` | Poll every 3–10 s |
| Block list | `chain.getBlockByIndex` × N | Walk from `height` down; throttle requests |
| Block detail | `chain.getBlock` or `getBlockByIndex` | Show `timestamp` as ms |
| Wallet balance | `account.getBalance` | Balance is `float64` CER — use decimal lib for UI |
| Send tx | `transaction.send` | Prefer DTO param; `amount` as string `"1.5"` |
| Faucet (testnet) | `account.faucet` | Amount as string |

### Error handling checklist

1. **HTTP layer:** non-2xx → read `response.text()` (often plain Go error string).
2. **JSON-RPC layer:** check `response.error` (`code`, `message`).
3. **Business errors:** some handlers return error strings inside `result` (legacy) — treat non-object `result` after mutating calls carefully.
4. **Timeouts:** client-side `AbortSignal.timeout(8000)` recommended (server limit is 5 s).
5. **Retries:** safe for read-only methods (`height`, `getInfo`, `getBalance`); avoid blind retries on `send`.

```typescript
const res = await fetch(url, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
  signal: AbortSignal.timeout(8000),
});
```

### JSON field quirks (important for UI)

| Field | Service | Type | UI note |
|-------|---------|------|---------|
| `avgTime` | chain.getInfo | seconds | Not ms; 0 until ≥2 mined blocks with one interval |
| `timestamp` | block header | Unix **ms** | `new Date(ts)` — do not multiply by 1000 |
| Balances | account.* | float64 CER | Use `decimal.js` / `big.js` for amounts in UI |
| `amount` (send) | transaction.send | string | `"10.5"`, not JS float |
| `latest` | chain.getInfo | hex hash | Full `0x…` string |
| `gas` | pool.minGas | float64 | Minimum gas price hint |

### Security notes for frontend

- **`create` / `restore` return `priv` and `mnemonic`** — never log them, never send to analytics, avoid `localStorage` unless encrypted wallet vault.
- **`transaction.send` with `key`** — in production, signing should happen server-side or in a dedicated wallet; the RPC `key` field expects public key material for the current validator API shape.
- Node RPC has **no auth** by default — bind to localhost in production or put an API gateway in front.

### Minimal fetch example (no framework)

```javascript
async function cereraCall(method, params = []) {
  const res = await fetch("http://127.0.0.1:8080/", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", method, params, id: 1 }),
  });
  if (!res.ok) throw new Error(await res.text());
  const { result, error } = await res.json();
  if (error) throw new Error(error.message);
  return result;
}

const [height, info] = await Promise.all([
  cereraCall("cerera.chain.height"),
  cereraCall("cerera.chain.getInfo"),
]);
console.log({ height, avgBlockSec: info.avgTime ?? 0, latest: info.latest });
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

