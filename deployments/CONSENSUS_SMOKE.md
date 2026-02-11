# Consensus smoke test (3–5 nodes)

This repo now finalizes blocks via a simple **PoW proposer + BFT vote** flow:

- proposer broadcasts `PrePrepare` including the full block payload
- validators broadcast `Prepare`/`Commit` votes (signed)
- on `Commit` quorum, the block is **added to the chain** (`chain.AddBlock`)

## Run (3 nodes)

1. Build + start:

```bash
docker compose -f ci-cd/docker-compose-3nodes.yml up --build
```

2. Ensure validators are auto-managed (dev-mode):

- default is **enabled**
- you can force-enable by setting `ICE_DEV_VALIDATORS=1` in the compose env

3. Trigger a block proposal.

This depends on how you run mining/proposal in your node (e.g. an internal miner loop, RPC, or “autogen blocks” mode). Once a node proposes, the other nodes should vote.

4. Verify in logs you see something like:

- `Voting manager started`
- `Started consensus round`
- `Commit quorum reached - block finalized`
- `Block finalized by consensus`

## Notes

- To disable dev validator behavior (connected peers == validators), set `ICE_DEV_VALIDATORS=0`.
- Quorum is computed from the **validator snapshot at round start**, so peers connecting/disconnecting mid-round won’t change the required quorum.

