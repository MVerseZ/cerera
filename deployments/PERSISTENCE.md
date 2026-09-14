# Cerera disk persistence and backup

Cerera stores node state on disk when started with `--data-dir` (or `CERERA_DATA_DIR`).

## Data directory layout

```
/app/data/                 # --data-dir (Docker volume: nodeN_data)
  config.json              # chain/vault paths, network settings
  chain.dat                # newline-delimited JSON blocks
  vault/                   # pogreb account database
    .vault_keys            # encryption keys (required for restore)
  logfile                  # node log (when --data-dir is set)

/app/config/               # Docker volume: nodeN_config
  ddddd.nodekey.pem        # node identity key (--key)
```

Start the node in disk mode:

```bash
cerera --mode=p2p --http=1337 --data-dir=./data --key=./ddddd.nodekey.pem --miner
```

Docker Compose stacks under `deployments/` use `--data-dir=/app/data` and mount `nodeN_data:/app/data`.

## Backup

Stop the node first (or backup from a quiesced replica). Then:

```bash
cereractl backup \
  --data-dir /app/data \
  --node-key /app/config/ddddd.nodekey.pem \
  --output cerera-node1-$(date +%Y%m%d).tar.gz
```

The archive contains:

- `config.json`
- `chain.dat`
- `vault/` (including `.vault_keys`)
- optional `nodekey.pem`
- `cerera-backup-manifest.json`

Inspect an archive without extracting:

```bash
cereractl inspect --input cerera-node1-20260101.tar.gz
```

## Restore

Stop the node, then:

```bash
cereractl restore --data-dir /app/data --input cerera-node1-20260101.tar.gz
```

To overwrite an existing data directory:

```bash
cereractl restore --data-dir /app/data --input backup.tar.gz --force
```

Existing files are moved to `.restore-backup-<timestamp>/` before extraction.

After restore, start the node with the same `--data-dir` and `--key` paths.

## Notes

- Restoring `vault/` without matching `.vault_keys` (or `VAULT_MNEMONIC` / `VAULT_PASSPHRASE`) will fail to decrypt accounts.
- Chain and vault are backed up together but not atomically; stop the node for a consistent snapshot.
- In-memory mode (`--mem=true`) skips disk writes; use disk mode for production-like deployments.
