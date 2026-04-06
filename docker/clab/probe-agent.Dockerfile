ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/probe-agent ./cmd/probe-agent

FROM alpine:3.21

RUN apk add --no-cache \
    iputils \
    mtr \
    traceroute \
    busybox-extras

COPY --from=build /bin/probe-agent /usr/local/bin/probe-agent

ENV MODE=active

ENTRYPOINT ["probe-agent"]
