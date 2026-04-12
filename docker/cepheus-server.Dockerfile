ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/cepheus-server ./cmd/control-plane

FROM alpine:3.21

COPY --from=build /bin/cepheus-server /usr/local/bin/cepheus-server

WORKDIR /app
COPY --from=build /src/cepheus-server.config.yaml /app/cepheus-server.config.yaml

EXPOSE 8080

ENTRYPOINT ["cepheus-server"]
