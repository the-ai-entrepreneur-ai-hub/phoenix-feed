# Cactus Watch Landing Mirror and Active Window Retention

## Why

Dan approved the original azincidentalert.com coming-soon layout, but the current Cactus Watch landing pages no longer match that cream/light visual language. The public site needs to return to that approved structure while using Cactus Watch branding, self-hosted assets, and the production CSP boundary.

Dan also reported a mountain rescue that Phoenix Fire still showed as active about 2 hours and 23 minutes after dispatch, while Cactus Watch no longer showed it. Active incidents that are still observed by the poller must remain in the feed even when their dispatch age exceeds the old 90-minute client/backend cutoff.

## What Changes

1. Rebuild `web/landing/index.html` from `legacy-coming-soon-page/index.html`, replacing AZ Incidents branding with Cactus Watch and Dan's approved 2026-05-17 copy.
2. Restore `web/landing/style.css` to the legacy cream/light style without Tailwind, Google Fonts, CDN scripts, or third-party trackers.
3. Rework the privacy, terms, about, and FAQ pages to the same cream/light chrome while preserving their existing body copy verbatim.
4. Copy Dan's three iPhone screenshots into `web/landing/img/`, create resized PNG files and WebP siblings, and preserve app-store badge assets.
5. Delete the obsolete Tailwind/observer landing assets that are incompatible with the production CSP.
6. Trace the active-incident retention rule and keep incidents that are still observed in the latest poll, regardless of dispatch age, until they age out after the cleared/not-seen window.
7. Add a backend unit test proving a 2-hour-old incident still observed in the latest poll remains in the active feed.

## Out of Scope

- Changing production CSP rules, Caddy proxy routing, Docker Compose service topology, TLS volumes, or API response shapes.
- Editing `legacy-coming-soon-page/`, `robots.txt`, `sitemap.xml`, or landing body copy for legal/about/FAQ pages.
- Adding analytics, trackers, third-party scripts, external fonts, or new backend dependencies.

## Impact

- Static landing files under `web/landing/`.
- Landing images under `web/landing/img/`.
- Backend active-incident filtering code after investigation.
- Backend tests for long-running active incident retention.
- Production deploy requires pulling `main` on the droplet and restarting Caddy only after tests and commit land.
