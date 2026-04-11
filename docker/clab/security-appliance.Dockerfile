ARG BASE=quay.io/frrouting/frr:10.5.1

FROM ${BASE}

RUN apk add --no-cache iptables
