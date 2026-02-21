# CHANGELOG.md - Prompt File Change Log

_Tracks every change to the workspace prompt files so we know exactly what shifted and why._

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

### Files NOT changed this round
- **SOUL.md** — No changes
- **VOICE.md** — No changes
- **KNOWLEDGE.md** — No changes
- **USER.md** — No changes
- **IDENTITY.md** — No changes
