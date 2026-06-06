ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY services/argus/ ./services/argus/
COPY libs/common/ ./libs/common/
COPY libs/telemetry/ ./libs/telemetry/

RUN CGO_ENABLED=0 go build -o /bin/cepheus-argus ./services/argus/cmd

FROM alpine:3.21

COPY --from=build /bin/cepheus-argus /usr/local/bin/cepheus-argus

WORKDIR /app

ENTRYPOINT [ "cepheus-argus" ]
