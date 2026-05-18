# Codex Brief — Cactus Watch v1 Polish (OpenSpec-driven, 2 repos)

**Issued:** 2026-05-18
**Owner:** George (The AI Entrepreneur)
**Client:** Dan
**Estimated duration:** 60 to 120 minutes of agent work. Long runs are explicitly approved.
**Progress file:** `D:\serveless-apps-2026\abusedMindset\phoenix-feed\docs\codex-cactus-watch-v1-polish-PROGRESS.md` — update after every task transition.

---

## Goal

Land every Dan feedback item from May 17 2026 across two repos, organized through OpenSpec change proposals so the work stays auditable. Two distinct changes: one in phoenix-feed (website rebuild plus backend active-window investigation), one in cactus_repo (iOS UI tightening). Deliver end to end: spec, tasks, implementation, tests, commit, push, and for the backend SSH deploy.

## Visual evidence (from Dan's WhatsApp screenshots, 2026-05-17 night)

Source files: `C:\Users\Administrator\Downloads\WhatsApp Image 2026-05-17 at *.jpeg`

**Screenshot 1 (11:22:58 PM) — incident feed, dark mode, TestFlight 5060**

- Visible tiles: DEBRIS FIRE (black), BRUSH FIRE (vivid blue), Crash Involving Motorcycle (vivid blue), 962 (black, partially cut off).
- Issue 1: DEBRIS FIRE shows "Dispatched: E36" with `E36` rendered in ORANGE while the same field on the blue BRUSH FIRE tile renders unit IDs in WHITE. Two different colors for the same data field.
- Issue 2: Only 3 incidents fit before the 4th (962) gets clipped at the bottom nav. In build 5054 the same screen showed 4 to 5 tiles. Font and padding are too generous.

**Screenshot 2 (11:17:52 PM) — same feed, Dan's hand annotations**

- Red circle around the clipped title "CRASH INVOLVING MO..." with ellipsis. Title must wrap to two lines.
- Light circle (marker) around the raw "962" tile. Dan wants 962 expanded to "Vehicle Crash" everywhere.
- Address "16800 W YUMA RD, GDY" rendered in low-contrast washed-out text against the bright blue tile background. Barely legible.

**Screenshot 3 (11:07:14 PM) — Select Units / favorites screen, dark mode**

- Dan drew a thick white circle around the SnackBar reading "Added E602" that is partially overlapping the still-visible Save button. The toast fired on Enter, BEFORE Save was tapped, so it lied about persistence.
- Visible favorites list: All Battalion Chiefs, E31, E304, E601, E609, E610, E611.

**Screenshot 4 (11:22:59 PM) — same feed, Dan's red circle around the Responding units block**

- BRUSH FIRE shows the units "BC192 | BE6130 | BR613 | E45 | E49 | E5..." with the last unit cut off at the right edge, and a "Dispatched: BC603 | BE611 | C919 | E613 | NDC | T612" line below that wraps awkwardly. Whole responding block is hard to scan and consumes ~40 percent of the tile.

---

## Workspaces

- Repo A: `D:\serveless-apps-2026\abusedMindset\phoenix-feed` (Go backend, Caddy, static landing site)
- Repo B: `D:\serveless-apps-2026\abuserOfcreativity\cactus_repo` (Flutter iOS app)
- Production droplet (SSH only, used at deploy step for change 1): `root@64.23.209.182` port 22, key `~/.ssh/id_ed25519`. App lives at `/opt/phoenix-feed/app`.

## Two OpenSpec changes to create

### Change 1 — `cactus-watch-landing-azincidents-mirror` (repo: phoenix-feed)

Restore the original azincidentalert.com page layout that Dan approved, rebranded for Cactus Watch. Also investigate the missing-mountain-rescue case in the active-window filter so long-running incidents do not get dropped while Phoenix Fire still lists them as active.

### Change 2 — `cactus-watch-ui-tightening-5061` (repo: cactus_repo)

