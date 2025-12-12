# --------------------
# Build Stage
# --------------------
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Install required tools
RUN apk add --no-cache git

# Copy go.mod and go.sum first for caching
COPY go.mod go.sum ./
RUN go mod download

# Now copy the project
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o coupon-service ./cmd/coupon-service

# --------------------
# Runtime Stage
# --------------------
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/coupon-service .

EXPOSE 8080

ENTRYPOINT ["./coupon-service"]
