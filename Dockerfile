FROM golang:1.25-alpine3.23 AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/api

# Runtime stage
FROM alpine:3.23

RUN apk --no-cache add ca-certificates

COPY --from=builder /gateway /gateway

EXPOSE 8080

CMD ["/gateway"]
