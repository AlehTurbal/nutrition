#!/bin/sh
set -eu

# Railway provides PORT and (via a reference variable) BACKEND_URL.
: "${PORT:?PORT is required}"
: "${BACKEND_URL:?BACKEND_URL is required}"

# Derive the DNS resolver nginx should use for runtime upstream resolution.
RESOLVER="$(awk '/^nameserver/ { print $2; exit }' /etc/resolv.conf)"
RESOLVER="${RESOLVER:-127.0.0.11}"
export PORT BACKEND_URL RESOLVER

# Substitute ONLY our three placeholders; leave nginx $variables intact.
envsubst '${PORT} ${BACKEND_URL} ${RESOLVER}' \
  < /etc/nginx/nginx.conf.template \
  > /etc/nginx/conf.d/default.conf

# Validate the rendered config, then run.
nginx -t
exec nginx -g 'daemon off;'
