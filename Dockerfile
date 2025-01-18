# Build stage
FROM --platform=$BUILDPLATFORM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy go mod files
COPY go.* ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /hurl-service

# Final stage
FROM debian:bullseye-slim

# Install Hurl and its dependencies
RUN apt-get -y update; apt-get -y install curl
RUN curl --location --remote-name https://github.com/Orange-OpenSource/hurl/releases/download/6.0.0/hurl_6.0.0_amd64.deb
RUN apt update -y && apt install ./hurl_6.0.0_amd64.deb -y

# Create directory for results
RUN mkdir /hurl_results && \
    chmod 755 /hurl_results

# Copy binary from builder
COPY --from=builder /hurl-service /usr/local/bin/

# Set environment variables
ENV PORT=8080

# Create non-root user
RUN useradd -r -u 1000 -U app && \
    chown app:app /hurl_results

# Switch to non-root user
USER app

# Expose port
EXPOSE 8080

# Run the service
CMD ["/usr/local/bin/hurl-service"]