Tighten the iOS app per Dan's six tile-level complaints from 5060: dark mode tile-color uniformity, address text contrast in dark mode, font density to match 5054, suppress the premature unit-added toast, wrap long titles to two lines, and expand the raw 962 dispatch codes to human labels.

---

## Workflow rule for both changes

For every change-id below, before any implementation:

1. `cd <repo>` and run `openspec list` to confirm OpenSpec is set up.
2. Create the change directory under `openspec/changes/<change-id>/`.
3. Write the four files in this exact order: `proposal.md`, `design.md`, `tasks.md`, `specs/<capability>/spec.md`.
4. Run `openspec validate <change-id> --strict`. Do not start coding until it passes.
5. Mark each task in `tasks.md` `[ ]` to `[x]` only when the work is done AND its verification step has run green.
6. Commit using Conventional Commits with a `change-id` trailer in the body: e.g. `feat(landing): ...\n\nchange-id: cactus-watch-landing-azincidents-mirror`.
7. **After every committed task, append a row to the PROGRESS file (see Progress reporting section at the end).**

---

## Change 1 details — phoenix-feed website rebuild

### Source of truth for layout

`D:\serveless-apps-2026\abusedMindset\phoenix-feed\legacy-coming-soon-page\index.html` is the exact azincidentalert.com page Dan wants mirrored. Use it as the structural template. Keep the existing layout, typography, colors, hero composition, app-store badges, and footer. Only the following changes apply:

- Brand swap: every "AZ Incidents" string becomes "Cactus Watch".
- Logo: replace `Cactus Alert.svg` references with `/favicon.svg` (already present in `web/landing/favicon.svg`). Confirm it renders.
- Headline below the logo: "Stay Informed with Cactus Watch".
- Subhead: replace the existing paragraph with Dan's verbatim verbiage from his 2026-05-17 message:

```
Incident Awareness for First Responders & Enthusiasts

Cactus Watch keeps you informed about emergency incidents across the Phoenix metro area, including fires, vehicle accidents, and more.

Designed for first responders, their families, and enthusiasts, Cactus Watch delivers easy access to public incident activity through a clean and detailed interactive map interface. View incident locations, dispatch details, timestamps, streets, intersections, and responding units when available.

Cactus Watch is an independent informational tool and is not affiliated with or endorsed by any government agency or emergency service provider. Information may be delayed, incomplete, or unavailable and may not reflect all incidents. Cactus Watch is provided for informational and situational-awareness purposes only and should not be relied upon for emergency response or safety decisions.
```

