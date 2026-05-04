# Executive summary

This memo answers two questions you raised about the Phoenix incident feed app:

1. Is the **free version** safe to ship under current Phoenix and Arizona law?
2. What changes are required before we can launch a **paid tier** without exposing the company to material legal risk?

The short answer to (1) is **yes, with five concrete guardrails** spelled out in section 5. The short answer to (2) is **not yet, but the gating work is small and cheap** if we sequence it correctly.

The most important thing this research surfaced is something we got wrong on the first pass: the working analogy that "this is just like a weather app pulling NOAA data" does not hold under Arizona law. Federal data is public domain by statute. Arizona municipal data is not. Phoenix retains ownership and the right to charge a market-rate fee for any commercial reuse, and Arizona's public-records statute carries a treble-damages penalty if we monetize without filing first. The penalty is real, and it reaches passive web scraping, not just formal records requests.

That sounds scarier than it is. The fix is filing a one-page commercial-purpose statement with the Phoenix City Clerk before the paid tier launches. The cost is whatever fee the city assesses, which is almost certainly small for a backend that pulls a public dashboard once a minute. We just have to do it on the front end, not the back end.

# What this memo covers

## How to read this document

This is a working memo, not a legal opinion. Every primary-source claim in here is one we pulled and verified ourselves; every citation links back to the actual page or statute. Where we cite a case or quote a statute, we have read the underlying source. Where the AI research we ran got something wrong, we say so and correct it.

The document is structured to answer your decision in order: what's the data, what's the law, can we ship the free tier today, what's the gate for the paid tier, what's left for a lawyer to confirm.

# Phoenix data sources

## System architecture

The architecture is what we already aligned on: poll the city, normalize on our backend, serve users from our own cache. Users never hit Phoenix's servers. That is the load-bearing design choice for both performance and legal posture, and we should not weaken it.

## What we pull, where it comes from, and the license attached to each

There are three Phoenix data properties that come up in any conversation about this app, and they are not interchangeable. They live on different domains, have different terms, and carry different risk. The single biggest factual error in the AI research we ran was conflating them.

| Source | URL | Real-time? | License posture |
|---|---|---|---|
| Phoenix Fire Active Incidents Dashboard | htms.phoenix.gov/publicweb (redirects to mapportal.phoenix.gov) | yes, ~60s | No published ToU; on-page disclaimer only |
| Phoenix Open Data Portal | phoenixopendata.com | no, daily batch | Custom revocable city license; "as is"; trademark restriction |
| Phoenix Public Records Search | phoenix.gov/cityclerk/.../search-public-records | no | Clickwrap: "I agree not to use these records for commercial purposes" |

We are using the **first two**. We are **not** using the third. This matters because the AI research mistakenly attributed the Public Records Search clickwrap to the Open Data portal, which created a false impression that commercial use of the open-data datasets was contractually forbidden. It is not. (See section 3 for the cleanup.)

The verbatim disclaimer attached to the Fire dashboard, which we verified directly:

<div class="verbatim">
This dashboard does not provide a complete listing of all Fire/EMS incidents. It only contains basic public information involving fire and emergency incidents, and omits incident reports involving information that is confidential by statute or common law privacy.

The City of Phoenix and the Phoenix Fire Department assume no liability for damages incurred directly or indirectly as a result of errors, omissions, or discrepancies in the information provided.
<span class="src">— htms.phoenix.gov/publicweb dashboard, on-page notice</span>
</div>

Phoenix's own disclaimer telegraphs the right design pattern for our app: city says "we omit confidential incidents and assume no liability for errors." Our app inherits both posture and risk, and we should mirror both.

## What the Open Data portal actually says

The Open Data portal terms of use, pulled verbatim from phoenixopendata.com/pages/terms-of-use:

<div class="verbatim">
The City of Phoenix ("City") grants you ("Licensee") a non-exclusive, limited and revocable rights to use, reproduce, and redistribute City data contained within City's Open Data portal (the "Data") subject to the following terms...

City of Phoenix trademarks and copyrighted materials, including any confusingly similar variants, may not be used in association with Data.

City maintains all title to, ownership of, and interest in and all Data.

Data is provided on an 'as is' and 'as available' basis...

City reserves the right to alter and/or no longer provide Data at any time without prior notice.

The laws of the State of Arizona will govern these disclaimers, terms and conditions.
<span class="src">— phoenixopendata.com/pages/terms-of-use</span>
</div>

A few non-obvious takeaways from that text:

- It is a **custom revocable license**, not a standard open-data license like ODC-By or CC-BY. The PRD asserted otherwise; the PRD was wrong.
- It does **not explicitly forbid commercial use**, but it also does not expressly grant it. That silence matters under section 4 below.
- The trademark restriction is hard. We cannot use Phoenix Fire's logo, the city seal, or any "confusingly similar" iconography in our UI.
- The "revocable" language gives the city a unilateral kill switch. If they ever decide they don't like our app, they can revoke access and force a source switch. The product needs a feature flag for source switching from day one.

