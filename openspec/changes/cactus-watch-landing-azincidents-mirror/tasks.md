# Tasks

## 1. Spec and Investigation

- [x] Validate this proposal with `openspec validate cactus-watch-landing-azincidents-mirror --strict`.
- [ ] Read the legacy coming-soon page and current landing/sub-page files end to end.
- [ ] Grep both repos for `kIncidentMaxAgeMinutes` and `kClearedIncidentMaxAgeSeconds`.
- [ ] Read `internal/ingester/`, `internal/janitor/`, and the active-incident query path.
- [ ] Identify whether the 90-minute cutoff is enforced backend-side, client-side, or both.

## 2. Landing Implementation

- [ ] Rebuild `web/landing/index.html` from the legacy layout with Cactus Watch branding and Dan's approved copy.
- [ ] Replace landing CSS with the legacy cream/light style and remove Tailwind/CDN dependencies.
- [ ] Rework privacy, terms, about, and FAQ chrome while preserving body copy verbatim.
- [ ] Copy app-store badges and Dan's three hero screenshots into `web/landing/img/`.
- [ ] Generate resized PNG and WebP hero image siblings with Pillow.
- [ ] Delete obsolete `web/landing/css/app.css` and `web/landing/js/app.js`.

## 3. Backend Implementation

- [ ] Add a failing unit test for a 2-hour-old incident still observed in the latest poll.
- [ ] Change active-window filtering so observed incidents remain active regardless of dispatch age.
- [ ] Confirm cleared/not-seen incidents still expire by the existing cleared retention window.

## 4. Verification

- [ ] Run `openspec validate cactus-watch-landing-azincidents-mirror --strict`.
- [ ] Serve `web/landing/` locally and curl `/`, `/privacy/`, `/terms/`, `/about/`, and `/faq/`.
- [ ] Run the banned CDN grep for Tailwind, Google Fonts, cdnjs, and jsdelivr.
- [ ] Run the backend long-running incident unit test.
- [ ] Run the relevant Go validators for the touched backend package.
- [ ] Confirm `robots.txt`, `sitemap.xml`, CSP config, Docker Compose services, and legacy source folder are untouched.
- [ ] Commit and push to `main` with a `change-id: cactus-watch-landing-azincidents-mirror` trailer.
- [ ] Deploy by SSH pull and Caddy restart, then verify `https://cactuswatch.com` and `https://feed.cactuswatch.com/v1/health`.
