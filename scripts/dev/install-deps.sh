#!/usr/bin/env bash
# Install all runtime and install time deps here
# ! DO NOT install dependencies in install-*.sh scripts other than this

set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -qq
apt-get install -y \
    traceroute \
    iputils-ping \
    net-tools \
    tcpdump \
    iperf3 \
    dnsutils \
    iproute2 \
    curl \
    mtr-tiny \
    jq \
    gnupg \
    lsb-release \
    make
