#!/usr/bin/env python3
"""Add ICE_DEV_VALIDATORS=1 to Cerera node services in compose files (idempotent)."""

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "deployments"


def dedupe_ice_env(text: str) -> str:
    return re.sub(
        r"(      - ICE_DEV_VALIDATORS=1\n)+",
        "      - ICE_DEV_VALIDATORS=1\n",
        text,
    )


def add_ice_env_to_seed_files(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "ICE_DEV_VALIDATORS=1" in text:
        new = dedupe_ice_env(text)
    else:
        new = re.sub(
            r"(      - SEED_NODES=[^\n]+)\n",
            r"\1\n      - ICE_DEV_VALIDATORS=1\n",
            text,
        )
    if new != text:
        path.write_text(new, encoding="utf-8", newline="\n")
        print(f"updated SEED: {path.name}")


def add_ice_env_to_miner_nodes(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "ICE_DEV_VALIDATORS=1" in text:
        new = dedupe_ice_env(text)
    else:
        new = re.sub(
            r'(    command: \["cerera",[^\n]*"--miner"\])\n(    ports:)',
            r"\1\n    environment:\n      - ICE_DEV_VALIDATORS=1\n\2",
            text,
        )
    if new != text:
        path.write_text(new, encoding="utf-8", newline="\n")
        print(f"updated miner env: {path.name}")


def main() -> None:
    for name in [
        "docker-compose-3nodes.yml",
        "docker-compose-5nodes.yml",
        "docker-compose-9nodes.yml",
        "docker-compose-15nodes.yml",
        "docker_rem9_observer_9nodes.yml",
    ]:
        add_ice_env_to_seed_files(ROOT / name)

    for name in [
        "docker-compose-full.yml",
        "docker-compose-single.yml",
        "docker-compose-public.yml",
        "docker-compose-nodes.yml",
        "docker-compose-15nodes.yml",
    ]:
        path = ROOT / name
        if path.exists():
            add_ice_env_to_miner_nodes(path)


if __name__ == "__main__":
    main()
