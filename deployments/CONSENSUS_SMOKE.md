# Consensus smoke test (multi-node)

This repo finalizes blocks via a simple **PoW proposer + BFT vote** flow:

- proposer broadcasts `PrePrepare` including the full block payload
- validators broadcast `Prepare`/`Commit` votes (signed)
- on `Commit` quorum, the block is **added to the chain** (`chain.AddBlock`)

All multi-node compose files under `deployments/` set `ICE_DEV_VALIDATORS=1` so connected peers are treated as validators in dev mode.

## Compose files and check scripts

| Compose file | Nodes | Check script |
|---|---|---|
| `docker-compose-single.yml` | 1 | `tests/check_1node.py` |
| `docker-compose-3nodes.yml` | 3 | `tests/check_3nodes.py` |
| `docker-compose-5nodes.yml` | 5 | `tests/check_5nodes.py` (+ chain integrity) |
| `docker-compose-9nodes.yml` | 9 | `tests/check_9nodes.py` |
| `docker-compose-15nodes.yml` | 15 | `tests/check_15nodes.py` |
| `docker-compose-nodes.yml` | 9 | `tests/check_9nodes.py` (same ports) |
| `docker-compose-full.yml` | 5 | `tests/check_5nodes.py` |
| `docker-compose-public.yml` | 1 | `tests/check_1node.py` |
| `docker_rem9_observer_9nodes.yml` | 9 | `tests/check_9nodes.py` |

## Run (example: 3 nodes)

1. Build + start:

```bash
docker compose -f deployments/docker-compose-3nodes.yml up --build
```

2. Wait a few minutes for mining and sync, then verify:

```bash
PYTHONIOENCODING=utf-8 python tests/check_3nodes.py
```

3. Expected success criteria:

- all nodes respond on RPC
- **heights match** across nodes
- **head block hash matches** across nodes (chain sync OK)
- `cerera.account.getAll` may differ per node (each node has its own address) — this is informational only

4. Verify in logs you see something like:

- `Voting manager started`
- `Started consensus round`
- `Commit quorum reached - block finalized`
- `Block successfully added to chain`

## Other stacks

```bash
# 5 nodes (+ full chain integrity walk)
docker compose -f deployments/docker-compose-5nodes.yml up --build
PYTHONIOENCODING=utf-8 python tests/check_5nodes.py

# 9 nodes
docker compose -f deployments/docker-compose-9nodes.yml up --build
PYTHONIOENCODING=utf-8 python tests/check_9nodes.py

# 15 nodes
docker compose -f deployments/docker-compose-15nodes.yml up --build
PYTHONIOENCODING=utf-8 python tests/check_15nodes.py

# single node
docker compose -f deployments/docker-compose-single.yml up --build
PYTHONIOENCODING=utf-8 python tests/check_1node.py
```

## Notes

- To disable dev validator behavior (connected peers == validators), set `ICE_DEV_VALIDATORS=0`.
- Quorum is computed from the **validator snapshot at round start**, so peers connecting/disconnecting mid-round won't change the required quorum.
- To (re)apply `ICE_DEV_VALIDATORS=1` across compose files idempotently: `python tests/update_compose_env.py`
