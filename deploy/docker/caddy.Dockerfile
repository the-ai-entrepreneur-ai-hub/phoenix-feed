FROM caddy:2-builder-alpine AS builder

RUN xcaddy build --with github.com/mholt/caddy-ratelimit@v0.1.0

FROM caddy:2-alpine

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY deploy/caddy-entrypoint.sh /usr/local/bin/caddy-entrypoint.sh
RUN chmod 0755 /usr/local/bin/caddy-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/caddy-entrypoint.sh"]
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