- Phone screenshots: use the three new iPhone shots Dan provided. Source files at `D:\serveless-apps-2026\abuserOfcreativity\abusedDRS\` named:
  - `Left device (home feed).png`
  - `Center device (Phoenix metro map).png`
  - `Right device (incident detail with units roster).png`

  Copy to `web/landing/img/` with URL-safe names (`hero-home-feed.png` etc.). Generate WebP siblings using Pillow (`quality=78, method=6`) and resize so each long side is between 720 and 900 px, so the PNG fallback drops from ~5 MB total to under 800 KB.

- App store badges: copy `app-store.png` and `google-play.png` from `legacy-coming-soon-page/` to `web/landing/img/` if not already there.
- Coming soon ribbon: keep it on the page until Dan removes it explicitly. Do not assume readiness.

### Sub-pages must also revert to the cream/light style

The current dark-themed versions at `web/landing/privacy/index.html`, `terms/index.html`, `about/index.html`, `faq/index.html` are out. Rewrite them in the same cream/light visual language as the legacy page. **Preserve the body content verbatim** (privacy clauses, terms, about copy, FAQ Q+A). Only the chrome (header, footer, type, color palette) changes.

### CSP compliance (HARD CONSTRAINT)

Production Caddy serves the landing with this CSP header:

```
default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self' data:
```

That means: NO external scripts (no Tailwind CDN), NO Google Fonts, NO third-party trackers. Everything self-hosted. The legacy `azincidentalert.com` page used inline CSS and `style.css` only, which is CSP-compliant. Stay that way. If new fonts are needed beyond system stack, self-host the woff2 under `/fonts/` and load from same origin.

After the rewrite, run this grep and assert the output is empty:

```
grep -rE "cdn\\.tailwindcss|fonts\\.google|fonts\\.gstatic|cdnjs|jsdelivr" web/landing/
```

### Backend active-window fix (still under Change 1)

Dan reported on 2026-05-17 that a mountain rescue dispatched at 8:39 PM was visible on Phoenix Fire's public page but missing from Cactus Watch at 11:02 PM (about 2 hours 23 minutes later). Cause is almost certainly the `kIncidentMaxAgeMinutes = 90` hard cutoff at `cactus_repo\lib\utils\app_constants.dart:43` OR a backend equivalent in phoenix-feed's ingester/janitor.

Steps:

1. Grep `kIncidentMaxAgeMinutes` and `kClearedIncidentMaxAgeSeconds` across both repos. Identify enforcement point.
2. Read `internal/ingester/`, `internal/janitor/`, and any active-incident query in phoenix-feed.
3. Determine whether the 90-minute cap is backend-side (in the API JSON we serve) or client-side (in Flutter).
4. **Fix rule:** do NOT drop incidents whose `seconds_since_last_seen` is below `kClearedIncidentMaxAgeSeconds`. If the latest poll still observed the incident, it stays. The 90-minute cap only applies to incidents we have STOPPED seeing. Long-running mountain rescues, crashes with extrication, structure fires routinely exceed 90 minutes; they are legitimate.
5. Add a unit test simulating an incident dispatched 2 hours ago but still observed in the latest poll, asserting it is included in the feed.

### File targets (Change 1)

| File | Action |
|---|---|
| `phoenix-feed/web/landing/index.html` | Rewrite from `legacy-coming-soon-page/index.html` template, rebranded |
| `phoenix-feed/web/landing/style.css` | Copy from `legacy-coming-soon-page/style.css`, no Tailwind |
| `phoenix-feed/web/landing/privacy/index.html` | Rewrite in cream/light style, keep body content |
| `phoenix-feed/web/landing/terms/index.html` | Same |
| `phoenix-feed/web/landing/about/index.html` | Same |
| `phoenix-feed/web/landing/faq/index.html` | Same |
| `phoenix-feed/web/landing/img/hero-*.png` and `.webp` | Resize + WebP siblings of Dan's 3 hero shots |
| `phoenix-feed/web/landing/img/app-store.png`, `google-play.png` | Copy from legacy folder |
| `phoenix-feed/web/landing/favicon.svg` | Already present, do not touch |
| `phoenix-feed/web/landing/css/app.css`, `web/landing/js/app.js` | DELETE (Tailwind compile + observer no longer needed) |
| Backend incident filter | Identify, fix, test (path determined by step 0 trace) |

### PRESERVE (Change 1)

- `phoenix-feed/web/landing/robots.txt` and `sitemap.xml`
- All Docker compose services (api, db, ingester, canary, janitor, caddy)
- Caddy reverse proxy for the API domain (feed.cactuswatch.com)
- TLS certs in the `caddy_data` volume
- `phoenix-feed/legacy-coming-soon-page/` (Dan's source backup, read-only)
- All v0.4-prod-hardening spec acceptance criteria
- Existing privacy/terms/about/faq body copy

---

## Change 2 details — cactus_repo iOS UI tightening

### Sub-task 2.1 — Dark mode tile color uniformity (HIGHEST PRIORITY)

**What screenshots reveal:** On the same dark-mode screen at the same moment, DEBRIS FIRE and 962 render as solid black tiles while BRUSH FIRE, Crash Involving Motorcycle, and NATURAL GAS LEAK render as solid vivid blue tiles. Dan wants ALL incident tiles to share the same neutral dark color in dark mode. No vivid blue card unless he opts in explicitly.

Investigation steps:

1. Read `lib/widgets/incident_tile.dart` line by line. Find every place a background color is selected.
2. The current logic likely uses one of: incident type (fire vs medical vs hazmat), favorited state (favorited stations get a colored tile), unread state (new since last view), or priority/category. Document which.
3. Cross-reference `lib/utils/app_themes.dart` and `lib/utils/app_colors.dart`. The 10-theme picker (5 pastel + 5 vivid) from build 5048 likely participates.
4. **Fix rule:** In dark mode, every tile background must use a single neutral color (e.g. `#16181B` or `#0A0A0A`). Per-incident or per-state visual differentiation can stay as a thin colored left border (4 px), or as a tinted icon, or as a colored unit-id pill, but never as the entire card background.
5. Add a widget test that renders 4 different incident types in dark mode and asserts all 4 share the same `Card.color` value.

