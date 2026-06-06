ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY services/agent/ ./services/agent/
COPY libs/common/ ./libs/common/
COPY cepheus-agent.config.yaml ./cepheus-agent.config.yaml
COPY api/ ./api/
COPY libs/scamper-client/ ./libs/scamper-client/
COPY libs/stamp/ ./libs/stamp/
COPY libs/telemetry/ ./libs/telemetry/

RUN CGO_ENABLED=0 go build -o /bin/agent ./services/agent/cmd

FROM scratch
COPY --from=build /bin/agent /cepheus-agent/cepheus-agent
COPY cepheus-agent.config.yaml /cepheus-agent/cepheus-agent.config.yaml
