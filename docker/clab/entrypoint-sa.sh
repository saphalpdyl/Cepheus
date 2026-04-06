#!/bin/sh
set -e

# Start FRR daemons
/usr/lib/frr/watchfrr -d zebra ospfd

# Hand off to probe-agent
exec probe-agent
