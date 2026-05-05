#!/bin/sh
set -eu

# Caddy entrypoint: renders /tmp/phoenix-feed-site.caddy from environment.
#
# Two modes supported:
#
#   1. Multi-site production mode — set LANDING_DOMAIN (apex), API_DOMAIN
#      (subdomain). Caddy auto-issues Let's Encrypt certs for each, redirects
#      www.<apex> to <apex>, serves /srv/landing as static files on the apex,
#      and reverse proxies the API on the API_DOMAIN.
#
#   2. Legacy single-site mode — set DOMAIN to one of:
#         :80                              plain HTTP (IP-only testing)
#         host.example.com                 auto Let's Encrypt
#         host.example.com tls internal    self-signed
#      Reverse proxies api:8080 only. Used before a real domain is wired.

SITE_SNIPPET=/tmp/phoenix-feed-site.caddy
DOMAIN_VALUE=${DOMAIN:-:80}
LANDING_DOMAIN_VALUE=${LANDING_DOMAIN:-}
API_DOMAIN_VALUE=${API_DOMAIN:-}

write_tls_policy() {
	# arg 1: "internal" or "public"
	mode="$1"
	if [ "$mode" = "internal" ]; then
		printf '\ttls internal {\n'
	else
		printf '\ttls {\n'
	fi
	cat <<'EOF'
		# TLS 1.3 ciphers are managed by Go and not configurable in Caddy.
		# The list below restricts TLS 1.2 to modern AEAD suites only.
		protocols tls1.2 tls1.3
		ciphers TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256 TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	}
EOF
}

write_api_security_headers() {
	# arg 1: "yes" to include HSTS, "no" otherwise
	hsts="$1"
	cat <<'EOF'
	header {
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
		Permissions-Policy "geolocation=(), microphone=(), camera=(), payment=()"
		Cross-Origin-Resource-Policy "same-site"
		Content-Security-Policy "default-src 'none'; frame-ancestors 'none'"
		>Server "cactus-watch"
		-Via
EOF
	if [ "$hsts" = "yes" ]; then
		printf '\t\tStrict-Transport-Security "max-age=31536000; includeSubDomains; preload"\n'
	fi
	printf '\t}\n'
}

write_landing_security_headers() {
	# Relaxed CSP so inline styles and SVG favicon work; HSTS always on
	# (landing only runs in multi-site mode where TLS is mandatory).
	cat <<'EOF'
	header {
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		Referrer-Policy "strict-origin-when-cross-origin"
		Permissions-Policy "geolocation=(), microphone=(), camera=(), payment=()"
		Content-Security-Policy "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self' mailto:"
		Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
		>Server "cactus-watch"
		-Via
	}
EOF
}

write_rate_limit() {
	cat <<'EOF'
	rate_limit {
		zone per_source_ip {
			key {http.request.remote.host}
			events 60
			window 1m
		}
	}
EOF
}

write_log() {
	cat <<'EOF'
	log {
		output file /var/log/caddy/access.log {
			roll_size 10MiB
			roll_keep 7
			roll_keep_for 168h
		}
		format json
	}
EOF
}

if [ -n "$LANDING_DOMAIN_VALUE" ] && [ -n "$API_DOMAIN_VALUE" ]; then
	WWW_DOMAIN="www.${LANDING_DOMAIN_VALUE}"

	{
		# Landing apex: static files
		printf '%s {\n' "$LANDING_DOMAIN_VALUE"
		write_tls_policy public
		write_landing_security_headers
		write_rate_limit
		write_log
		printf '\troot * /srv/landing\n'
		printf '\tfile_server\n'
		printf '\tencode gzip zstd\n'
		printf '\thandle_errors {\n'
		write_landing_security_headers
		printf '\t\trespond "{http.error.status_code} {http.error.status_text}" {http.error.status_code}\n'
		printf '\t}\n'
		printf '}\n\n'

		# www -> apex (301)
		printf '%s {\n' "$WWW_DOMAIN"
		write_tls_policy public
		printf '\tredir https://%s{uri} permanent\n' "$LANDING_DOMAIN_VALUE"
		printf '}\n\n'

		# API subdomain: reverse proxy
		printf '%s {\n' "$API_DOMAIN_VALUE"
		write_tls_policy public
		write_api_security_headers yes
		write_rate_limit
		write_log
		printf '\treverse_proxy api:8080\n'
		printf '\thandle_errors {\n'
		write_api_security_headers yes
		printf '\t\trespond "{http.error.status_code} {http.error.status_text}" {http.error.status_code}\n'
		printf '\t}\n'
		printf '}\n'
	} > "$SITE_SNIPPET"
else
	# Legacy single-site mode
	SITE_ADDRESS=$DOMAIN_VALUE
	TLS_MODE=public

	case "$DOMAIN_VALUE" in
		*" tls internal")
			SITE_ADDRESS=$(printf '%s' "$DOMAIN_VALUE" | sed 's/[[:space:]]tls[[:space:]]internal$//')
			TLS_MODE=internal
			;;
	esac

	HSTS_FLAG=no
	if [ "$SITE_ADDRESS" = "${SITE_ADDRESS#:}" ]; then
		HSTS_FLAG=yes
	fi

	{
		printf '%s {\n' "$SITE_ADDRESS"
		if [ "$SITE_ADDRESS" != "${SITE_ADDRESS#:}" ]; then
			if [ "$TLS_MODE" = "internal" ]; then
				write_tls_policy internal
			fi
		else
			write_tls_policy public
		fi
		write_api_security_headers "$HSTS_FLAG"
		write_rate_limit
		write_log
		printf '\treverse_proxy api:8080\n'
		printf '\thandle_errors {\n'
		write_api_security_headers "$HSTS_FLAG"
		printf '\t\trespond "{http.error.status_code} {http.error.status_text}" {http.error.status_code}\n'
		printf '\t}\n'
		printf '}\n'
	} > "$SITE_SNIPPET"
fi

exec "$@"
