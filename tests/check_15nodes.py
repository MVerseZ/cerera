#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Smoke check for deployments/docker-compose-15nodes.yml."""

from check_nodes_common import configure_utf8_stdout, run_basic_cluster_check

COMPOSE_FILE = "docker-compose-15nodes.yml"
PORTS = list(range(1337, 1352))  # 1337-1351
NODES = [f"node{i}" for i in range(1, 16)]


def main() -> None:
    configure_utf8_stdout()
    run_basic_cluster_check(COMPOSE_FILE, NODES, PORTS)


if __name__ == "__main__":
    main()
