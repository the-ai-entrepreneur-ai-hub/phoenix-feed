# Codex Brief — Cactus Watch iOS 5064 (rescue the build, then re-apply 962A fix)

**Issued:** 2026-05-18 (UTC)
**Owner:** George (The AI Entrepreneur)
**Estimated duration:** 30 to 60 minutes
**Hard rule:** DO NOT trigger Codemagic. George/Claude triggers manually after this brief is complete.

---

## What happened

- Build 5061 shipped successfully (commit `3bcdf19`) with full Codex tightening.
- Build 5062 (commit `a66ff43`) added 962A and 962MC to the dispatch code table plus a regex-based family fallback. **Failed twice** at the "Build iOS IPA" step in 78 to 105 seconds, way too fast for a real AOT compile. No log content was retrievable through the Codemagic API.
- Build 5063 (commit `fdb61eb`) was a diagnostic: dispatch_codes reverted to 5061 state, with `flutter build ipa --release --verbose` and a `2>&1 | tail -200` tacked on to capture log output. Codemagic reported "success" in 75 seconds, but produced ZERO artifacts and ZERO ASC distribution. The pipe `| tail -200` swallowed the exit code (tail always returns 0), so any failure inside `flutter build ipa` is masked and the step appears green. This was George's tooling bug, not Codex's.

## Goal

Land Phoenix Fire dispatch code translation for the **962A** variant (and any unknown 962 prefix) without breaking the release-mode iOS AOT build. Two work streams:

1. **Fix the CI pipe** so flutter build failures actually fail the step and surface their log.
2. **Re-apply the 962A / 962MC / family-prefix fix** in a release-mode-safe way and verify locally before pushing.

End state: commit pushed to `feat/wire-cactuswatch-backend` ready for George to bump `BUILD_FLOOR` and trigger Codemagic.

---

## Workspace

- Repo: `D:\serveless-apps-2026\abuserOfcreativity\cactus_repo`
- Branch: `feat/wire-cactuswatch-backend`
- Local Flutter: `C:\tools\flutter\bin\flutter.bat`

## File:line targets

| File | Action |
|---|---|
| `cactus_repo/codemagic.yaml:154-158` | Remove the `2>&1 \| tail -200` suffix from the flutter build ipa command, OR add `set -o pipefail` at the top of the script step so the build exit code propagates. Keep `--verbose` so we can debug if it fails next time. |
| `cactus_repo/lib/utils/dispatch_codes.dart` | Re-implement 962A and 962MC support plus a 962-prefix family fallback, in a **release-AOT-safe** form (see implementation guidance below). |
| `cactus_repo/test/dispatch_codes_test.dart` | Update tests to cover 962A, 962MC, family fallback, raw-code fallback rejection, and the existing fields. |
| `cactus_repo/codemagic.yaml:118` | Do NOT change `BUILD_FLOOR`. Leave it as 5063. George/Claude bumps to 5064 just before triggering the next build. |

---

## Implementation guidance for `dispatch_codes.dart`

The earlier v2 attempt introduced these constructs that all pass analyzer and tests locally but seem to correlate with the IPA build hangs:

- A top-level `final RegExp _kRawCodePattern = RegExp(...)` (lazy-initialized).
- A top-level `const List<String> _kFamilyPrefixes`.
- Private helper functions `_looksLikeRawCode` and `_familyLabelFor`.

We cannot be 100 percent sure those caused the build failures (the pipe masked the real error), but the bisect-friendly move is to write the simplest possible version that still solves the bug.

**Preferred shape**: keep everything inline inside `labelForDispatchCode`. No new top-level finals. No new top-level functions. Just the existing `kDispatchCodeLabels` map plus inline logic. This minimises the diff against the working 5061 implementation.

