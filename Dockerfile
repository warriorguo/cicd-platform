# =============================================================================
# CI/CD Platform - Combined Dockerfile
# Builds frontend and backend into a single container with nginx
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Build Frontend
# -----------------------------------------------------------------------------
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files and install dependencies
COPY frontend/package*.json ./
RUN npm ci

# Copy source and build
COPY frontend/ ./
RUN npm run build

# -----------------------------------------------------------------------------
# Stage 2: Build Backend
# -----------------------------------------------------------------------------
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY backend/ ./backend/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cicd-platform ./backend/cmd/main.go

# -----------------------------------------------------------------------------
# Stage 3: Final Production Image with Nginx
# -----------------------------------------------------------------------------
FROM alpine:latest

# Install required packages
RUN apk --no-cache add ca-certificates tzdata nginx supervisor

# Create directories
RUN mkdir -p /run/nginx /var/log/supervisor /app

WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /app/cicd-platform .

# Copy frontend build to nginx html directory
COPY --from=frontend-builder /app/frontend/build /usr/share/nginx/html

# Copy nginx configuration
COPY docker/nginx.conf /etc/nginx/nginx.conf

# Copy supervisord configuration
COPY docker/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Copy default config (optional)
COPY backend/config.yaml ./config.yaml 2>/dev/null || true

# Environment variables
ENV SERVER_HOST=127.0.0.1
ENV SERVER_PORT=8080
ENV GIN_MODE=release

# Expose port (nginx)
EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost/health || exit 1

# Run supervisord to manage both nginx and backend
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