# The legal trigger you need to know about

## Arizona Revised Statutes § 39-121.03

This is the single statute that drives the free-versus-paid gating decision. The full text is at `azleg.gov/ars/39/00121-03.htm`. The operative parts:

**Subsection (D), the trigger:**

<div class="verbatim">
"Commercial purpose" means the use of a public record for the purpose of sale or resale or for the purpose of producing a document containing all or part of the copy, printout or photograph for sale, or the obtaining of names and addresses from public records for the purpose of solicitation, or the sale of names and addresses to another for the purpose of solicitation, or for any purpose in which the purchaser can reasonably anticipate the receipt of monetary gain from the direct or indirect use of the public record.
<span class="src">— ARS § 39-121.03(D)</span>
</div>

**Subsection (A), the fee mechanic:**

<div class="verbatim">
Upon being furnished the [commercial-purpose] statement the custodian of such records may furnish reproductions, the charge for which shall include the following:
1. A portion of the cost to the public body for obtaining the original or copies of the documents...
2. A reasonable fee for the cost of time, materials, equipment and personnel...
3. The value of the reproduction on the commercial market as best determined by the public body.
<span class="src">— ARS § 39-121.03(A)</span>
</div>

**Subsection (C), the penalty:**

<div class="verbatim">
A person who obtains a public record for a commercial purpose without indicating the commercial purpose, or who obtains a public record for a noncommercial purpose and uses or knowingly allows the use of such public record for a commercial purpose, or who obtains a public record from anyone other than the custodian of such records and uses it for a commercial purpose shall, in addition to other penalties, be liable to the state or the political subdivision from which the public record was obtained for damages in the amount of three times the amount which would have been charged for the public record had the commercial purpose been stated, plus costs and reasonable attorney fees.
<span class="src">— ARS § 39-121.03(C)</span>
</div>

The reason this matters for us, in plain English: subsection (C) explicitly catches anyone who "obtains a public record from anyone other than the custodian of such records and uses it for a commercial purpose." That phrase reaches scraping. We do not have to file a formal records request to be on the hook. Pulling the Phoenix Fire dashboard and selling access to a transformed copy of it is enough to trigger the statute.

## What we got wrong on the first pass

Before we ran this research, we were operating on three assumptions that turn out to be wrong or misleading. Worth naming them so we don't backslide.

| Assumption | Reality |
|---|---|
| "Phoenix municipal data is like NOAA data — public domain." | False. Federal works are public domain by 17 USC § 105. Arizona state and local government works are not. The city retains title and can charge market-value fees for commercial reuse. |
| "ARS § 39-121.03 only applies if you formally request records from a custodian." | False. Subsection (C) reaches commercial use of records "obtained from anyone other than the custodian," which captures web scraping. |
| "The Open Data portal is licensed under ODC-By or CC-BY." | False. It's a custom city license, revocable, with explicit trademark restrictions and a reservation of all title. |

## What the AI research got wrong, for the record

Two specific things we should not propagate from the research output:

- The "I agree not to use these records for commercial purposes" clickwrap was attributed to the Open Data portal. That clickwrap exists, but it lives on a different city property — the **City Clerk's Public Records Search**, used for ordinances and council documents. It does not apply to incident data and we are not using that portal.
- The research cited *Mount v. PulsePoint* as a relevant comparable. The case is real, but it is against PulsePoint Inc., the **digital advertising company** sued in the Second Circuit over Safari cookie circumvention. It is not the PulsePoint Foundation that runs the public-safety dispatch app. Different company, same brand. The case does not tell us anything useful about public-safety data risk.

# Free version: ship with these guardrails

We can ship the free tier now. Federal CFAA case law is on our side: *hiQ v. LinkedIn* (9th Cir. 2022) and *Van Buren v. United States* (S.Ct. 2021) jointly establish that polling a publicly accessible, unauthenticated endpoint is not "without authorization" under the federal anti-hacking statute. Even if Phoenix sends a cease-and-desist letter, it cannot turn a public-website fetch into a federal crime.

State exposure on the free tier is also limited because we are not extracting commercial value yet, so § 39-121.03 does not bite.

What we **must** have in place before launch, all of which is small and tractable:

1. **Polling cadence.** Default 60 seconds, configurable up to 300. No tighter without a specific reason. We are a guest on a city server; aggressive polling is the fastest way to invite a technical block or a "trespass to chattels" theory of liability.

2. **Persistent disclaimer banner.** Every screen showing incident data carries a visible "not for emergency use; call 911" line. This is the single highest-leverage liability mitigation in the product. We model this on language used by every comparable service.

3. **Capped EULA.** "As is" and "as available" warranty disclaimer; explicit waiver of liability for bodily harm, death, or property damage; aggregate liability cap of $100. This is the language Citizen and PulsePoint use. We do the same.

4. **PII redaction layer.** The normalizer strips exact street numbers, names, and any free-text descriptions before anything is written to our cache. Phoenix's feed already does most of this, but accidents happen on their side and our redaction is the second line.

