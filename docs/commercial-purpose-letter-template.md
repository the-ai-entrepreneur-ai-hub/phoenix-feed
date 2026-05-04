# Commercial Purpose Statement Template

**Filed under A.R.S. § 39-121.03(A)**
**Reusable boilerplate — only the bracketed fields change per agency.**

Fields to swap per agency are wrapped in `[[ ]]`. Everything else stays as is.

---

```
[[YOUR COMPANY LEGAL NAME]]
[[STREET ADDRESS]]
[[CITY, STATE ZIP]]
[[EMAIL]]
[[PHONE]]

[[DATE]]

[[CUSTODIAN NAME OR TITLE — e.g., "Records Custodian"]]
[[AGENCY LEGAL NAME — e.g., "City of Phoenix Fire Department"]]
[[AGENCY ADDRESS LINE 1]]
[[AGENCY ADDRESS LINE 2]]

Re: Verified Statement of Commercial Purpose Pursuant to A.R.S. § 39-121.03(A)
    Records Sought: [[BRIEF NAME — e.g., "Active Incident Dispatch Data"]]


Dear [[CUSTODIAN NAME OR "Records Custodian"]]:

Pursuant to Arizona Revised Statutes § 39-121.03(A), this letter
constitutes [[YOUR COMPANY LEGAL NAME]]'s verified statement of the
commercial purpose for which the public records described below will
be used, and a request that the [[AGENCY SHORT NAME]] furnish such
records and assess any applicable fees.

1. REQUESTING PARTY

   [[YOUR COMPANY LEGAL NAME]], a [[STATE OF FORMATION]] [[ENTITY TYPE
   — e.g., "limited liability company"]], with principal place of
   business at [[FULL ADDRESS]]. The undersigned, [[YOUR NAME]], is
   authorized to make this request on behalf of the company.

2. RECORDS REQUESTED

   We request continuous access to the following public records held
   or made available by [[AGENCY SHORT NAME]]:

   [[DESCRIBE THE RECORDS — be specific. For Phoenix Fire example:
   "Active incident dispatch records published via the Phoenix Fire
   Department Active Incidents Dashboard at htms.phoenix.gov/publicweb,
   including incident type codes, geocoded locations at the level of
   precision the City currently publishes, dispatch timestamps, and
   incident status updates, on a recurring polling basis of one
   request per sixty (60) seconds."]]

3. STATEMENT OF COMMERCIAL PURPOSE (A.R.S. § 39-121.03(A), (D))

   [[YOUR COMPANY]] intends to use the records described in
   Paragraph 2 for the following commercial purpose:

   [[YOUR COMPANY]] operates a software application that ingests the
   above records, normalizes and caches them on its own backend
   infrastructure, and exposes them to end users via a proprietary
   web and mobile interface. End users access the application
   through (i) a free tier that is offered without charge,
   advertisements, or other monetization, and (ii) a paid
   subscription tier offering value-added features including but not
   limited to push notifications, geofence-based alerts, mapping,
   filtering, historical analytics, and a developer API.

   The records themselves are not sold, resold, or licensed to third
   parties as a standalone dataset; rather, end users may indirectly
   benefit from the records through the application's interface. We
   acknowledge that this constitutes a "commercial purpose" within
   the meaning of A.R.S. § 39-121.03(D), and we make this filing to
   satisfy the disclosure requirement of § 39-121.03(A) accordingly.

4. ACKNOWLEDGMENT OF FEES

   [[YOUR COMPANY]] acknowledges that the [[AGENCY SHORT NAME]] is
   authorized under § 39-121.03(A) to charge a fee comprising:

      (1) a portion of the cost of producing the records;
      (2) a reasonable fee for time, materials, equipment, and
          personnel; and
      (3) the value of the reproduction on the commercial market as
          determined by the public body.

   Please provide a written assessment of the fee applicable to the
   records and access pattern described in Paragraph 2. Upon receipt
   we will, in good faith, either (a) remit the assessed fee in full
   prior to commencing commercial use, or (b) advise [[AGENCY SHORT
   NAME]] in writing that we will not proceed with the commercial
   use described herein.

5. ATTRIBUTION & TRADEMARKS

   [[YOUR COMPANY]] will not use the [[AGENCY SHORT NAME]]'s
   trademarks, logos, seals, or any confusingly similar variants in
   association with the data, and will not represent itself as an
   official channel or partner of [[AGENCY SHORT NAME]] absent a
   separate written agreement.

6. CONTACT

   Please direct your fee assessment, requests for clarification, or
   any conditions on access to:

      [[YOUR NAME]]
      [[EMAIL]]
      [[PHONE]]

   We are happy to provide additional detail on access patterns,
   technical architecture, or end-user disclosures upon request.

7. VERIFICATION

   I, [[YOUR NAME]], being duly authorized to make this statement on
   behalf of [[YOUR COMPANY]], declare under penalty of perjury under
   the laws of the State of Arizona that the foregoing statement of
   commercial purpose is true and correct to the best of my
   knowledge.

   Executed on [[DATE]] at [[CITY, STATE]].


   _______________________________
   [[YOUR NAME]]
   [[YOUR TITLE]]
   [[YOUR COMPANY LEGAL NAME]]
```

---

## Per-agency variants you'll need

| Agency | Custodian / address |
|---|---|
| **Phoenix Fire Department** | City of Phoenix Public Records Office, 200 W. Washington St., Phoenix, AZ 85003 — re: Phoenix Fire Active Incidents Dashboard |
| **Mesa Fire & Medical Department** | City of Mesa Public Records, 20 E. Main St., Mesa, AZ 85201 — note: no public dashboard, so framed differently (see below) |
| **Tucson Fire Department (City)** | City of Tucson Records, 255 W. Alameda St., Tucson, AZ 85701 |
| **Northwest Fire District (Tucson area, second dispatch center)** | Northwest Fire District Records, 5125 W. Camino de Fuego, Tucson, AZ 85743 |
| **Arizona Department of Transportation (ADOT) — for VistaScan / AZ511** | ADOT Public Records Officer, 206 S. 17th Ave., Phoenix, AZ 85007 — *but try the AZ511 developer API key registration first; that may already cover commercial use under a separate developer agreement* |

## Mesa-specific note

Mesa has no public incident dashboard, so the records request paragraph (Paragraph 2) needs to be reframed as a request for access to active dispatch incident summaries through whatever mechanism Mesa makes available, including but not limited to a CAD data feed, public-records-portal export, or written confirmation that the same data we receive via SDR scanner of public-safety frequencies is records-of-record-equivalent. Worth a phone call to their records officer before sending the letter to find out what posture they prefer.

## Process & expected timeline

1. **Send certified mail** with return receipt — paper trail matters.
2. Cc the email address on the letter so there's an electronic record.
3. Most Arizona agencies respond within 10 to 30 days under § 39-121.01.E (which sets a "promptly" standard).
4. Their response will either: (a) assess a fee, (b) ask clarifying questions, (c) deny the request, or (d) refer you to a different custodian.
5. Until you receive a written fee assessment AND pay it, do not begin commercial monetization of records data from that agency.

## What you do NOT need from them

- A signed contract.
- A formal license agreement.
- An exclusive partnership.
- Their endorsement of your product.

You only need: their fee assessment in writing, and proof you paid it.
