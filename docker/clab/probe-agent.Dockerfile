ARG GO_VERSION=1.25.4
ARG BASE=alpine:3.21

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/probe-agent ./cmd/probe-agent

FROM ${BASE}

RUN apk add --no-cache \
    iputils \
    mtr \
    traceroute \
    busybox-extras

COPY --from=build /bin/probe-agent /usr/local/bin/probe-agent
COPY docker/clab/entrypoint-sa.sh /usr/local/bin/entrypoint-sa.sh

ENV MODE=active

ENTRYPOINT ["probe-agent"]
