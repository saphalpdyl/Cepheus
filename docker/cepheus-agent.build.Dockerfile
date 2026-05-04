ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY agent/ ./agent
COPY common/ ./common/
COPY cepheus-agent.config.yaml ./cepheus-agent.config.yaml
COPY api/ ./api/
COPY scamper-client/ ./scamper-client/
COPY stamp/ ./stamp/
COPY telemetry/ ./telemetry/
COPY cmd/agent/ ./cmd/agent/

RUN CGO_ENABLED=0 go build -o /bin/agent ./cmd/agent

FROM scratch
COPY --from=build /bin/agent /cepheus-agent/cepheus-agent
COPY cepheus-agent.config.yaml /cepheus-agent/cepheus-agent.config.yaml
