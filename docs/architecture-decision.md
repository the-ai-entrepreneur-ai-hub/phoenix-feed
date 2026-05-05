# Position

Do not buy the GPU workstation. Ship Phoenix dashboard mode now, file the commercial-purpose statement before taking paid subscriptions, and spend the next 30 days proving strangers will pay $4.99 per month. Whisper is not the launch bottleneck. The hardware buy unlocks nothing until Cactus Watch has 100 paying subscribers retained for one full billing cycle.

This brief replaces version 1.0, which framed the question as a procurement decision for a funded company. The actual question is what Dan should spend before the first customer pays. The answer is **$199 maximum**.

# Where Dan stands today

The DigitalOcean droplet `cactus-watch-prod` at 64.23.147.218 is live, polling Phoenix Fire's ArcGIS endpoint every 60 seconds, and serving cached JSON. The iOS app can begin hitting it today. One real incident has already been tracked through its full lifecycle in production data. **The launch backend is not the bottleneck.**

LEDYZ (the Dell Inspiron 3910 in Dan's office) is running ProScan, SDR-Trunk, SDRplay, VB-Audio Virtual Cable, and a Python Whisper script that produces paired `.wav` and `.transcript.json` files in `D:\cactus\recordings\`. Two real problems exist:

1. Every recording is named `..._unknown.wav` — RadioReference Premium is paid for but the talkgroup database is not wired into SDR-Trunk preferences. This is a 5-minute fix.
2. Transcripts on disk do not yet flow to the DigitalOcean droplet. There is no API endpoint for them and no publisher running. This is a 1-day build, deferrable.

Cumulative subscribers: zero. Pre-revenue. No marketing list, no landing page, no Apple TestFlight invite issued.

# Counter-recommendation

If this were my money, I would spend $0 on GPU hardware, $0 on cloud GPU, and $0 on multi-city work this month.

The product to ship is Phoenix dashboard mode only: live map, list, freshness state, attribution, and the "not for emergency use" copy. The backend already supports that path. The compliance memo says the paid tier needs a commercial-purpose statement before launch, so file it before charging.

Cash this week is capped at **$199**: $80 for a 32GB DDR4-3200 desktop UDIMM kit, $20 for a domain, $99 for the Apple Developer Program if Dan has not already paid it. No workstation belongs on this week's purchase order.

Sell at $4.99 per month. Twenty paying users is $99.80 gross and proves only the payment plumbing works. One hundred paying users is $499 gross per month and proves the channel has a pulse. Three hundred thirty users is $1,646.70 gross per month, which makes a $1,280 workstation a 0.78-month gross payback instead of a leap of faith.

Whisper is a paid-upgrade story, not the v1 promise. LEDYZ can keep recording and labeling experiments while the app proves whether civilians care. The hardware trigger is simple: buy a prebuilt tower with an RTX 4060 Ti 16GB card and a 650W PSU only after **100 paid subscribers survive one full billing cycle**, OR **25 beta users explicitly refuse to pay without transcription**.

# What to skip or defer

- Skip the $1,200 workstation now. Revisit after 100 paid subscribers retained for 30 days.
- Skip multi-city expansion (Mesa, Tucson, Tempe). Revisit after Phoenix mode clears 12 percent day-30 retention among paying users — a realistic consumer-app threshold, not enterprise SaaS.
- Skip Whisper `large-v3` and fine-tuning. Revisit after users ask for transcript search in support messages, not founder imagination.
- Skip cloud GPU. Revisit only for a one-week batch experiment with a fixed budget under $75.
- Skip paid history, geofences, and notifications. Revisit after the basic feed converts at least 1.5 percent of landing-page visitors to trial starts.

# This weekend's action plan

| # | Action | Effort | Cash | If Dan does NOT do it | How Dan knows it worked |
|---|---|---|---|---|---|
| 1 | Buy a 32GB DDR4-3200 UDIMM desktop kit (Crucial Ballistix, Corsair Vengeance LPX, G.Skill Ripjaws V — make sure it says **UDIMM 288-pin**, NOT SODIMM, since LEDYZ is a desktop). Inspiron 3910 supports up to 64GB. | 1 hour | $80 | LEDYZ keeps running scanner + Java + Whisper at 2.5GB free; one Windows update spawns a memory crisis | SDR-Trunk + ProScan + Whisper run 24 hours with no pagefile climb |
| 2 | Configure RadioReference talkgroup import in SDR-Trunk preferences (Premium subscription is already paid) | 2 hours | $0 | Every recording stays `_unknown`; transcripts have no agency labels and no product value | New filenames contain real talkgroup labels (e.g. `PHX_Fire_North_Disp`) |
| 3 | Buy `cactuswatch.com` or `.app` from Namecheap or Porkbun | 15 minutes | $12-20 | Squatter takes the obvious domain after the rebrand becomes public | DNS resolves to a parked page Dan controls |
| 4 | Ship Phoenix dashboard-only iOS app to TestFlight under the Cactus Watch brand | 8 hours | $99 if Apple Dev not paid, else $0 | Dan keeps building architecture before testing demand | 20 testers install it, 5 reopen it the next day |
| 5 | Stand up a one-page landing site (Carrd, Webflow, or static HTML on the droplet) and a $4.99/month offer with Stripe Checkout. Do not charge cards until the commercial-purpose statement is filed with Phoenix. | 3 hours | $0 (domain already bought above) | Subscriber math stays fantasy and pricing remains untested | 30 qualified visitors produce 5 price-aware signups (waitlist or pre-orders) |

Total weekend cash outlay: **$191-199**. Total weekend hours: **~14**.

# Trigger to revisit hardware

Resume this brief — and re-evaluate the $1,200 workstation buy — when one of these is true:

1. **100 paid subscribers** retained through one full $4.99 billing cycle (30 days). At that point gross MRR is approximately $499 and the workstation pays back in under 3 months. Revisit the architecture diagrams in Appendix C.
2. **25 explicit refusals** from beta users who cite "no audio transcripts" or "missing dispatch detail" as the deal-breaker. At that point you have evidence customers want what the GPU buys, not a hypothesis.
3. **Phoenix removes or restricts the public ArcGIS endpoint** (HTTP 403 sustained, ToS change, or shutdown). At that point the Whisper pipeline is the only data path, and the workstation goes from "premature" to "urgent" overnight.

Until one of these fires, the workstation is premature optimization regardless of how clean the eventual architecture diagram looks.

---

# Appendix A — Verified production state (DigitalOcean)

Probed via HTTP on 2026-05-05.

```
GET http://64.23.147.218/v1/health

{
  "state": "ok",
  "db_reachable": true,
  "server_time": "2026-05-05T08:06:38Z",
  "sources": {
    "phoenix-fire-mapserver": {
      "last_success_at": "2026-05-05T08:05:46Z",
      "seconds_since_success": 51,
      "parser_version": "phx-fire-2026-05",
      "canary": { "passed": true, "drift": null }
    }
  }
}
```

Recent ingester logs (excerpt):

```
"poll applied" source=phoenix-fire-mapserver observed=0 cleared=0 latency_ms=49 poll_id=84
"poll applied" source=phoenix-fire-mapserver observed=0 cleared=0 latency_ms=42 poll_id=83
"poll applied" source=phoenix-fire-mapserver observed=0 cleared=1 latency_ms=50 poll_id=81
"incidents cleared" source=phoenix-fire-mapserver count=1 ids=[F26200227]
```

Droplet spec: 2 vCPU, 2 GB RAM, 60 GB SSD, sfo3 region, weekly backups, **$21.60 per month** all-in.

# Appendix B — Verified LEDYZ state

Probed via SSH over the ngrok TCP tunnel on 2026-05-05.

| Component | Spec |
|---|---|
| Model | Dell Inspiron 3910, service tag GHB39R3 |
| OS | Windows 11 Home, build 26200 |
| CPU | Intel Core i5-12400, 6P/12T, 2.5 GHz base, 4.4 GHz boost |
| RAM | 11.7 GB usable (8 + 4 GB at 3200 MHz, mismatched) |
| GPU | Intel UHD Graphics 730 integrated only |
| Storage C: | 218 GB KIOXIA NVMe, 140 GB free |
| Storage D: | 931 GB Seagate ST1000DM010 7200 rpm |
| Free RAM at probe | 2.5 GB |
| Pagefile peak | 518 MB |

Active processes:

| Process | RAM | Cumulative CPU | Live CPU | Purpose |
|---|---|---|---|---|
| `java` (SDR-Trunk) | 1.2 GB | 27 hours | ~134% of one core | SDR decoder + audio source |
| `python` (cactus_transcribe.py) | 1.2 GB | 31 hours | ~26% of one core | Whisper transcription on CPU |
| `ProScan` | 21 MB | low | ~10% | Uniden scanner control |
| `sdrplay_apiservice` | low | live | ~50% | SDRplay USB driver |

Audio routing: Cirrus Logic codec + USB Audio Device (SDS100) + VB-Audio Virtual Cable. Output: paired `.wav` and `.transcript.json` files in `D:\cactus\recordings\` produced in real time but tagged `_unknown` because RadioReference is not wired in.

# Appendix C — Eventual end-state architecture (post-revenue trigger only)

This appendix exists for the day the revisit trigger fires. **Do not act on it now.**

The end-state architecture has three independent tiers:

1. **LEDYZ — radio brain.** Keeps doing exactly what it does today: ProScan, SDR-Trunk (with RadioReference talkgroups), SDRplay USB, audio capture. Add a small Python audio relay that streams Opus-compressed audio over the local LAN to the workstation.

2. **New workstation — AI brain (~$1,200).** Prebuilt tower with NVIDIA RTX 4060 Ti 16 GB and a 650 W PSU. Examples in this price band: Lenovo Legion T5 26IRB8 configured with 4060 Ti; HP Omen 25L GT16-2000; NZXT Player Two with 4060 Ti. Runs `faster-whisper large-v3` continuously, takes audio over the LAN, posts finished transcripts to the droplet over HTTPS.

3. **DigitalOcean droplet — serving tier.** No change from today. Add one new endpoint: `POST /v1/transcripts` accepting transcript JSON from the workstation. Postgres + PostGIS already supports the schema.

Failure isolation: each tier is independent and replaceable. Audio never leaves Dan's house except as finished transcripts, which are tiny.

The "current vs end-state" architecture diagrams from version 1.0 of this brief are still on disk at `pdf-build/svg/10-current-architecture.svg`, `11-recommended-architecture.svg`, and `12-decision-tree.svg`. They are correct depictions of the eventual target. They are not, today, a plan.
