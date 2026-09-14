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

## Fork testing (devnet)

Cerera detects competing tips and prev-hash mismatches, stores orphan blocks, and switches to the branch with **higher total difficulty** (tie-break: height, then head hash lexicographic).

### Manual two-node fork

1. Start two nodes **without** connecting them (separate compose stacks or isolated networks).
2. Mine several blocks on each node so they diverge (different head hash at the same or different height).
3. Connect the nodes (shared bootstrap / peer add).
4. Watch logs for:
   - `competing block at tip` or `prev hash mismatch — fork candidate`
   - `orphan stored`
   - `heavier branch detected — reorg` (when the alternate branch is complete and heavier)
5. After convergence, run the usual check script — **heights and head hash should match** on all nodes.

### Metrics

Prometheus counters/gauges (when metrics endpoint is enabled):

- `icenet_fork_detected_total{reason="competing_tip|prev_hash_mismatch"}`
- `icenet_orphan_blocks`
- `icenet_reorg_total`

### Optional state-root check

Cross-node `header.stateRoot` consistency can be checked with:

```bash
PYTHONIOENCODING=utf-8 python tests/check_nodes_common.py --check-state-root
```

Use this after a fork/reorg scenario to confirm vault replay converged.
