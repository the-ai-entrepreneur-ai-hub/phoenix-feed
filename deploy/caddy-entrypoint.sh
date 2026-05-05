#!/bin/sh
set -eu

SITE_SNIPPET=/tmp/phoenix-feed-site.caddy
DOMAIN_VALUE=${DOMAIN:-:80}
SITE_ADDRESS=$DOMAIN_VALUE
TLS_MODE=public

case "$DOMAIN_VALUE" in
	*" tls internal")
		SITE_ADDRESS=$(printf '%s' "$DOMAIN_VALUE" | sed 's/[[:space:]]tls[[:space:]]internal$//')
		TLS_MODE=internal
		;;
esac

write_tls_policy() {
	if [ "$TLS_MODE" = "internal" ]; then
		printf '\ttls internal {\n'
	else
		printf '\ttls {\n'
	fi
	cat <<'EOF'
		# TLS 1.3 cipher suites are controlled by Go and are not configurable
		# in Caddy. Go's TLS 1.3 server offers the modern AEAD suites requested
		# here; the explicit list below rejects weak TLS 1.2 suites.
		protocols tls1.2 tls1.3
		ciphers TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256 TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	}
EOF
}

write_security_headers() {
	cat <<'EOF'
	header {
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
		Permissions-Policy "geolocation=(), microphone=(), camera=(), payment=()"
		Cross-Origin-Resource-Policy "same-site"
		Content-Security-Policy "default-src 'none'; frame-ancestors 'none'"
		Server "cactus-watch"
EOF
	if [ "$SITE_ADDRESS" = "${SITE_ADDRESS#:}" ]; then
		printf '\t\tStrict-Transport-Security "max-age=31536000; includeSubDomains; preload"\n'
	fi
	cat <<'EOF'
	}
EOF
}

{
	printf '%s {\n' "$SITE_ADDRESS"
	if [ "$SITE_ADDRESS" != "${SITE_ADDRESS#:}" ]; then
		if [ "$TLS_MODE" = "internal" ]; then
			write_tls_policy
		fi
	else
		write_tls_policy
	fi
	write_security_headers
	printf '\treverse_proxy api:8080\n'
	printf '\thandle_errors {\n'
	write_security_headers
	printf '\t\trespond "{http.error.status_code} {http.error.status_text}" {http.error.status_code}\n'
	printf '\t}\n'
	printf '}\n'
} > "$SITE_SNIPPET"

exec "$@"
