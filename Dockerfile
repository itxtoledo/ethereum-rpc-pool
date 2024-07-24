# Use the official Golang image as a builder
FROM golang:1.18 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN go build -o app .

# Use a minimal base image to run the application
FROM alpine:latest

# Install necessary CA certificates
RUN apk --no-cache add ca-certificates

# Set the working directory inside the container
WORKDIR /root/

# Copy the built Go application from the builder stage
COPY --from=builder /app/app .

# Command to run the application
CMD ["./app"]
