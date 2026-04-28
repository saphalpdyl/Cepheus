ARG GO_VERSION=1.25.4

FROM golang:${GO_VERSION}-alpine AS build

RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/cepheus-trace-processor ./cmd/trace-processor

FROM alpine:3.21

COPY --from=build /bin/cepheus-trace-processor /usr/local/bin/cepheus-trace-processor

WORKDIR /app

ENTRYPOINT [ "cepheus-trace-processor" ]