#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

# Containerlab
curl -sL https://containerlab.dev/setup | bash -s "all"