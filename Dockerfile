# Speed Reader - Multi-stage Docker build with Go backend
# Stage 1: Build frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app

# Copy package files and install dependencies
COPY package*.json ./
RUN npm ci

# Copy source and build with Babel
COPY .babelrc ./
COPY src ./src
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.23-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy Go module files
COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download

# Copy source code
COPY backend-go/ ./

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o server ./cmd/server

# Stage 3: Final minimal image
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS (if needed)
RUN apk add --no-cache ca-certificates

# Copy the Go binary
COPY --from=backend-builder /app/server ./

# Copy frontend files
COPY index.html reader.html styles.css polyfills.js ./
COPY --from=frontend-builder /app/dist ./dist

# Create data directory for SQLite
RUN mkdir -p /app/data

# Set environment variables
ENV PORT=8000
ENV DATABASE_PATH=/app/data/books.db
ENV STATIC_DIR=/app

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health || exit 1

# Run the server
CMD ["./server", "-port", "8000", "-static", "/app", "-db", "/app/data/books.db"]
