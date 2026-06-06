ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY services/stamp-processor ./services/stamp-processor/
COPY libs/common ./libs/common/
COPY libs/telemetry/ ./libs/telemetry/

RUN CGO_ENABLED=0 go build -o /bin/cepheus-stamp-processor ./services/stamp-processor/cmd

FROM alpine:3.21

COPY --from=build /bin/cepheus-stamp-processor /usr/local/bin/cepheus-stamp-processor

WORKDIR /app

ENTRYPOINT [ "cepheus-stamp-processor" ]
