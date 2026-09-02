# Build stage
FROM --platform=$BUILDPLATFORM node:24-alpine AS builder

WORKDIR /app

# Copy package files and npm config
COPY web/package*.json web/.npmrc ./

# Install dependencies
RUN npm ci

ARG CACHEBUST=1

# Sentry and captcha settings are injected by the runtime entrypoint. Keep
# them out of the build stage so secrets cannot be baked into image layers or
# the Vite bundle, and so one image can be promoted across environments.

# Copy source code and the GraphQL schema used by the build-time validator
COPY web/ .
COPY server/internal/graphql/schema /server/internal/graphql/schema

# Build
RUN npm run build

# Runtime stage
FROM nginx:alpine

LABEL org.opencontainers.image.source="https://github.com/Veritas-Calculus/llm-router-platform"
LABEL org.opencontainers.image.description="LLM Router Platform - Web Frontend"
LABEL org.opencontainers.image.licenses="MIT"

# Patch Alpine security updates and install gettext for envsubst (used by
# docker-entrypoint.sh to materialize /runtime-config.js at container start —
# audit H-08).
RUN apk upgrade --no-cache && apk add --no-cache gettext

# Create non-privileged user
RUN addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -h /app -s /sbin/nologin

# Copy full nginx config (handles non-root paths internally) plus the
# security-headers snippet that every location `include`s — see
# web/snippets/security-headers.conf for the rationale (single source of
# truth for CSP/HSTS/Frame-Options to avoid the duplicate-header drift
# tracked as audit finding H-01).
COPY web/nginx.conf /etc/nginx/nginx.conf
COPY web/snippets/ /etc/nginx/snippets/
# Entrypoint that runs envsubst over the runtime-config template, then
# execs nginx. See web/docker-entrypoint.sh and audit H-08.
COPY web/docker-entrypoint.sh /docker-entrypoint-runtime-config.sh
RUN chmod +x /docker-entrypoint-runtime-config.sh
# Remove default server block config (ours is in the main nginx.conf)
RUN rm -f /etc/nginx/conf.d/default.conf

# Copy built frontend files (includes the runtime-config.template.js the
# entrypoint expands at startup).
COPY --from=builder --chown=appuser:appgroup /app/dist /usr/share/nginx/html

# Switch to non-privileged user
USER appuser

# Non-privileged port
EXPOSE 8080

ENTRYPOINT ["/docker-entrypoint-runtime-config.sh"]
CMD ["nginx", "-g", "daemon off;"]
