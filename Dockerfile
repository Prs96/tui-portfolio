# Stage 1: Build the binary
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o portfolio main.go

# Stage 2: Create a hardened, minimal runtime container
FROM alpine:3.19
RUN apk --no-cache add ca-certificates

# Create a non-root user for security
RUN adduser -D -g '' appuser
WORKDIR /home/appuser

# Copy binary from builder
COPY --from=builder /app/portfolio .

# Create directory for persistent SSH host keys
RUN mkdir -p .ssh && chown -R appuser:appuser .ssh
USER appuser

# Force truecolor so lipgloss renders hex colors correctly over SSH
ENV TERM=xterm-256color
ENV COLORTERM=truecolor

# Expose the internal port defined in main.go
EXPOSE 23234
CMD ["./portfolio"]