### Sub-task 2.2 — Address text contrast in dark mode

**What screenshots reveal:** "16800 W YUMA RD, GDY" on a vivid blue card appears in such low-contrast washed-out grey that it is barely readable. Even after Sub-task 2.1 normalizes the tile color, address text must hit WCAG AA (4.5:1) against the new dark background.

1. Identify the `Text` widget rendering the address line.
2. Use `Theme.of(context).textTheme.bodyLarge?.color` or a deliberate `Colors.white70` minimum. Never `Colors.white24` or `Colors.black54` in dark mode.
3. Add a test that resolves the address text color in dark mode and asserts a luminance contrast ratio above 4.5 against the tile background.

### Sub-task 2.3 — Font density to match 5054

**What screenshots reveal:** 5060 shows only 3 incidents before the 4th gets clipped at the bottom nav. 5054 fit 4 to 5. Dan was explicit: "the font is too big because all the units get cut off that is on the newest version app which is version 5060. The other one is from version 5054 and they all fit."

1. `git log --oneline -- lib/widgets/incident_tile.dart lib/utils/app_themes.dart` from the 5054 commit to HEAD.
2. Identify font-size or padding bumps introduced between 5054 and 5060.
3. Revert the typography/padding regression while preserving non-typography improvements (color fixes, accessibility tweaks, etc).
4. Specifically aim for: tile vertical padding 16 px down to 12 px, title 18sp down to 16sp, body 14sp down to 13sp. Confirm exact values from `git show <5054-hash>:lib/widgets/incident_tile.dart`.
5. Goal: 4 incidents visible on an iPhone 15 portrait without scrolling.
6. Update `test/incident_tile_test.dart` snapshot to lock the tightened layout.

### Sub-task 2.4 — Suppress "Added E602" toast before Save

**What screenshots reveal:** Image 6 shows the toast overlapping the still-visible Save button. The user typed E602, pressed Enter, and immediately got a SnackBar saying "Added E602", even though Save had not been tapped and the persisted list does not yet include E602.

1. Grep `ScaffoldMessenger.of(context).showSnackBar` across `lib/screens/` and `lib/widgets/`.
2. Locate the SnackBar that fires on `onSubmitted` / `onEditingComplete` in the unit-add input field.
3. Remove it from the Enter handler. Move it to the actual Save tap handler so the toast fires only AFTER persisted state changes.
4. Optional: replace with a subtler in-list visual confirmation (the new unit appears in the list above) so the user still gets feedback.
5. Widget test: simulate typing E602 + Enter, assert NO SnackBar. Then simulate tapping Save, assert the SnackBar appears.

### Sub-task 2.5 — Wrap long incident titles to two lines

**What screenshots reveal:** "CRASH INVOLVING MO..." gets clipped with ellipsis in one screenshot and shows as "Crash Involving / Motorcycle" wrapped to two lines on another tile in the same build. Inconsistent. Dan wants two-line wrap consistently.

1. In `lib/widgets/incident_tile.dart`, find the title `Text` widget.
2. Set `maxLines: 2`, `softWrap: true`, `overflow: TextOverflow.ellipsis`. Let Flutter handle the word break point; do not hardcode line breaks.
3. Verify the tile height adjusts gracefully when a title wraps (no pixel overflow).
4. Test: render a tile with "Crash Involving Motor Vehicle" and assert 2 lines visible with no trailing ellipsis.