5. **No Phoenix marks in the UI.** No Phoenix Fire logo, no city seal, no language that implies official endorsement. The Open Data ToU is explicit about this and we respect it whether or not we ever use that source.

# Paid version: gate before launch

Do not launch the paid tier without these four things resolved. None of them is hard; they're sequence questions, not blockers.

## P1. File a commercial-purpose statement

This is the cheap defense. Under § 39-121.03(A) we send the Phoenix City Clerk a written statement disclosing that we intend to make commercial use of incident data, and the city assesses a fee. The fee will include some portion of "the value of the reproduction on the commercial market." For a once-a-minute pull of an already-public dashboard, that figure will almost certainly be modest. The exact number is the answer to the email we should send the City Clerk this week.

The reason to file this even if we think the fee will be small is that filing converts our § 39-121.03(C) treble-damages exposure into a § 39-121.03(A) one-times fee. That is a one-decimal-place change in worst-case downside.

## P2. Resolve the ad-revenue question

The hardest question in the whole memo, and the one we should put to a lawyer in writing: **does ad revenue on the "free" tier flip us into commercial purpose under subsection (D)?** The statutory text reaches "any purpose in which the purchaser can reasonably anticipate the receipt of monetary gain from the direct or indirect use of the public record." Reasonable people can read that two ways. We need a written opinion before we monetize the free tier with anything other than donations.

## P3. Counsel-reviewed terms of service

The current draft EULA needs a real lawyer pass before we charge anyone. Specifically: the liability cap, the assumption-of-risk language, the governing-law clause, and the indemnification mirror.

## P4. Source-switching escape hatch

The Open Data portal terms reserve the city's right to "no longer provide Data at any time without prior notice." That sentence is a product requirement, not just a legal note. The ingestion pipeline must be able to swap sources behind the API contract with a config change, not a redeploy. If Phoenix pulls the plug, we don't want to be down for a week.

# What comparable services teach us

Four peers, four different strategies, one useful pattern.

| Service | Data path | Posture |
|---|---|---|
| PulsePoint Respond | Direct API contracts with dispatch agencies | Slow to scale, zero scraping risk |
| Citizen | Mixed scraping + crowdsourcing | Survives because it is venture-funded; lots of bad press |
| Watch Duty | Volunteer human curation, NWS feeds | 501(c)(3) nonprofit; sidesteps commercial-use statutes by design |
| Broadcastify | Crowdsourced public-safety radio audio | Different legal regime entirely (Communications Act); audio-only |

We are closest in shape to early Citizen. That is fine for the free tier and risky for the paid tier. The path that scales without legal drama is the PulsePoint path: a written arrangement with the city. We do not need a full PulsePoint-style integration agreement to ship the paid tier. The § 39-121.03(A) commercial-purpose filing is a much lighter version of the same posture and it gets us legitimacy without slowing the roadmap.

# Open questions for counsel

These are the four questions worth one paid hour with an Arizona lawyer. Bring them in writing.

1. Does ad revenue on a free tier qualify as "commercial purpose" under ARS § 39-121.03(D)? If yes, when?
2. What is the formula or precedent the City of Phoenix uses to assess "value of the reproduction on the commercial market" for a continuous data feed under § 39-121.03(A)?
3. If we mathematically aggregate Phoenix incident data with NWS weather, OSM map data, and crowdsourced reports, at what point does the resulting product become a "transformative derivative work" rather than a redistribution?
4. What state-specific assumption-of-risk language do we need in the EULA to insulate us from gross-negligence and wrongful-death claims if the cache fails during a major incident?

# What I need from you (Dan)

Three decisions, in order of urgency.

1. **Disclaimer language sign-off.** I'll send a one-page draft of the in-app banner and the EULA carve-out by end of week. Approve or redline.
2. **City Clerk fee budget.** Confirm we can absorb whatever § 39-121.03(A) fee Phoenix assesses, up to a number you set. If the fee comes back below that ceiling, I file and we move. Above it, we pause and reassess.
3. **Lawyer consult.** Half an hour, Arizona-licensed, on the four questions in section 8. I can find someone or use yours; your call.

Everything else is engineering and is on me.

# Sources

All URLs verified the week of 2026-05-03.

- Phoenix Open Data Terms of Use — phoenixopendata.com/pages/terms-of-use
- Phoenix Fire Active Incidents Dashboard — htms.phoenix.gov/publicweb (redirects to mapportal.phoenix.gov)
- Arizona Revised Statutes § 39-121.03 — azleg.gov/ars/39/00121-03.htm
- *hiQ Labs, Inc. v. LinkedIn Corp.*, 9th Cir. 2022
- *Van Buren v. United States*, 593 U.S. 374 (2021)
- *CDK Global LLC v. Brnovich*, No. 20-16469 (9th Cir. Oct. 25, 2021)
- 17 U.S.C. § 105 (federal works in the public domain)
- Phoenix City Clerk Public Records Search — phoenix.gov/administration/departments/cityclerk/programs-services/search-public-records.html (cited only to clarify it does *not* apply to our use case)
