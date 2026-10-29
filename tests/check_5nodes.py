#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Smoke check for deployments/docker-compose-5nodes.yml with full chain integrity validation.
"""

from check_nodes_common import configure_utf8_stdout, run_cluster_smoke_check

COMPOSE_FILE = "docker-compose-5nodes.yml"
PORTS = [1337, 1338, 1339, 1340, 1341]
NODES = ["node1", "node2", "node3", "node4", "node5"]


def main() -> None:
    configure_utf8_stdout()
    run_cluster_smoke_check(
        COMPOSE_FILE,
        NODES,
        PORTS,
        chain_integrity=True,
        subtitle="и целостности цепочки",
    )


if __name__ == "__main__":
    main()
