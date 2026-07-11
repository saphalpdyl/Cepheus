ARG GO_VERSION=1.25.12

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY services/server/ ./services/server/
COPY cepheus-server.config.yaml ./cepheus-server.config.yaml
COPY libs/api/ ./libs/api/
COPY libs/common/ ./libs/common/
COPY libs/telemetry/ ./libs/telemetry/

RUN CGO_ENABLED=0 go build -o /bin/server ./services/server/cmd

FROM alpine:3.21

COPY --from=build /bin/server /usr/local/bin/cepheus-server

WORKDIR /app
COPY --from=build /src/cepheus-server.config.yaml /app/cepheus-server.config.yaml

EXPOSE 8080

ENTRYPOINT ["cepheus-server"]
