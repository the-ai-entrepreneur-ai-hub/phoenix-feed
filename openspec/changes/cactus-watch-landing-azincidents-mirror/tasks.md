# Tasks

## 1. Spec and Investigation

- [x] Validate this proposal with `openspec validate cactus-watch-landing-azincidents-mirror --strict`.
- [x] Read the legacy coming-soon page and current landing/sub-page files end to end.
- [x] Grep both repos for `kIncidentMaxAgeMinutes` and `kClearedIncidentMaxAgeSeconds`.
- [x] Read `internal/ingester/`, `internal/janitor/`, and the active-incident query path.
- [x] Identify whether the 90-minute cutoff is enforced backend-side, client-side, or both.

## 2. Landing Implementation

- [x] Rebuild `web/landing/index.html` from the legacy layout with Cactus Watch branding and Dan's approved copy.
- [x] Replace landing CSS with the legacy cream/light style and remove Tailwind/CDN dependencies.
- [x] Rework privacy, terms, about, and FAQ chrome while preserving body copy verbatim.
- [x] Copy app-store badges and Dan's three hero screenshots into `web/landing/img/`.
- [x] Generate resized PNG and WebP hero image siblings with Pillow.
- [x] Delete obsolete `web/landing/css/app.css` and `web/landing/js/app.js`.

## 3. Backend Implementation

- [x] Add a regression unit test for a 2-hour-old incident still observed in the latest poll.
- [x] Confirm phoenix-feed active filtering keeps observed incidents active regardless of dispatch age.
- [x] Confirm cleared/not-seen incidents still expire by the existing cleared retention window.

## 4. Verification

- [x] Run `openspec validate cactus-watch-landing-azincidents-mirror --strict`.
- [x] Serve `web/landing/` locally and curl `/`, `/privacy/`, `/terms/`, `/about/`, and `/faq/`.
- [x] Run the banned CDN grep for Tailwind, Google Fonts, cdnjs, and jsdelivr.
- [x] Run the backend long-running incident unit test.
- [x] Run the relevant Go validators for the touched backend package.
- [x] Confirm `robots.txt`, `sitemap.xml`, CSP config, Docker Compose services, and legacy source folder are untouched.
- [ ] Commit and push to `main` with a `change-id: cactus-watch-landing-azincidents-mirror` trailer.
- [ ] Deploy by SSH pull and Caddy restart, then verify `https://cactuswatch.com` and `https://feed.cactuswatch.com/v1/health`.
