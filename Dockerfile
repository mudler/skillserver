# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o skillserver ./cmd/skillserver

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates git

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/skillserver .

# Create skills directory
RUN mkdir -p /app/skills

# Expose web server port
EXPOSE 8080

# Run the application
ENTRYPOINT ["./skillserver"]
CMD ["--dir", "/app/skills", "--port", "8080"]
