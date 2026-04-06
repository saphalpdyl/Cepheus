#!/bin/sh
set -e

# Start FRR daemons
/usr/lib/frr/watchfrr -d zebra bgpd

# Hand off to probe-agent
exec probe-agent
