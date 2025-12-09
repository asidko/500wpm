# Speed Reader - Multi-stage Docker build
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

# Stage 2: Final image with Python backend + static frontend
FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

# Copy backend requirements and install Python dependencies
COPY backend/requirements.txt ./backend/
RUN pip install --no-cache-dir -r backend/requirements.txt

# Copy backend code
COPY backend ./backend

# Copy frontend files
COPY index.html reader.html styles.css polyfills.js ./
COPY --from=frontend-builder /app/dist ./dist

# Expose ports (3000 for frontend, 8000 for backend)
EXPOSE 3000 8000

# Create startup script
RUN echo '#!/bin/sh\n\
cd /app/backend && python main.py &\n\
cd /app && python -m http.server 3000\n\
' > /app/start.sh && chmod +x /app/start.sh

# Run both servers
CMD ["/app/start.sh"]