### Sub-task 2.6 — Expand 962-family codes to human labels

**What screenshots reveal:** A tile literally reads "962" as the title. Dan wants this expanded to "Vehicle Crash" everywhere.

1. Find the dispatch-code translation table. Likely lives in `lib/utils/app_constants.dart` near `getIconForSymbolCode`, or create a new `lib/utils/dispatch_codes.dart`.
2. Add the mapping:
   - `962` -> `Vehicle Crash`
   - `962X` -> `Crash Requiring Extrication`
   - `962BC` -> `Crash Involving Bicycle`
   - `962MV` -> `Crash Involving Motor Vehicle`
   - `962HZ` -> `Crash with Hazmat`
   - `962F` -> `Crash with Fire`
   - `962R` -> `Crash with Rollover`
   - Any other 962-prefixed codes found in production logs.
3. Apply translation at the **data ingestion layer** (when JSON arrives from the API), not at render time. Store both raw code and human label on the incident model so other features can reference either.
4. Fallback: if no translation exists, render the raw code (current behavior) so unknown codes never break.
5. Unit tests covering each translation and the fallback case.

### File targets (Change 2)

| File | Action |
|---|---|
| `cactus_repo/lib/widgets/incident_tile.dart` | Tile color uniformity, contrast, font density, title wrap |
| `cactus_repo/lib/utils/app_themes.dart` | Possible single-color helper for dark mode tile background |
| `cactus_repo/lib/utils/app_colors.dart` | Same |
| `cactus_repo/lib/utils/app_constants.dart` OR new `lib/utils/dispatch_codes.dart` | 962 translation table |
| `cactus_repo/lib/screens/` (unit favorites screen) | Remove premature SnackBar |
| `cactus_repo/lib/models/incident.dart` | Add `displayLabel` field, populated at ingestion |
| `cactus_repo/test/incident_tile_test.dart`, `test/dispatch_codes_test.dart` | New + updated tests |
| `cactus_repo/codemagic.yaml` | **Do NOT bump BUILD_FLOOR.** George owns the trigger. |

### PRESERVE (Change 2)

