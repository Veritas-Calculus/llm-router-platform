# Build stage
FROM node:25-alpine AS builder

WORKDIR /app

# Copy package files and npm config
COPY package*.json .npmrc ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build
RUN npm run build

# Runtime stage
FROM nginx:alpine

LABEL org.opencontainers.image.source="https://github.com/Veritas-Calculus/llm-router-platform"
LABEL org.opencontainers.image.description="LLM Router Platform - Web Frontend"
LABEL org.opencontainers.image.licenses="MIT"

# Patch Alpine security updates
RUN apk upgrade --no-cache

# Create non-privileged user
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -h /app -s /sbin/nologin

# Copy full nginx config (handles non-root paths internally)
COPY nginx.conf /etc/nginx/nginx.conf
# Remove default server block config (ours is in the main nginx.conf)
RUN rm -f /etc/nginx/conf.d/default.conf

# Copy built frontend files
COPY --from=builder --chown=appuser:appgroup /app/dist /usr/share/nginx/html

# Switch to non-privileged user
USER appuser

# Non-privileged port
EXPOSE 8080

ENTRYPOINT []
CMD ["nginx", "-g", "daemon off;"]
