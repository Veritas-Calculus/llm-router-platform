#!/bin/sh
# Web container entrypoint.
#
# At container start we materialize the runtime-config.js consumed by the
# SPA by running envsubst over the build-time template. This lets the
# same production image be deployed to dev / staging / prod without
# rebuild — Sentry DSN, captcha keys, etc. all come from container env
# vars. See audit finding H-08.
#
# The web container runs with `read_only: true` in docker-compose, so we
# cannot write into /usr/share/nginx/html at runtime. Instead we write to
# /tmp (mounted as tmpfs) and nginx serves /runtime-config.js from there
# via an `alias` directive — see web/nginx.conf.
#
# The template uses literal ${VAR} placeholders. envsubst replaces only the
# variables we explicitly whitelist; unrelated `${...}` patterns in the file
# (none today, but a safety net) are left untouched.

set -eu

TEMPLATE=/usr/share/nginx/html/runtime-config.template.js
OUTPUT=/tmp/runtime-config.js

if [ -f "$TEMPLATE" ]; then
    envsubst '${VITE_SENTRY_DSN} ${VITE_SENTRY_ENVIRONMENT} ${VITE_CAPTCHA_PROVIDER} ${VITE_CAPTCHA_SITE_KEY} ${VITE_DEV_CAPTCHA_BYPASS_TOKEN}' \
        < "$TEMPLATE" > "$OUTPUT"
    echo "[entrypoint] runtime-config.js generated at $OUTPUT"
else
    echo "[entrypoint] WARN: $TEMPLATE not found — frontend will fall back to build-time values"
fi

# Exec the original command (nginx).
exec "$@"