```dart
const Map<String, String> kDispatchCodeLabels = {
  '962': 'Vehicle Crash',
  '962A': 'Vehicle Crash',
  '962X': 'Crash Requiring Extrication',
  '962BC': 'Crash Involving Bicycle',
  '962MC': 'Crash Involving Motorcycle',
  '962MV': 'Crash Involving Motor Vehicle',
  '962HZ': 'Crash with Hazmat',
  '962F': 'Crash with Fire',
  '962R': 'Crash with Rollover',
};

String labelForDispatchCode(String? code, {String? fallback}) {
  final normalizedCode = code?.trim().toUpperCase() ?? '';
  final normalizedFallback = fallback?.trim() ?? '';

  // A "raw code" fallback is one that is itself just digits with optional
  // trailing letters (e.g. "962", "962A"). Phoenix Fire sometimes sends
  // nature_desc equal to the raw code, which is useless as a display label.
  final fallbackIsRawCode = normalizedFallback.isNotEmpty &&
      RegExp(r'^\d{2,4}[A-Z]{0,4}$').hasMatch(normalizedFallback);
  final humanFallback = fallbackIsRawCode ? '' : normalizedFallback;

  if (normalizedCode.isEmpty) {
    return humanFallback;
  }

  // 1. Exact match in the table.
  final exactLabel = kDispatchCodeLabels[normalizedCode];
  if (exactLabel != null) {
    return humanFallback.isNotEmpty ? humanFallback : exactLabel;
  }

  // 2. Family prefix fallback. Any unknown 962-something becomes Vehicle Crash.
  if (normalizedCode.startsWith('962')) {
    final baseLabel = kDispatchCodeLabels['962'];
    if (baseLabel != null) {
      return humanFallback.isNotEmpty ? humanFallback : baseLabel;
    }
  }

  // 3. No match at all. Use the human fallback if present, otherwise the raw code.
  return humanFallback.isNotEmpty ? humanFallback : normalizedCode;
}
```

The `RegExp(...)` is constructed inline on every call. Slightly slower than caching at top-level, but eliminates any release-mode lazy-initialization concerns. The function is small enough that inline construction is fine.

---

## Tests to write or update

`cactus_repo/test/dispatch_codes_test.dart`:

```dart
import 'package:az_incident_alert/models/incident_model.dart';
import 'package:az_incident_alert/utils/dispatch_codes.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('962 family dispatch codes map to human labels', () {
    expect(labelForDispatchCode('962'), 'Vehicle Crash');
    expect(labelForDispatchCode('962A'), 'Vehicle Crash');
    expect(labelForDispatchCode('962X'), 'Crash Requiring Extrication');
    expect(labelForDispatchCode('962BC'), 'Crash Involving Bicycle');
    expect(labelForDispatchCode('962MC'), 'Crash Involving Motorcycle');
    expect(labelForDispatchCode('962MV'), 'Crash Involving Motor Vehicle');
    expect(labelForDispatchCode('962HZ'), 'Crash with Hazmat');
    expect(labelForDispatchCode('962F'), 'Crash with Fire');
    expect(labelForDispatchCode('962R'), 'Crash with Rollover');
  });

  test('unknown 962 variants fall back to Vehicle Crash via family prefix', () {
    expect(labelForDispatchCode('962ZZ'), 'Vehicle Crash');
    expect(labelForDispatchCode('962Q'), 'Vehicle Crash');
  });

  test('962A with raw-code fallback renders Vehicle Crash, not the raw code',
      () {
    expect(labelForDispatchCode('962A', fallback: '962'), 'Vehicle Crash');
    expect(labelForDispatchCode('962', fallback: '962'), 'Vehicle Crash');
  });

  test('human API descriptions override the mapped label', () {
    expect(
      labelForDispatchCode('962', fallback: 'Crash Involving Truck'),
      'Crash Involving Truck',
    );
  });

  test('completely unknown codes preserve raw code when no human fallback', () {
    expect(labelForDispatchCode('999XYZ'), '999XYZ');
    expect(labelForDispatchCode(''), '');
  });

  test('incident ingestion stores raw dispatch code and display label', () {
    final incident = Incident.fromJson({
      'incident_id': 'F26200443',
      'nature_code': '962',
      'nature_desc': '962',
    });

    expect(incident.natureCode, '962');
    expect(incident.displayLabel, 'Vehicle Crash');
    expect(incident.natureDesc, 'Vehicle Crash');
    expect(incident.toJson()['nature_code'], '962');
    expect(incident.toJson()['display_label'], 'Vehicle Crash');
  });

  test('incident ingestion translates 962A with raw nature_desc fallback', () {
    final incident = Incident.fromJson({
      'incident_id': 'F26200444',
      'nature_code': '962A',
      'nature_desc': '962',
    });

    expect(incident.natureCode, '962A');
    expect(incident.displayLabel, 'Vehicle Crash');
  });
}
```

---

## codemagic.yaml fix (CRITICAL)

