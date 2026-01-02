# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates (needed for downloading modules and HTTPS)
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ai-reviewer .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and git for GitHub operations
RUN apk --no-cache add ca-certificates git

# Create non-root user
RUN addgroup -g 1000 reviewer && adduser -D -u 1000 -G reviewer reviewer

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/ai-reviewer .

# Change ownership to non-root user
RUN chown reviewer:reviewer /app/ai-reviewer

# Switch to non-root user
USER reviewer

# Set entrypoint
ENTRYPOINT ["./ai-reviewer"]