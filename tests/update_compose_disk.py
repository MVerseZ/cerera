#!/usr/bin/env python3
"""Switch compose stacks to disk persistence (--data-dir=/app/data)."""

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "deployments"

DISK_ARGS = '"--data-dir=/app/data", "--key=/app/config/ddddd.nodekey.pem", "--miner"'

PATTERN = re.compile(
    r'(\["cerera",[^\]]*?)("--mem=true",\s*"--miner"\])'
)


def update_compose(path: Path) -> None:
    text = path.read_text(encoding="utf-8")
    if "--data-dir=/app/data" in text:
        return
    new, count = PATTERN.subn(rf"\1{DISK_ARGS}]", text)
    if count:
        path.write_text(new, encoding="utf-8", newline="\n")
        print(f"updated disk persistence: {path.name} ({count} services)")


def main() -> None:
    for path in sorted(ROOT.glob("docker-compose*.yml")):
        update_compose(path)
    rem9 = ROOT / "docker_rem9_observer_9nodes.yml"
    if rem9.exists():
        update_compose(rem9)


if __name__ == "__main__":
    main()
