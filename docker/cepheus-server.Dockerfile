ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY server/ ./server/
COPY api/ ./api/
COPY cepheus-server.config.yaml ./cepheus-server.config.yaml
COPY common/ ./common/
COPY telemetry/ ./telemetry/
COPY cmd/server ./cmd/server/


RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server

FROM alpine:3.21

COPY --from=build /bin/server /usr/local/bin/cepheus-server

WORKDIR /app
COPY --from=build /src/cepheus-server.config.yaml /app/cepheus-server.config.yaml

EXPOSE 8080

ENTRYPOINT ["cepheus-server"]