- Every existing AppTheme enum value (pastel + vivid, 10 total)
- Cactus Watch user-facing branding strings
- TestFlight build pipeline (do NOT trigger Codemagic; George's call)
- SharedPrefs theme index back-compat (do not reorder enum entries)
- The Apple ITMS-90683 `NSLocationWhenInUseUsageDescription` string in `ios/Runner/Info.plist`
- Firebase Crashlytics wiring in `lib/services/firabse_service.dart` and `lib/providers/incidents_provider.dart`
- All 170+ currently-passing flutter tests
- Dio timeout config in `lib/services/base_api_service.dart` (30s connect/receive/send)
- StoreKit pricing `$6.99/mo`, `$76.99/yr` at `ios/Sales.storekit`

---

## Progress reporting

After every committed task, append a row to:

`D:\serveless-apps-2026\abusedMindset\phoenix-feed\docs\codex-cactus-watch-v1-polish-PROGRESS.md`

Format (markdown table):

```
| UTC time | change-id | task | status | commit | notes |
```

- `status` is one of: STARTED, DONE, BLOCKED, REVERTED.
- For BLOCKED, fill `notes` with the specific question or blocker.
- Commit + push the PROGRESS file update after every entry so George can `git pull` and see progress without waiting for the whole job to finish.

If you hit a blocker that needs a human decision, add a BLOCKED row, stop work on that sub-task, and continue with the next independent sub-task. Do not idle.

---

## Verification gates

### After Change 1

- `cd phoenix-feed && openspec validate cactus-watch-landing-azincidents-mirror --strict` returns clean.
- `python -m http.server` in `web/landing/` and curl every page: `/`, `/privacy/`, `/terms/`, `/about/`, `/faq/`. All return 200 with the cream/light theme matching the legacy azincidentalert.com look.
- `grep -rE "cdn\\.tailwindcss|fonts\\.google|fonts\\.gstatic" web/landing/` returns nothing.
- Backend unit test for the long-running incident retention passes.
- Git commit + push to main.
- SSH to droplet: `ssh -i ~/.ssh/id_ed25519 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@64.23.209.182 'cd /opt/phoenix-feed/app && git pull origin main && docker compose -f docker-compose.prod.yml restart caddy'`
- `curl -sI https://cactuswatch.com` returns 200 with the new title.
- `curl -sI https://feed.cactuswatch.com/v1/health` proves API still healthy.

### After Change 2

- `cd cactus_repo && openspec validate cactus-watch-ui-tightening-5061 --strict` returns clean.
- `flutter analyze --no-pub` returns "No issues found".
- `flutter test --no-pub` returns all green (170+ new tests included).
- `grep -rE "962" lib/utils/` shows the translation table is in place.
- Visual sanity: simulator screenshot of the feed in dark mode shows 4+ tiles, all uniform dark background, address text fully legible.
- Git commit + push to `feat/wire-cactuswatch-backend` branch.
- Do NOT bump `BUILD_FLOOR` or trigger Codemagic. Final report should say "ready for George to bump floor and trigger build XXXX".

## Output format (paste at end of run)

For each change-id confirm with the exact YES/NO words:

```
[ Change 1: cactus-watch-landing-azincidents-mirror ]
- openspec validate strict passed:    YES / NO
- legacy azincidentalert layout mirrored:    YES / NO
- 4 sub-pages reverted to cream/light style:    YES / NO
- 3 hero images resized + WebP siblings present:    YES / NO
- no tailwind/google fonts/external scripts in HTML:    YES / NO
- backend active-window filter fix landed:    YES / NO
- backend test for long-running incident green:    YES / NO
- committed and pushed (hash):    YES / NO  hash=__________
- SSH deploy succeeded, curl https://cactuswatch.com returns 200 with new title:    YES / NO
- existing privacy/terms/about/faq content preserved exactly:    YES / NO
- API still healthy (curl /v1/health):    YES / NO

[ Change 2: cactus-watch-ui-tightening-5061 ]
- openspec validate strict passed:    YES / NO
- all dark mode tiles share a single neutral background:    YES / NO
- address text passes WCAG AA contrast in dark mode:    YES / NO
- font density restored to 5054 baseline (4+ tiles fit):    YES / NO
- premature unit-added SnackBar removed:    YES / NO
- long incident titles wrap to 2 lines, no clip:    YES / NO
- 962 + variants translated to human labels at ingestion:    YES / NO
- flutter analyze --no-pub clean:    YES / NO
- flutter test --no-pub passes (total tests: ___):    YES / NO
- committed and pushed (hash):    YES / NO  hash=__________
- BUILD_FLOOR NOT bumped, Codemagic NOT triggered (awaiting George):    YES / NO
- existing 10 themes still selectable:    YES / NO
- existing Crashlytics wiring intact:    YES / NO
- existing StoreKit prices unchanged:    YES / NO
```

If any line is NO, explain in 2 sentences underneath.

## Out of scope (do not do)

- Triggering Codemagic for the iOS app. George owns the build trigger.
- Touching the cactus_repo TestFlight pipeline beyond the code changes themselves.
- Modifying the production CSP rules.
- Bumping any version numbers or marketing strings beyond what is explicitly listed.
- Touching `D:\serveless-apps-2026\abuserOfcreativity\abusedDRS\` (read-only source for screenshots).
- Adding analytics, trackers, or third-party scripts of any kind.
- Editing this brief or the PROGRESS file's header. Only append rows to the PROGRESS table.

## Rollback plans

### Website

```
cd /opt/phoenix-feed/app
git log --oneline -5
git revert HEAD --no-edit
git push origin main
git pull
docker compose -f docker-compose.prod.yml restart caddy
```

Old dark Apple-style landing returns within 30 seconds.

### iOS app

Not deployed by this brief (no Codemagic trigger). If George decides not to ship, just `git revert` the change-2 commit and force-push the branch. No TestFlight impact.
