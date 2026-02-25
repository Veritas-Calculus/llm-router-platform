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

# Patch CVE-2026-25646 and other Alpine security updates
RUN apk upgrade --no-cache

# Copy built files
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
