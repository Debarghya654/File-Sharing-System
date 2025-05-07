# Step 1: Build the Go application
FROM golang:1.20 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod tidy

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o file-sharing-system .

# Step 2: Create a small image for the app to run in
FROM alpine:latest

# Install necessary libraries (e.g., SSL certificates for HTTPS)
RUN apk --no-cache add ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/file-sharing-system .

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./file-sharing-system"]
