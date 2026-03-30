# CHANGELOG.md - Prompt File Change Log

_Tracks every change to the workspace prompt files so we know exactly what shifted and why._

---

## 2026-03-30

### Source: Off-grid appointment overlap bug (booking failure on available slots)

Call transcript showed agent offering 8:30 AM on May 13 for Dr. Bach, but all 3 booking attempts failed with AMD 4101 ("overlaps existing appointment"). Root cause: a 15-min appointment at 8:45 (booked when Bach had 15-min intervals) sat between the 30-min grid lines. The overlap check only tested one direction — whether the slot start fell inside an existing appointment — but not whether the booking's duration would extend into an appointment starting later.

---

### handlers.go + availability.go — Bidirectional overlap check

**Fixed: `hasOverlappingAppointment` now checks both overlap directions**
- Was: `slotStart ∈ [apptStart, apptEnd)` — only caught appointments that started before the slot
- Now: `slotStart < apptEnd AND apptStart < slotEnd` — catches any overlap between the booking range and existing appointments
- Function now accepts `slotDuration` parameter to define the booking footprint
- Applied to both server handler (`internal/http/handlers.go`) and CLI (`cmd/cli/availability.go`)
- Added 3 test cases for off-grid scenarios (8:45 blocking 8:30, 9:15 blocking 9:00, 8:45 NOT blocking 8:00)

**Result on May 13 for Dr. Bach:**
- Before: 10 available slots, first at 8:30 AM (2 phantom slots that would fail to book)
- After: 8 available slots, first at 10:30 AM (all slots genuinely bookable)

---

### CLAUDE.md — Updated Bach interval + AMD 4101 description

- Bach interval corrected: 15 min → 30 min (changed in AMD since last audit on 2026-02-19)
- AMD 4101 description updated to reflect bidirectional overlap behavior

---

## 2026-03-16

### Source: Transfer resistance + "you are the office" language fix

Two changes based on 210-call deflection analysis: 43 transfers/wk were callers asking for a human who actually needed scheduling help, and the agent's language ("transfer you to the office") made it sound like a remote call center instead of the front desk.

---

### TOOLS.md — "Understand Why They're Calling"

**Added: "They ask for a human or say 'transfer me'" intent**
- New bullet between "Someone told them to call back" and "They want to know if their insurance is accepted"
- Agent asks what they're calling about before transferring, framed as routing: "I just want to make sure I get you to the right person — what are you calling about?"
- If the caller describes something in the agent's wheelhouse (scheduling, confirming, cancelling, rescheduling, insurance, general info), agent offers to handle it
- If they insist or it's outside scope, transfer without pushback

---

### TOOLS.md — transfer_to_number (rewritten)

**Changed: From "transfer immediately" to "triage first"**
- Was: "Don't overthink it. Don't make the caller justify why they need a human."
- Now: Four-case flow:
  1. Ask what they're calling about (framed as routing)
  2. If in wheelhouse → offer to handle ("I can actually take care of that for you right now")
  3. If they insist or ask twice → transfer promptly, no resistance
  4. If genuinely outside scope → transfer without the offer

---

### TOOLS.md + SOUL.md — "to the office" language replaced

**Changed: 11 instances in TOOLS.md, 2 in SOUL.md**
- Was: "transfer you to the office," "get you over to the office"
- Now: "connect you with someone here," "get someone here to help," "one of your coworkers"
- Agent lives at the office — it IS the front desk, not a remote call center
- SOUL.md reinforced: "You ARE the office"

---

### Files NOT changed this round
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-03-15

### Source: Prompt hardening — Dr. Bach expectations, insurance pre-check

Two changes based on observed agent behavior: agent not setting expectations about Dr. Bach's limited availability (leading to caller confusion when dates are far out), and running callers through the full scheduling flow before telling them their insurance isn't accepted.

---

### TOOLS.md — "Understand Why They're Calling"

**Added: Insurance question intent**
- New bullet between "Someone told them to call back" and "They have a general question"
- If caller asks "do you accept [insurance]?" — answer immediately from the accepted insurance list in add_patient
- If recognized, confirm and ask if they'd like to schedule
- If not recognized, say you're not sure and offer to transfer
- Don't make them go through verify/register just to find out — answer the insurance question first

---

### TOOLS.md — get_availability