Inside the `scripts:` section of `ios-testflight` workflow, locate the `Build iOS IPA` script step. Two acceptable fixes:

### Option A — preferred — restore unpiped output

Remove the `2>&1 | tail -200` entirely. Codemagic will surface the full log in the dashboard:

```yaml
flutter build ipa --release --verbose \
  --build-name=2.0.0 \
  --build-number=$NEXT_BUILD \
  --dart-define=CACTUSWATCH_FIRE_STATIONS_URL=https://feed.cactuswatch.com/v1/fire-stations \
  --export-options-plist=/tmp/export_options.plist
```

### Option B — keep the tail but propagate failures

Add `set -o pipefail` at the top of the script step (the line right after `set -e`):

```yaml
set -e
set -o pipefail
```

Pick Option A. Keep `--verbose` so we can grep the log if it fails again.

---

## Verification steps (run in order, do not skip)

1. **`cd D:\serveless-apps-2026\abuserOfcreativity\cactus_repo`**
2. **`C:\tools\flutter\bin\flutter.bat analyze --no-pub`** must return "No issues found".
3. **`C:\tools\flutter\bin\flutter.bat test --no-pub`** must return all tests passing (expect 188+ green).
4. **`C:\tools\flutter\bin\flutter.bat test --no-pub test/dispatch_codes_test.dart`** must return 8 dispatch tests passing.
5. **`C:\tools\flutter\bin\flutter.bat build bundle --release --no-pub`** must exit 0 (this is the closest local proxy to a release AOT compile).
6. `grep -n "tail -200" codemagic.yaml` must return nothing.
7. `grep -n "962A" lib/utils/dispatch_codes.dart` must return at least one match.
8. **`git status`** must show `codemagic.yaml`, `lib/utils/dispatch_codes.dart`, and `test/dispatch_codes_test.dart` as the only modified files.
9. Commit with message: `fix(ios): re-apply 962A translation in release-safe form; restore unpiped flutter build`. Add `change-id: cactus-watch-5064-build-rescue` trailer.
10. Push to `feat/wire-cactuswatch-backend`. Do NOT trigger Codemagic. Stop.

---

## PRESERVE (do not touch)

- `BUILD_FLOOR` value in codemagic.yaml — leave at 5063. George/Claude bumps to 5064 just before triggering the next build.
- All other steps in codemagic.yaml (ASC cert fetch, keychain, code signing, agvtool, publishing block).
- All other Dart files in cactus_repo. Only `lib/utils/dispatch_codes.dart` and `test/dispatch_codes_test.dart` change.
- The pubspec.lock dirty state is George's local drift; do NOT stage it.
- The Apple ITMS-90683 `NSLocationWhenInUseUsageDescription` string in `ios/Runner/Info.plist`.
- Firebase Crashlytics wiring.
- The 10 AppTheme enum entries and order.
- StoreKit pricing `$6.99/mo` and `$76.99/yr`.
- All other Codex deliverables from build 5061.

## Out of scope

- Triggering Codemagic. George/Claude owns it.
- Bumping `BUILD_FLOOR`. George/Claude owns it.
- Changing any UI, theme, or tile color logic.
- Touching the backend API or `phoenix-feed` repo.
- Adding new files outside the two listed.

## Rollback plan

If after George triggers, the build fails again with a visible verbose log, post the FULL log content (no tail truncation) to the conversation. The verbose mode will show the actual Dart compile or Xcode error. Codex's job ends at "pushed, verified locally, stopped".

## Output format (paste at end of run)

```
- flutter analyze --no-pub clean:    YES / NO
- flutter test --no-pub passes (total tests: ___):    YES / NO
- flutter test --no-pub test/dispatch_codes_test.dart passes (8 tests):    YES / NO
- flutter build bundle --release --no-pub exits 0:    YES / NO
- grep "tail -200" codemagic.yaml returns nothing:    YES / NO
- grep "962A" lib/utils/dispatch_codes.dart returns at least 1:    YES / NO
- dispatch_codes.dart has no new top-level finals or private functions vs the working 5061 version (inline RegExp only):    YES / NO
- commit pushed (hash):    YES / NO  hash=__________
- BUILD_FLOOR untouched at 5063:    YES / NO
- Codemagic NOT triggered:    YES / NO
- no other Dart files modified:    YES / NO
- no pubspec.lock staged:    YES / NO
```

If any line is NO, explain in 2 sentences underneath.
