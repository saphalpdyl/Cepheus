#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -qq
apt-get install -y ca-certificates curl gnupg lsb-release

# Containerlab
curl -sL https://containerlab.dev/setup | bash -s "all"