**Added: Dr. Bach limited schedule context**
- Sub-bullet under step 1 (ask when they'd like to come in)
- Dr. Bach only works at Spring Hill a couple of times per month and is usually booked
- Agent should set expectations early for patients who need Bach (pediatric, strabismus, double vision)
- Suggested phrasing: "Dr. Bach has a limited schedule at this location, so it may be a couple weeks out — let me see what's available."
- Don't be surprised if the system auto-searches forward many days

---

### KNOWLEDGE.md — Dr. Bach provider section

**Added: Limited schedule note**
- "Dr. Bach is only at the Spring Hill office a couple of times per month. Availability may be several weeks out."
- Gives the agent knowledge-base context to answer questions about Bach's schedule even outside the scheduling flow

---

### SOUL.md — Date/time context (manually added)

**Added: Dynamic date and time variables**
- `The current date is {{current_date}} and the current time is {{current_time}}. Use this information for any relative date calculations.`
- Added below the opening identity paragraph
- Gives the agent real-time date/time context so relative date math ("next Thursday," "tomorrow") is accurate
- Uses ElevenLabs dynamic variable syntax — values are injected at runtime

---

### Files NOT changed this round
- **VOICE.md** — No changes

---

---

## 2026-03-13

### Source: Reschedule flow + cancel noshowreasonid fix

Agent can now handle rescheduling directly instead of transferring to the office. Rescheduling uses existing tools in sequence: verify → confirm_appt → get_availability → book_appt → cancel_appt. Books the new appointment before cancelling the old one to protect against leaving the patient with no appointment.

Also fixed cancel_appt: removed `noshowreasonid` from the AMD REST request body — AMD returns a 500 when it's included.

---

### TOOLS.md — New "Rescheduling" section

**Added: Reschedule flow documentation**
- Not a new tool — chains existing tools: verify → confirm_appt → get_availability → book_appt → cancel_appt
- Key safety rule: book new appointment FIRST, then cancel old one
- If new booking fails, original appointment stays intact
- If cancel fails after booking, agent tells caller new appointment is booked and offers to transfer for cleanup
- Confirmation phrasing: "I've moved your appointment to [new date] at [new time] with [doctor]"

**Changed: "Understand Why They're Calling" intent routing**
- Reschedule is no longer a transfer
- Was: "They want to reschedule → Transfer immediately"
- Now: "They want to reschedule → verify → confirm_appt → get_availability → book_appt → cancel_appt flow"

**Changed: cancel_appt section**
- Step 7 now mentions rescheduling is available if caller wants it after cancelling

**Changed: transfer_to_number description**
- Removed "rescheduling" and "cancellations" from the list of transfer reasons

---

### SOUL.md — Boundaries

**Changed: "Stay in your lane" updated to include rescheduling**
- Was: "You schedule appointments, verify patients, register new ones, confirm existing appointments, and cancel appointments"
- Now: "You schedule appointments, verify patients, register new ones, confirm existing appointments, cancel appointments, and reschedule appointments"
- Removed "If someone needs to reschedule an appointment, offer to transfer them to the office"

---

### Code change: cancel_appt noshowreasonid removed

**Fixed: AMD 500 error on cancel**
- `noshowreasonid` field removed from cancel request body in `advancedmd_rest.go`
- AMD returns HTTP 500 when `noshowreasonid` is included; works fine with just `{"id": appointmentID}`
- `cancelNoshowReasonID` constant removed
- Test updated to remove noshowreasonid assertion
- README updated to remove "hardcoded no-show reason ID (23)" reference

---

### Files NOT changed this round
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-03-11 (Round 2)

### Source: New cancel appointment tool

Patients calling to cancel an appointment can now be handled by the agent instead of being transferred. Uses AMD's REST PUT /scheduler/appointments/{id}/cancel endpoint. Extends the confirm_appt flow by including the appointment ID in responses.

---

### TOOLS.md — New `cancel_appt` tool section

**Added: cancel_appt tool**
- Flow: verify patient → confirm_appt (get appointments + IDs) → identify which to cancel → confirm with caller → cancel_appt
- Input: appointmentId (from confirm_appt response `id` field)
- Output: status (cancelled / error), appointmentId, message
- Agent confirms before cancelling: "Just to confirm, you'd like to cancel your appointment on [date] at [time] with [doctor]?"
- Agent confirms after cancelling: "Your appointment has been cancelled."
- Failure handling: retry once silently, then offer transfer

**Changed: Tool count updated from seven to eight**

**Changed: "Understand Why They're Calling" section**
- Cancel is no longer a transfer — it's now a handled flow
- Was: "They want to reschedule or cancel → Transfer immediately"
- Now: "They want to cancel → verify → confirm_appt → cancel_appt flow" / "They want to reschedule → Transfer immediately"

---

### SOUL.md — Boundaries

**Changed: "Stay in your lane" updated to include appointment cancellation**
- Was: "You schedule appointments, verify patients, register new ones, and confirm existing appointments"
- Now: "You schedule appointments, verify patients, register new ones, confirm existing appointments, and cancel appointments"
- Reschedule remains transfer-only

---

### Files NOT changed this round
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-03-11

### Source: New confirm appointment tool

Patients calling to confirm their appointment can now be handled by the agent instead of being transferred. Uses the AMD REST scheduler/appointments endpoint with forView=month to fetch all appointments across allowed columns, then filters by patient ID server-side.

---

### TOOLS.md — New `confirm_appt` tool section

**Added: confirm_appt tool**
- Two-step flow: verify patient first (reuses existing verify_patient), then call confirm_appt with patientId
- Server searches next 60 days automatically — no date input needed from caller
- Returns upcoming appointments with date, time, provider, type, facility, and confirmed status
- Agent reads back appointment details and gets verbal confirmation
- Handles no-appointments case (offer to schedule or transfer)
- Handles multiple appointments (read nearest first, don't list all at once)

**Changed: Tool count updated from six to seven**

**Changed: "Understand Why They're Calling" section**
- Confirm appointment is no longer a transfer — it's now a handled flow
- Was: "They want to reschedule, cancel, or confirm → Transfer immediately"
- Now: "They want to confirm → verify → confirm_appt flow" / "They want to reschedule or cancel → Transfer immediately"

---

### SOUL.md — Boundaries

**Changed: "Stay in your lane" updated to include appointment confirmation**
- Was: "You don't reschedule, cancel, or confirm existing appointments"
- Now: "You schedule appointments, verify patients, register new ones, and confirm existing appointments"
- Reschedule and cancel remain transfer-only

---

### Files NOT changed this round
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-03-10

### Source: Preauthorization requirement for HMO/managed care plans

8 insurance plans require preauth before scheduling. Agent needs to inform the patient and pass the flag to get_availability so the server enforces a 14-day minimum.

---

### TOOLS.md — add_patient

**Added: Preauth insurance list and response handling**
- Listed 8 preauth plans: Humana Gold Plus, Humana Medicaid, United Healthcare HMO, Aetna HMO, Florida Blue Medicare HMO, Cigna HMO, Tricare Prime, Tricare Forever
- `preauthRequired` added to "What comes back" section
- Agent script: "Your insurance requires a preauthorization before we can see you, so the earliest we can schedule is about two weeks out."

---

### TOOLS.md — get_availability

**Added: `preauthRequired` parameter**
- New optional param: pass `true` when add_patient returned `preauthRequired: true`
- Server auto-advances search date to 14 days out if too soon

---

### TOOLS.md — Insurance list updates

**Added:** Aetna HMO, United Healthcare HMO, Florida Blue Medicare HMO, Tricare Forever, BCBS Medicare HMO guidance
**Changed:** Humana Gold → Humana Gold Plus

---

### Source: Production bug — common last names not found in verify_patient

Patients with common last names (e.g., "Gonzalez") were returning `not_found` because AMD paginates lookuppatient results (50 per page) and the middleware only read page 1. Sending `"LastName,FirstName"` in `@name` lets AMD filter server-side.

---

### TOOLS.md — verify_patient

**Changed: First name now required (was optional)**
- Agent now asks caller to spell first name before last name
- Both are spelled back letter by letter and confirmed before proceeding
- "What you send" section updated: `firstName` changed from optional to required
- Rationale: Without first name, common last names return 1000+ paginated results and the patient is never found

---

## 2026-03-09

### Source: Production edge cases — parent callers, same-day booking, accented names, subscriber IDs, date shifting

Multiple prompt hardening updates based on real agent behavior: agent didn't clarify who the appointment was for when a parent called, sent "TBD" for subscriber ID, offered same-day slots, and didn't explain when the system auto-advanced to a later date.

---

### TOOLS.md — New "Identify the Patient" section

**Added: Parent-calling-for-child handling**
- New section between "First: Understand Why They're Calling" and "General Rules"
- If someone says "I need an appointment for my son/daughter" — the patient is the child, not the caller
- If unclear, agent asks: "Is this appointment for you or for someone else?"
- All collected info (name, DOB, insurance) must be for the patient, not the caller
- Gentle redirect if caller gives their own info: "And what's your child's name? That's who I'll need to look up."

---

### TOOLS.md — verify_patient

**Added: Two-last-name guidance**
- Sub-bullet under the last name spelling step
- Some patients have two last names (e.g., "Lopez Sanchez") — send both
- If lookup fails, retry with just the first last name since some records may only have one

---

### TOOLS.md — add_patient

**Added: No-placeholder rule for subscriber ID**
- Sub-bullet under step 11 (subscriber/member ID)
- Never send "TBD," "N/A," or any placeholder — the field requires the real number
- If caller doesn't have their card, offer to hold while they grab it
- If they can't get it, offer to transfer to the office to finish registration

**Added: Pre-submit readback of key details**
- Before calling the tool, agent reads back name, DOB, email, and address in one conversational pass
- Waits for confirmation before submitting
- Keeps it natural, not a 13-item checklist

---

### TOOLS.md — get_availability

**Added: No same-day appointments rule**
- Sub-bullet under step 1 (ask when they'd like to come in)
- If caller asks for today: "We're not able to book same-day appointments — the earliest I can look is tomorrow."
- Don't call the tool with today's date (middleware also enforces this server-side)

**Added: Date-shifted explanation (step 4)**
- New step between "Call the tool" and "Pick one slot"
- Response has `searchedDate` (what was requested) and `date` (what came back)
- If they differ, the requested date had no availability and the system found the next open day
- Agent must tell the caller: "I don't have anything available on [requested date], but the next opening is [returned date]"
- Don't skip this — caller needs to know the date changed before being offered a slot

---

### Files NOT changed this round
- **SOUL.md** — No changes
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-03-05

### Source: Production call review — agent offering callbacks and phone numbers

Agent was telling callers to "call the office" or that "someone will reach out," but the agent IS the office phone line. There is no callback functionality, no alternative number, and no outbound follow-up. The only escalation path is transferring the call to a human.

---

### SOUL.md — Boundaries

**Added: "You are the office phone line"**
- New rule at the top of the Boundaries section
- When someone calls Abita Eye Care, they reach the agent — there is no separate number to give
- No callback option, no "someone will reach out," no alternative phone number
- Only escalation: transfer the call to a human at the office
- "Never tell a caller to 'call the office' — they already did."

---

### TOOLS.md — General Rules

**Fixed: Tool failure fallback language**
- Was: "offer a different option or let them know the office will follow up"
- Now: "offer a different option or to transfer them to the office"

### TOOLS.md — book_appt

**Fixed: Booking failure fallback language**
- Was: "I can have the office call you back to confirm"
- Now: "I can transfer you to the office"

---

### Files NOT changed this round
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes (phone number kept as reference data, but agent should never speak it to callers)

---

---

## 2026-03-04

### Source: Insurance list audit (PDF rev 9.4.2025 vs insurance.go)

Compared the Abita Insurance List PDF against insurance.go. All routing rules were correct. Found 16 plans in the code but missing from the TOOLS.md prompt. Restructured the prompt insurance section from a flat list to network-grouped format so the agent has context about which names to send.

---

### TOOLS.md — add_patient insurance section

**Changed: Replaced flat 54-name list with network-grouped 70-name format**
- Was: single comma-separated list of 54 insurance names with "send exactly one of these"
- Now: names grouped by carrier network (Aetna, Aetna/iCare, Ambetter/Envolve, Cigna, Cigna/Humana, Florida Blue, iCare, Molina, Oscar, Tricare, United Healthcare, Standalone)
- Each group includes agent guidance for shorthand mapping (e.g., "If patient says 'Oscar,' send 'Oscar Health'")
- Molina group has explicit MUST-ask rule — agent must ask which Molina plan (Medicaid, Medicare, or Marketplace)
- Aetna EPO has follow-up rule — ask North Broward or University of Miami
- Added 16 missing plan names: Aetna Healthy Kids, Aetna QHP Individual Exchange, Ambetter Select, Ambetter Value, Children's Medical Services, Cigna Miami-Dade Public Schools, Cigna Open Access, Florida Blue Medicare PPO, Florida Blue PPO Federal Employee, Florida Blue PPO Out of State, Florida Community Care, Medicaid, Miami Children's Health Plan, Staywell Medicare, Sunshine Medicaid, Vivida

**Why:** Agent was sending shorthand like "Oscar" instead of "Oscar Health", causing insurance attachment failures. The grouped format gives the agent context about which names belong together and when shorthand is OK vs when it needs to be specific.

---

### Files NOT changed this round
- **SOUL.md** — No changes
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-03-02

### Source: Production review — full middleware walkthrough before go-live

---

### Workspace structure changes

**Deleted: `workspace.go`**
- The `go:embed` loader was dead code — prompt files are managed directly in ElevenLabs, not loaded as dynamic variables
- `Variables()` was also broken: it referenced `IDENTITY.md` and `USER.md` which no longer existed, causing it to error on every call and load zero prompt files
- Removing the loader has no impact — prompts were never being sent via dynamic variables

**Deleted: `files/` subdirectory**
- Prompt files moved from `internal/workspace/files/` to `internal/workspace/`
- Flatter structure, no Go code in the directory

**Removed: `IDENTITY.md` and `USER.md` references**
- These files no longer exist in the repo
- Were still referenced in the (now-deleted) `workspace.go` mapping

**Added: `KNOWLEDGE.md` tracking**
- File existed but was never included in the `workspace.go` mapping
- Now tracked alongside other prompt files in the workspace directory

### Files NOT changed this round
- **SOUL.md** — No changes
- **TOOLS.md** — No changes
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes

---

---

## 2026-02-19

### Source: Transcript review of 4 most recent calls
- **Transcript 1** (296s, Chase + Kyle) — Two successful bookings, but multi-field asks caused confusion, agent couldn't calculate "next Thursday," misspelled Kyle's last name
- **Transcript 2** (18s) — Caller hung up after "Are you a new or existing patient?" question
- **Transcript 3** (166s) — Missed letter in spelled name, asked caller to provide full date for "this Wednesday," booking tool failed with zero recovery ("please try again" then call ended)
- **Transcript 4** (5s) — Immediate hangup during greeting, no data

---

### SOUL.md

**Fixed typo: "schedueling" → "scheduling"**
- Line 28 in Vibe section
- Was misspelled since the original file

**Removed "Do the math yourself" rule (moved to TOOLS.md)**
- Originally added to Core Truths section during this session
- Relocated because date calculation is a tool-level behavior, not a soul-level personality trait
- User directed this move

---

### TOOLS.md — Full Rewrite

**Why:** The original TOOLS.md read like API documentation — robotic, overly formal, and had leftover scaffolding text ("Here's your add_patient tool formatted to match the structure:"). The agent was following it literally, leading to stiff multi-field asks and no recovery when tools failed.

**What changed:**

1. **Added "General Rules" section at the top** — five rules that apply across all tools:
   - One question at a time (never batch fields)
   - Echo before you search (read back spelled names before looking them up)
   - Do the math yourself (calculate relative dates like "next Thursday" — never ask the caller)
   - If a tool fails, retry silently once, then offer alternatives (never dead-end)
   - Auth is automatic (no need to mention headers/tokens per tool)

2. **verify_patient:**
   - Fixed header typo: "verfiy_patient" → "verify_patient"
   - Removed "Always call this tool first before any other patient-related operation" (now implied by section ordering and flow)
   - Added recovery guidance: "If they're not found, ask if the spelling was right. If it was, pivot to add_patient."
   - Changed from asking all three fields (first, last, DOB) to asking one at a time (last name first, then DOB, first name optional)

3. **add_patient:**
   - Removed leftover scaffolding block ("Here's your add_patient tool formatted to match the structure:" / name / description / homepage)
   - Rewrote collection order from robotic instructions ("Ask the patient to spell out their **first name**") to natural conversational prompts ("Can you spell your first name for me?")
   - Kept all 12 fields and their order intact
   - Kept the spell-confirm rule for first name, last name, and email
   - Removed redundant auth/timeout notes (now covered by General Rules)

4. **get_availability:**
   - Added explicit date calculation rule with an example of what to say: "So that'd be Wednesday, February 25th. Let me see what's open."
   - Combined the Notes section into the flow for readability
   - Added "Don't" summary at bottom for quick reference
   - Kept all existing rules: one slot at a time, no doctor selection, reuse results before re-calling

5. **book_appt:**
   - Removed leftover scaffolding block
   - Rewrote pre-booking confirmation from a bullet list to a natural example: "So that's Wednesday, February 25th at eight fifteen AM with Dr. Bach at the Spring Hill office — does that sound right?"
   - Added explicit tool failure recovery: retry once silently, then offer alternatives or office callback
   - Removed the dead-end "Please try again" pattern that caused Transcript 3's failure
   - Kept all field requirements and the rule about values coming from get_availability

6. **Tone throughout:**
   - Replaced formal headers ("Collection order (ask one at a time)") with conversational ones ("How the conversation should flow:")
   - Removed repeated auth/timeout boilerplate from each tool section
   - Added natural phrasing examples where helpful

---

### USER.md

**Fixed typo: "appointemnts" → "appointments"**
- Line 19 in Context section
- Was misspelled since the original file

---

---

## 2026-02-19 (Round 2)

### Source: Analysis of most recent call (548s, Chase + Margaret Test)
- Two successful bookings, one new patient registration
- Agent calculated relative dates correctly (new rules working)
- Single-slot presentation and one-sentence confirmations working
- Found 4 remaining issues from transcript analysis

---

### TOOLS.md — General Rules additions

**Added: "Never say data formats out loud"**
- Agent said "your date of birth please in MM/DD/YYYY format" to a caller at @30s
- Format hints like MM/DD/YYYY are internal instructions, not spoken text
- New rule tells agent to ask naturally ("what's your date of birth?") and convert silently

**Added: "Numbers in tool calls are digits, not words"**
- When caller said "one two three Hickory Lane," agent should send `123 Hickory Lane` in the API request, not the spelled-out version
- Applies to addresses, zip codes, phone numbers, subscriber IDs — any number going into a tool call

**Clarified: Echo rule now explicit about direction**
- Agent was asking callers to re-spell names they'd already given (@213s: "Can you spell her last name for me again just to be sure" → caller: "I already gave it to you.")
- Updated rule to emphasize: *you* read it back to confirm, don't ask *them* to repeat. "So that's T-E-S-T?" not "Can you spell that again?"

---

### VOICE.md

**Added: Natural grammar rule**
- "You speak with natural grammar — mostly lowercase, using capitalization only for specific emphasis on time, dates, or critical details."
- Added to Core Truths section after the ellipsis rule
- Ensures spoken output doesn't sound over-capitalized or formal

---

### Files NOT changed this round
- **SOUL.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes

---

---

## 2026-02-20

### Source: Call review (Kyle + Chase post-call debrief)
- Agent verbally stated the patient ID after verification — internal system data exposed to the caller
- Agent ignored a patient question about follow-up instructions mid-booking and steamrolled through confirmation flow

---

### TOOLS.md — General Rules addition

**Added: "Internal data stays internal"**
- Agent read the patient ID aloud after verifying a caller
- Patient IDs, system IDs, column IDs, profile IDs are for tool usage only — never spoken, referenced, or hinted at
- Agent may confirm identity naturally ("I found you in our system") but must never read back any ID
- New rule added to General Rules alongside existing "don't say X aloud" rules

---

### TOOLS.md — verify_patient return values

**Tightened: `patient_id` description now explicitly forbids speaking it**
- Changed from: "You need this for everything after."
- Changed to: "You need this for every tool call after. **Never say this to the caller.** Confirm identity naturally: 'I found you in our system.'"
- Reinforces the general rule at the exact point the agent first encounters the value

---

### TOOLS.md — add_patient return values

**Tightened: `patient_id` description now explicitly forbids speaking it**
- Changed from: "from `patientId`"
- Changed to: "from `patientId`. **Never say this to the caller** — it's for tool calls only."
- Same reinforcement as verify_patient, for consistency

---

### TOOLS.md — book_appt confirmation flow

**Added: Conversational interrupt rule during confirmation**
- Agent ignored a patient question about follow-up instructions and continued confirming the appointment
- New rule: if the patient asks a question during confirmation, pause and answer it first, then circle back to confirmation
- Includes example phrasing: "and just to confirm, that's Wednesday at eleven AM with Dr. Bach at the Spring Hill office — sound good?"

---

### SOUL.md — Core Truths addition

**Added: "Never steamroll past a question"**
- Added after "Match the caller's energy" in Core Truths
- If the caller asks something — even mid-booking, even during confirmation — stop and answer it first
- The workflow can wait; the person can't
- After addressing the question, pick up where you left off
- This is a soul-level behavior: the agent should always prioritize the person over the process

---

### Files NOT changed this round
- **VOICE.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes

---

---

## 2026-02-20 (Round 2)

### Source: Call review continued (Kyle + Chase)
- Agent told a caller it could change their insurance, then tried and failed
- Agent couldn't answer questions outside the knowledge base and had no fallback

---

### KNOWLEDGE.md — New file

**Added: ElevenLabs knowledge base tracked in repo**
- Abita Eye Clinic – Spring Hill scheduling and general knowledge base
- Covers: emergency notice, urgency triage questions, location/contact info, hours, services, providers (Dr. Bach, Dr. Noel, Dr. Licht), glasses warranty, insurance/referrals, what to bring, appointment expectations, payment, scope limitations
- This file is the source of truth for the ElevenLabs RAG knowledge base — changes here should be manually synced to ElevenLabs

---

### TOOLS.md — New "What You Can't Do" section

**Added: Explicit capability boundaries**
- Agent told a caller it could change their insurance, then tried and failed — no guardrail existed
- New section between General Rules and verify_patient explicitly lists what the agent cannot do
- Only actions with tools are allowed: verify patient, register patient, check availability, book appointment
- For anything else: "I'm not able to do that from my end, but I can transfer you to someone who can help."
- Hard rule: "If you're unsure whether you can do something, you can't."

---

### SOUL.md — Boundaries additions

**Added: Unknown answer fallback protocol**
- Two new rules in the Boundaries section
- If the answer isn't in the knowledge base, don't guess — offer to transfer to someone who can help
- "Never fabricate an answer to sound helpful. Honest uncertainty beats confident misinformation."
- Prevents the agent from making up answers to sound competent

---

### VOICE.md — New pacing section

**Added: "Vary Your Pace and Speed"**
- New section after Expressive Tags
- Use pacing tags like `[faster]`, `[slow]`, `[quick]` to sound more realistic
- Real people don't speak at one constant speed — vary pace naturally throughout the conversation

---

### Files NOT changed this round
- **USER.md** — No changes
- **IDENTITY.md** — No changes

---

---

## 2026-02-20 (Round 3)

### Source: Transcript review of 8 most recent calls
- **Call 1** (200s, Chase) — Successful booking but: multi-field ask, dumped multiple doctor/slot options, confirmed after booking instead of before, sent 2027 date instead of 2026, broke character when asked personal questions
- **Call 2** (98s) — Clean booking, skipped echo on last name
- **Call 3** (8s) — Immediate hangup, double [warmly] tag in greeting
- **Call 4** (253s) — Strong call, good boundary enforcement and knowledge base usage
- **Call 5** (41s) — Caller disconnected, clean handling
- **Call 6** (29s) — verify_patient failed, agent told caller immediately instead of retrying silently
- **Call 7** (47s) — verify_patient failed, no silent retry, conflated tool error with "patient not found" and suggested registration
- **Call 8** (156s) — Clean booking but offered to update insurance ("I can also update it for you now") then had to walk it back

---

### TOOLS.md — General Rules

**Added concrete example to "One question at a time"**
- Agent still batching last name + DOB in a single ask (Call 1 @0:08)
- Added Bad/Good example to make the rule unmissable

**Added: "When a date hasn't passed yet this year, use the current year"**
- Agent sent 2027-04-08 when caller said "April 8th" during a February 2026 call (Call 1 @1:04)
- April 8, 2026 hadn't passed yet — should have been 2026-04-08

---

### TOOLS.md — verify_patient

**Added: Explicit tool error vs. not-found distinction**
- Agent treated verify_patient errors as "patient not found" and suggested new patient registration (Call 7 @0:40)
- Patient was confirmed to exist in other calls — the tool itself was failing
- New guidance: retry silently on error, only suggest registration when tool succeeds but returns no match

---

### TOOLS.md — get_availability

**Added: Rejected slot handling rule**
- Agent listed multiple doctors and their available times side-by-side (Call 1 @1:41: "Dr. Bach has 4:45, Dr. Noel has 4:00")
- New rule: if a slot is rejected, suggest one different time — never compare two doctors' availability

---

### TOOLS.md — book_appt

**Added: Slot offer vs. full confirmation are two separate steps**
- Agent treated "Sure, let's do it" (response to a slot offer) as full consent and booked without the complete readback (Call 1 @2:09-2:13)
- New rule: slot offer gets interest, full confirmation (date + time + doctor + location) gets consent — never skip the full confirmation

---

### SOUL.md — Boundaries

**Added: Stay in character on personal questions**
- Agent said "I am a conversational agent and do not have a salary" and "As an AI, I don't experience emotions" (Call 1 @2:52, @3:07)
- New rule: deflect naturally and steer back to the task, don't break character

---

### Files NOT changed this round
- **VOICE.md** — No changes (greeting double-tag is an ElevenLabs first-message config issue, not a prompt file issue)
- **KNOWLEDGE.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes

---

---

## 2026-02-20 (Round 4)

### Source: Agent behavior review (Chase)
- Agent was making multiple tool calls without waiting for responses
- Agent wasn't asking existing patients what kind of appointment (follow-up vs post-op)
- Appointment type was hardcoded to id 13 instead of using correct AMD type IDs
- No distinction between adult and pediatric appointment types

---

### TOOLS.md — General Rules

**Added: "One tool call at a time"**
- Agent was batching tool calls or continuing conversation before receiving tool results
- New rule: call a tool, wait for the response, then decide your next step
- Never assume what a tool will return or plan ahead while a tool is running

---

### TOOLS.md — New "Determine Appointment Type" section

**Added: Decision step between add_patient and get_availability**
- Agent needs to determine the correct appointment type before checking availability
- Uses DOB (already collected) to calculate age silently — never asks the patient their age
- New patient: type is automatic based on age (1006 adult / 1004 pediatric)
- Existing patient: agent asks "is this a follow-up visit or a post-op visit?"
  - Follow-up: 1007 adult / 1005 pediatric based on age
  - Post-op: 1008 regardless of age
- Agent holds the type id for use when booking

---

### TOOLS.md — book_appt

**Changed: Dynamic appointment type instead of hardcoded id 13**
- Was: `type` (array) — always `[{ "id": 13 }]`
- Now: `type` (array) — `[{ "id": <appointment_type_id> }]` using the type id determined earlier (1004, 1005, 1006, 1007, or 1008)
- Maps to AMD appointment types:
  - 1004 = NEW PEDIATRIC MEDICAL
  - 1005 = ESTABLISH PEDIATRIC MED
  - 1006 = NEW ADULT MEDICAL
  - 1007 = ESTABLISH ADULT MEDICAL
  - 1008 = POST OP

---

### TOOLS.md — get_availability + book_appt

**Changed: Merged slot offer and booking confirmation into one step**
- Agent was double-confirming: first a vague slot offer ("How about 2:30 with Dr. Bach?"), then a full confirmation readback ("So that's Wednesday February 25th at 2:30 with Dr. Bach at Spring Hill — sound right?")
- Now: the slot offer itself includes full details (date, time, doctor, location). If the caller says yes, that's consent — book immediately without repeating
- Removed the "two different steps" rule from book_appt that enforced the double-ask
- Updated get_availability step 5 to include full details in the offer
- Updated book_appt to clarify: "The slot offer IS the confirmation"

---

### VOICE.md — New "Before a Tool Call" section

**Added: Spoken transition phrases before each tool call**
- Agent was going silent while tools ran — no verbal signal to the caller
- New section with natural phrases mapped to each tool:
  - verify_patient: "one moment while I pull up your chart"
  - add_patient: "ok, one moment while I get you set up"
  - get_availability: "let me check what's available"
  - book_appt: "one moment while I get that booked for you"
- Two example phrases per tool for variety
- Rule: keep it to one short sentence, then let the tool run

---

### TOOLS.md — General Rules

**Added: "Always ask callers to spell their name" as a top-level rule**
- Agent was still not asking callers to spell names despite per-tool guidance
- Promoted from tool-specific instructions to a hard general rule with Bad/Good examples
- Applies to both first and last name, every time, no exceptions
- Clarified the two-step process: ask them to spell it, then YOU read it back letter by letter
- Separated from the "echo before you search" rule to make the spelling ask unmissable

---

### TOOLS.md — General Rules

**Added: "Always verify or register before scheduling"**
- Agent could skip straight to checking availability without verifying the patient
- New hard rule: no patient ID, no schedule lookup

---

### TOOLS.md — verify_patient

**Rewrote: Collection flow with explicit spelling instructions**
- Was: "Last name — have them spell it, echo it back" (vague note)
- Now: Explicit conversation flow with exact phrasing: "Can you spell your last name for me?"
- Added DO NOT skip warnings — agent must ask for spelling and confirm before searching

---

### TOOLS.md — Appointment Types

**Changed: Hardcode type id 13 for development**
- Commented out the full appointment type matrix (1004-1008)
- Using type id 13 for all appointments until ready to go live with dynamic types

---

### VOICE.md — Expressive Tags

**Commented out: Expressive tags section**
- Disabled `[warmly]`, `[checking]`, `[focused]`, etc. tags
- Section preserved in HTML comments for future re-enablement

---

### VOICE.md — Vary Your Pace and Speed

**Changed: Removed tag syntax**
- Was: "Use pacing tags like `[faster]`, `[slow]`, and `[quick]`"
- Now: "Use pacing and pausing to make yourself sound more realistic"
- Removed specific tag references, kept the natural pacing guidance

---

### Files NOT changed this round
- **SOUL.md** — No changes
- **KNOWLEDGE.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes

---

---

## 2026-02-22

### Source: Kyle UX review — agent spelling and confirmation pacing

- Agent spells names back too fast — letters blur together and callers can't follow
- Agent rushes to the next question after confirming a name without waiting for the caller to actually say yes

---

### VOICE.md

**Added: "Slow down slightly when spelling back"**
- Agent was spelling names back too fast for callers to follow
- Light rule: ease up on speed when spelling, and wait for confirmation before moving on
- Kept it brief — one line, not a whole section

**Changed: Renamed "Vibe" section to "Phrasing"**
- SOUL.md already has a "Vibe" section covering personality and rhythm
- VOICE.md's "Vibe" was about phrasing style (simple words, no corporate filler)
- Renamed to "Phrasing" to avoid duplicate section names across files

---

### TOOLS.md — verify_patient

**Added: Explicit "Wait" step after spelling back last name**
- Flow was: spell back → immediately ask DOB
- Now: spell back → **step 2: Wait** → only proceed after caller confirms
- Includes gentle prompt if caller is silent: "does that look right?"

**Added: Explicit wait step after spelling back**
- Kept original spelling example as-is (no forced pauses)
- Added step 2: wait for caller to confirm before asking DOB

---

### TOOLS.md — add_patient

**Changed: Steps 1 and 2 now include explicit wait-for-confirmation**
- Was: "spell your first name" → "echo it back" → next field
- Now: "spell your first name" → spell it back → **wait for them to confirm** → next field
- Same change for last name (step 2)

**Changed: Bottom "Important" note reinforced**
- Added "wait for confirmation before moving to the next field" to the existing spell-confirm reminder

---

### SOUL.md — Redundancy cleanup

**Changed: Trimmed "Match the caller's energy" line**
- Removed "Calm with calm. Direct with direct." — VOICE.md now owns all pacing details
- Kept: "Warm when warmth is offered. Don't overpower the conversation."

**Removed: "Move at their rhythm" from Vibe section**
- Was: "Move at their rhythm. If they pause, pause. If they speak slowly, match their pace. Calm cadence builds trust."
- Removed entirely — VOICE.md's "Match the caller's speed" section already covers this
- Avoids the agent receiving the same instruction from two different files with slightly different wording

---

### Files NOT changed this round
- **KNOWLEDGE.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes

---

---

## 2026-02-24

### Source: Insurance crosswalk implementation (INSURANCE_CROSSWALK.md)

Replaced the generic 7-carrier map (test-environment IDs) with 44 plan-specific entries using live Spring Hill carrier IDs, and added server-side insurance routing that restricts which providers a patient can see based on their insurance plan.

---

### TOOLS.md — verify_patient

**Added: Insurance routing fields in response**
- `routing` — the routing rule (`all_three`, `bach_only`, `bach_licht`, `not_accepted`)
- `allowedProviders` — display names of doctors this patient can see
- `routingAmbiguous` — if true, carrier ID is shared across plans; agent should ask clarifying question
- If `routing` is `not_accepted`, agent must tell patient immediately and not proceed to scheduling

---

### TOOLS.md — add_patient

**Changed: `carrierId` field replaced with `insurance`**
- Was: `carrierId` (string) — one of 4 generic carriers (`cigna`, `blue cross blue shield`, `aetna`, `medicare`)
- Now: `insurance` (string) — one of 44 specific plan names from the insurance crosswalk
- Added full list of accepted insurance names inline in TOOLS.md
- Response now includes `routing` and `allowedProviders` fields
- If insurance is `not_accepted`, patient is created but routing is rejected with a clear message

---

### TOOLS.md — get_availability

**Added: `routing` parameter**
- Optional parameter passed through from verify/add-patient response
- Server uses it to filter which doctors' slots are returned (enforced server-side)
- If `routing` is `not_accepted`, agent must NOT call this tool — tell the patient immediately

---

### TOOLS.md — Appointment Types

**Changed: Activated live appointment type IDs**
- Removed hardcoded type id `13` override and HTML comment block
- Uncommented full appointment type matrix:
  - New Adult Medical → 1006
  - New Pediatric Medical → 1004
  - Established Adult Medical → 1007
  - Established Pediatric Medical → 1005
  - Post Op → 1008
- Agent now determines type from patient age (DOB) and visit reason

---

### Files NOT changed this round
- **SOUL.md** — No changes
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes
