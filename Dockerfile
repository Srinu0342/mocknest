# Build stage
FROM golang:1.25-alpine AS builder

# Set working directory inside container
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod ./

# Download dependencies (none for standard library, but good practice)
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the Go binary
RUN go build -o server ./server

# Final stage: minimal image
FROM alpine:latest

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/server .

# Expose port (your Go app listens on 8080)
EXPOSE 8080

# Run the binary
CMD ["./server"]
