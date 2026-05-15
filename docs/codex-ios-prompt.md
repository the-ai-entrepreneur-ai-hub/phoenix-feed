# Codex iOS Wire-Up Prompt — Cactus Watch v1 to TestFlight

**When to use this:** after the data-quality-overhaul PR is merged and the live API at `https://feed.cactuswatch.com` is producing clean labels (Vehicle Crash, multi-unit arrays, severity, unit_type). Paste the block below into Codex (`codex exec --dangerously-bypass-approvals-and-sandbox`).

---

```
You are a senior Flutter / iOS release engineer running autonomously to wire the Cactus Watch app to its production API and ship a build to TestFlight. There is no margin for error. Every claim of "done" must be backed by a verified observation, not a hopeful assumption. If a gate fails, you stop and report — no workarounds.

CORRECTION FROM PRIOR ATTEMPT: this is a Flutter app, not native iOS. Local Xcode builds are impossible on this Windows box. The build verification gate runs on Codemagic, not locally. Reconnaissance shows the app currently bypasses our backend entirely and hits Phoenix Maps directly — that is the headline thing to fix.

REFERENCE DATA

- iOS repo (already cloned): D:\serveless-apps-2026\abuserOfcreativity\cactus_repo
- Remote: github.com/dani4553/Cactus_Alert.git
- Branch to work from: recovered/build-18 (current); cut a new feature branch feat/wire-cactuswatch-backend for your changes
- Production API base URL: https://feed.cactuswatch.com
- Backend cert: Let's Encrypt, valid through 2026-08-03; ATS-clean, no NSAllowsArbitraryLoads workaround needed

Endpoints the iOS app must call (anonymous, free tier — no API key):

- GET /v1/health — diagnostic only, do not show in main feed UI
- GET /v1/incidents/active — main feed source
- POST /v1/incidents/refresh — pull-to-refresh; server throttles per source IP to once per 120 sec, returns 429 with Retry-After header on abuse
- GET /v1/incidents/{source}/{incident_id} — single-incident detail (use only if a row tap shows a detail screen; otherwise skip)

Response top-level shape (your Dart models must match this exactly):

{
  "meta": {
    "source_last_success_at": "<RFC3339>",
    "data_age_seconds": 45,
    "parser_version": "phx-fire-2026-05",
    "disclaimer": "Not for emergency use; call 911",
    "attribution": "Data via City of Phoenix Fire Department",
    "refresh_min_seconds": 600,
    "tier": "free"
  },
  "incidents": [
    {
      "source": "phoenix-fire-mapserver",
      "incident_id": "F26200443",
      "nature_code": "962",
      "nature_desc": "Vehicle Crash",
      "severity": "low",
      "units": [
        {"Unit": "E23", "Status": "Responding", "unit_type": "Engine"},
        {"Unit": "BC2", "Status": "Command", "unit_type": "BattalionChief"}
      ],
      "channel": "K8",
      "symbol_code": "sc004-crash",
      "location_text": "S 32ND ST/I10 ,PHX",
      "lon": -112.011,
      "lat": 33.411,
      "incident_date": "<RFC3339>",
      "received_at": "<RFC3339>",
      "last_seen_at": "<RFC3339>",
      "source_last_success_at": "<RFC3339>",
      "seconds_since_last_seen": 16
    }
  ]
}

This is completely different from the ESRI feature JSON the app currently parses from Phoenix Maps. You will rewrite the Dart models — do not try to massage the old ones.

CI: Codemagic. Workflow name ios-testflight (do NOT rename). App Store Connect API credentials are already configured in the Codemagic team environment:

- Issuer ID: c738e3d2-e54a-488d-a980-7b86042c803a
- Key ID: H2G5SPKG5U
- .p8 path on this box: C:\Users\Administrator\Downloads\AuthKey_H2G5SPKG5U(1).p8 (do not transmit; Codemagic already has its own copy)

Codemagic API token: search environment / ~/.codemagic / codemagic.yaml comments for the token. If absent, stop and report — Dan provides.

WORK TO PERFORM (in order)

1. Swap the API base URL. In lib/services/base_api_service.dart (and any callers / config files), replace the Phoenix Maps URL with https://feed.cactuswatch.com. The base URL must live in one place — a config constant. Search for any hardcoded ArcGIS URLs elsewhere in the codebase and either route them through the config constant or delete them.

2. Rewrite the Dart models to match the new JSON shape above. Old model file(s) for ESRI features get deleted. New models: IncidentFeed (root with meta + incidents), FeedMeta (the disclaimer/attribution/refresh fields), Incident, IncidentUnit. JSON deserialization via the project's existing approach (json_serializable / freezed / manual — don't add a new dep).

3. Update the active-feed call. Whatever method currently fetches and parses Phoenix Maps' response should now hit GET /v1/incidents/active, parse into IncidentFeed, and return.

4. Wire pull-to-refresh. The user gesture must call POST /v1/incidents/refresh. On 200, replace list contents from the response. On 429 or 5xx, keep the current list visible and avoid crash dialogs, stack traces, or persistent feed-delay banners.

5. Render the meta block.
   - Persistent banner at the top of the feed: meta.disclaimer ("Not for emergency use; call 911"). Render the server's value, do not hard-code it — copy can change without an app update.
   - Footer beneath the list: small text rendering meta.attribution.
   - If meta.refresh_min_seconds differs from 600, use the server's value as the auto-refresh interval (foreground only).

6. Free tier UI per the v2.0 decision brief. v1 is intentionally spartan:
   - Single muted color, no theme picker
   - Disclaimer banner (above list, never dismissible)
   - Scrollable list of incidents from the API
   - Each row: nature_desc (large), location_text (medium), incident_date formatted as relative time ("3 minutes ago"), units rendered as comma-joined string ("E23 (Responding), L7 (Dispatched)")
   - Footer: meta.attribution
   - Pull-to-refresh gesture
   - Auto-refresh every 600 seconds in foreground only — no background fetch
   - Use the new severity field for visual emphasis: high = subtle red dot, medium = yellow dot, low = no dot. Do not introduce a paid-tier color theme.
   - Hide / gate behind Tier.paid flag (default off): any existing map view, color theme picker, fire-station overlay, custom alert UI. Do not delete this code unless required by the permission strip; just hide it.

7. Strip v1-irrelevant permissions (Dan approved):
   - lib/services/push_notification_service.dart — comment out or wrap in if (Tier.paid) the permission request. Do not delete the file.
   - ios/Runner/Info.plist — remove the location-permission key (the NSLocationWhenInUseUsageDescription or similar at line ~33) and the background mode entry (UIBackgroundModes at line ~44).
   - AndroidManifest.xml — same treatment for the Android side: strip location and background-fetch permissions if they exist.
   - Goal: the app launches and asks for zero runtime permissions in v1.

8. Display name and bundle.
   - ios/Runner/Info.plist CFBundleDisplayName → "Cactus Watch"
   - Android app/src/main/AndroidManifest.xml android:label → "Cactus Watch"
   - Do NOT change the bundle identifier or applicationId — keeping TestFlight history.

9. Privacy / Terms links. Settings screen (or About screen) should link to https://cactuswatch.com/privacy and https://cactuswatch.com/terms. These URLs are LIVE — verified pages with App Store ready content. Plus link to https://cactuswatch.com/about and https://cactuswatch.com/faq if a Settings screen exists.

10. Codemagic config sanity. Open codemagic.yaml. Confirm the ios-testflight workflow:
    - Triggers on push to feat/wire-cactuswatch-backend (add a trigger rule if missing)
    - Uses flutter build ipa --release or equivalent
    - Has app_store_connect integration set with submit_to_testflight: true
    - Bundle identifier matches the project's existing one (do not change)
    - Build number auto-increments via app-store-connect get-latest-app-store-build-number
    Make minimal changes only.

VERIFICATION GATES (each must produce evidence)

Run in order. If any fails, stop and report.

1. Pre-flight. cd D:\serveless-apps-2026\abuserOfcreativity\cactus_repo && git status returns clean. Print the output.

2. Backend reachable with the new shape. curl -sS https://feed.cactuswatch.com/v1/incidents/active | head -c 500 returns JSON containing nature_desc with human labels (e.g., "Vehicle Crash"), severity field, and units array with unit_type. If the response shape is missing severity or unit_type, the data-quality-overhaul PR has not been merged yet — stop and tell Dan to merge it first.

3. API base URL swap landed. After your edits: grep -rn "maps.phoenix.gov" --include="*.dart" --include="*.yaml" --include="*.plist" . returns zero hits. Paste the (empty) output. Also: grep -rn "feed.cactuswatch.com" --include="*.dart" . returns at least one hit in lib/services/. Paste that.

4. Permissions stripped.
   - grep -rn "requestPermission\|NSLocationWhenInUseUsageDescription\|UIBackgroundModes\|ACCESS_FINE_LOCATION" . either returns nothing, or returns only commented-out / paid-tier-gated lines. Paste output.

5. Dart analyze clean. flutter analyze exits 0. If flutter is not on this box, run dart analyze instead. If neither is available, stop and report — Dan installs the Flutter SDK or Codex installs it via flutter_windows.zip. Do not skip this gate.

6. Branch pushed. git push origin feat/wire-cactuswatch-backend succeeds. Paste the push output. If the push is rejected because gh is logged in as the-ai-entrepreneur-ai-hub and you don't have collaborator access on dani4553/Cactus_Alert, stop — Dan adds collaborator, you retry.

7. Codemagic build triggered. Use the Codemagic API to trigger the ios-testflight workflow against feat/wire-cactuswatch-backend. Paste the build ID and the API response.

8. Codemagic build succeeded. Poll the build status until terminal. If the result is anything other than success, paste the last 100 lines of the build log and stop.

9. TestFlight processing. Hit App Store Connect API to confirm the build appears under TestFlight Internal Testing in Processing or Ready to Test state. Paste the build number and state.

AUTONOMY RULES

- Do NOT change the bundle identifier or Android applicationId.
- Do NOT add third-party dependencies. Use what pubspec.yaml already declares.
- Do NOT enable NSAllowsArbitraryLoads. Backend is HTTPS with valid public cert.
- Do NOT modify Codemagic secrets or App Store Connect credentials.
- Do NOT push to main or master. Push only to feat/wire-cactuswatch-backend.
- Commit message style: one logical change per commit, prefixes feat(ios):, fix(ios):, chore(ios):, refactor(api):. Aim for 6 to 10 commits across the run.
- Final commit before pushing should bump the Flutter version string in pubspec.yaml to the next patch version (e.g. 1.0.0 → 1.0.1) so the App Store Connect build number bump is unambiguous.

FINAL DELIVERABLE

Single markdown report titled CACTUS WATCH v1 — TESTFLIGHT SHIPPED containing:

1. Branch name + list of commits (hash + subject)
2. Files added / modified / deleted
3. The grep evidence from gates 3 and 4 (proof no Phoenix Maps URLs and no permission requests remain)
4. flutter analyze (or dart analyze) clean exit
5. Codemagic build ID, duration, status
6. App Store Connect TestFlight build number + processing state
7. Exact instruction for Dan: "Open the TestFlight app on your iPhone, accept the invite for Cactus Watch, install build N. The app should open to a list of live Phoenix incidents with the disclaimer banner at the top. Pull down to refresh. Report back if anything is missing or broken."
8. Blockers for full App Store submission (if any)

If any gate fails, replace sections 1-8 with: which gate, what the actual output was, what you tried, what stopped you. No prettifying.

Begin with gate 1. Do not edit Dart code until gates 1 and 2 both pass.
```

---

## Pre-flight Dan must do before firing this

1. Make sure `the-ai-entrepreneur-ai-hub` is a collaborator on `dani4553/Cactus_Alert` (GitHub Settings → Collaborators).
2. iPhone with TestFlight app installed and signed in to the Apple ID that's an internal tester for the app.
3. The data-quality-overhaul PR is merged so the API actually returns the shape the iOS code will expect.
