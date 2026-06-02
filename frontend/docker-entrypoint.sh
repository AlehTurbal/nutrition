#!/bin/sh
set -eu

# Railway provides PORT and (via a reference variable) BACKEND_URL.
: "${PORT:?PORT is required}"
: "${BACKEND_URL:?BACKEND_URL is required}"

# Derive the DNS resolver nginx should use for runtime upstream resolution.
# Railway's internal DNS is IPv6 (e.g. fd12::10); nginx requires IPv6 resolver
# addresses to be wrapped in brackets, otherwise it misreads the trailing
# hextet as a port ("invalid port in resolver").
RESOLVER="$(awk '/^nameserver/ { print $2; exit }' /etc/resolv.conf)"
RESOLVER="${RESOLVER:-127.0.0.11}"
case "$RESOLVER" in
  *:*) RESOLVER="[$RESOLVER]" ;;
esac
export PORT BACKEND_URL RESOLVER

# Substitute ONLY our three placeholders; leave nginx $variables intact.
envsubst '${PORT} ${BACKEND_URL} ${RESOLVER}' \
  < /etc/nginx/nginx.conf.template \
  > /etc/nginx/conf.d/default.conf

# Validate the rendered config, then run.
nginx -t
exec nginx -g 'daemon off;'
