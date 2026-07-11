ARG GO_VERSION=1.25.12

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY services/ping-processor ./services/ping-processor/
COPY libs/common ./libs/common/
COPY libs/telemetry/ ./libs/telemetry/

RUN CGO_ENABLED=0 go build -o /bin/cepheus-ping-processor ./services/ping-processor/cmd

FROM alpine:3.21

COPY --from=build /bin/cepheus-ping-processor /usr/local/bin/cepheus-ping-processor

WORKDIR /app

ENTRYPOINT [ "cepheus-ping-processor" ]
