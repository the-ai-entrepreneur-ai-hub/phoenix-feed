FROM caddy:2-alpine

COPY deploy/caddy-entrypoint.sh /usr/local/bin/caddy-entrypoint.sh
RUN chmod 0755 /usr/local/bin/caddy-entrypoint.sh

ENTRYPOINT ["/usr/local/bin/caddy-entrypoint.sh"]
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
