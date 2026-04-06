#!/usr/bin/env bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

source /tmp/version.env
bash -c "$(curl -sL https://get.containerlab.dev)" -- -v $CONTAINERLAB_VERSION