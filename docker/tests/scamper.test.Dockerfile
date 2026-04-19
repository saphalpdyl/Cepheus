FROM golang:1.25.4-alpine

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

CMD ["sleep", "infinity"]