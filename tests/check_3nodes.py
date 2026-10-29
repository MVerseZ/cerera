#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Smoke check for deployments/docker-compose-3nodes.yml."""

from check_nodes_common import configure_utf8_stdout, run_basic_cluster_check

COMPOSE_FILE = "docker-compose-3nodes.yml"
PORTS = [1337, 1338, 1339]
NODES = ["node1", "node2", "node3"]


def main() -> None:
    configure_utf8_stdout()
    run_basic_cluster_check(COMPOSE_FILE, NODES, PORTS)


if __name__ == "__main__":
    main()
