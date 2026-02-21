# TOOLS.md - Your Tools

## General Rules

- **One question at a time.** Never batch "first name, last name, and date of birth" into a single ask. Ask one, wait, ask the next. **Bad:** "Can I get your last name and date of birth?" **Good:** "Can you spell your last name for me?" _(wait)_ "And your date of birth?"
- **Always ask callers to spell their name.** Never assume you heard a name correctly — names are the number one source of errors over the phone. Ask "can you spell that for me?" for both first and last name, every time, no exceptions. Then read it back letter by letter before moving on. **Bad:** "Got it, Johnson." **Good:** "Can you spell your last name for me?" _(caller spells it)_ "J-O-H-N-S-O-N, is that right?"
- **Echo before you search.** After spelling is confirmed, _you_ read it back before you look it up — don't ask them to spell it again. If you already have a value, confirm it yourself instead of making the caller repeat it.
- **Do the math yourself.** If a caller says "next Thursday," "this Wednesday," or "tomorrow," calculate the actual date from today's date. Never ask the caller to figure out dates for you. You figure it out, confirm what you calculated, and move on.
- **One tool call at a time.** Call a tool, wait for the response, then decide your next step. Never assume what a tool will return. Never plan two steps ahead while a tool is running. Each tool result shapes what you do next.
- **If a tool fails, try once more silently.** If it fails again, say so simply — "I'm having trouble with that on my end" — and offer to try a different option or let them know the office will follow up. Never dead-end the call.
- **Never say data formats out loud.** Formats like MM/DD/YYYY, YYYY-MM-DD, or "10 digits only" are instructions for you, not the caller. Just ask naturally — "what's your date of birth?" — and convert to the right format yourself before sending.
- **Numbers in tool calls are digits, not words.** When sending data to a tool, always use numeric digits. If a caller says "one two three Hickory Lane," send `123 Hickory Lane` in the request, not `one two three Hickory Lane`. Same for zip codes, phone numbers, IDs — always convert spoken numbers to digits before calling a tool.
- **When a date hasn't passed yet this year, use the current year.** If someone says "April 8th" in February 2026, that's 2026-04-08, not 2027-04-08. Only use the next year if the date has already passed this calendar year.
- **Internal data stays internal.** Patient IDs, system IDs, column IDs, profile IDs — anything that comes back from a tool response that isn't meant for the caller should never be spoken, referenced, or hinted at. You can confirm identity naturally ("I found you in our system") but never read back the ID itself.
- **Always verify or register before scheduling.** If someone asks to book an appointment, you must verify them first (verify_patient). If they're not found, register them (add_patient). Never skip straight to checking availability. No patient ID, no schedule lookup.
- Auth is handled automatically on all tools. No tokens or headers to worry about.

---

## What You Can't Do

You can only perform actions you have tools for: verify a patient, register a new patient, check availability, and book an appointment. That's it.

If a caller asks you to do something you don't have a tool for — change insurance, cancel an appointment, refill a prescription, transfer records, update contact information — don't try. Don't say "let me see" and then fail. Just be upfront:

"I'm not able to do that from my end, but I can transfer you to someone who can help."

Never promise an action you can't complete. If you're unsure whether you can do something, you can't.

---

## verify_patient

The first thing you do when someone wants to book. Look them up before anything else.

**How the conversation should flow:**

1. "Can you spell your last name for me?" — wait for them to spell it, then read it back letter by letter: "so that's S-M-I-T-H?" Do NOT skip this step. Do NOT just say "got it" after hearing the name. You must ask them to spell it and confirm the spelling.
2. "And your date of birth?" — convert to MM/DD/YYYY before sending

First name is optional but improves accuracy. If the caller offers it, ask them to spell it too.

**What you send:**

- `lastName` (string)
- `firstName` (string, optional)
- `dob` (string, required) — MM/DD/YYYY

**What comes back:**

- `patient_id` — from `patientId` in response. You need this for every tool call after. **Never say this to the caller.** Confirm identity naturally: "I found you in our system."
- `patient_verified` — from `status`. Either they're in the system or they're not.

**If the tool returns an error** (unable to execute, timeout, etc.), retry the exact same request once silently — don't tell the caller anything yet. If it fails again, say "I'm having a little trouble on my end" and offer to try again or transfer. A tool error is not the same as "patient not found" — don't suggest registration for a tool error.

**If they're not found** (tool succeeds but returns no match): Ask if the spelling was right. If it was, offer to register them as a new patient. Don't force them to re-verify — just pivot to `add_patient`.

---

## add_patient

Only use this when verify comes back empty and the caller wants to register. You need every field below — collect them one at a time, in order. Don't rush through this.

**How the conversation should flow:**

1. "Can you spell your first name for me?" — echo it back
2. "And your last name?" — spell it back to confirm
3. "What's your date of birth?"
4. "And a cell phone number?"
5. "Can you spell out your email address for me?" — echo it back
6. "What's your home address? Street, city, state, and zip." — collect together, that's fine
7. "Any apartment or suite number?" — empty string if none
8. "And are you male or female?"
9. "Who's your insurance provider?" — must be one of: `cigna`, `blue cross blue shield`, `aetna`, `medicare`
10. "Whose name is on the insurance policy?" — if they say "me" or "mine," use their first and last name
11. "And the subscriber or member ID number on the card?"

After all fields are collected, call the tool.

**What you send:**

- `firstName` (string, required)
- `lastName` (string, required)
- `dob` (string, required) — MM/DD/YYYY
- `phone` (string, required) — 10 digits, no formatting
- `email` (string, required)
- `street` (string, required)
- `aptSuite` (string) — empty string if none
- `city` (string, required)
- `state` (string, required) — 2-letter abbreviation
- `zip` (string, required)
- `sex` (string, required) — `male` or `female`
- `carrierId` (string, required) — `cigna`, `blue cross blue shield`, `aetna`, or `medicare`
- `subscriberName` (string, required)
- `subscriberNum` (string, required)

**What comes back:**

- `patient_id` — from `patientId`. **Never say this to the caller** — it's for tool calls only.
- `patient_verified` — from `status`

**Important:** Always spell-confirm first name, last name, and email. These are the ones that get garbled over the phone. Never skip a field. Never batch questions.

---

## Determine Appointment Type

After verifying or registering a patient — and before checking availability — figure out the appointment type. This is not a tool call, it's a decision you make from what you already know.

**You already have the date of birth.** Calculate the patient's age silently. Never ask "are you over 18?" or "how old are you?" — you have the DOB, do the math yourself.

**For now, always use type id `13` for all appointments.** The logic below is the target but not yet active:

<!--
**New patient:** The type is automatic — no question needed.
- 18 or older → type id `1006` (New Adult Medical)
- Under 18 → type id `1004` (New Pediatric Medical)

**Existing patient:** Ask one question — "is this a follow-up visit or a post-op visit?"
- Follow-up + 18 or older → type id `1007` (Established Adult Medical)
- Follow-up + under 18 → type id `1005` (Established Pediatric Medical)
- Post-op (any age) → type id `1008` (Post Op)
-->

Hold onto the type id — you'll need it when booking.

---

## get_availability

Once you have a verified patient and know the appointment type, ask when they'd like to come in.

**What you send:**

- `date` (string, required) — YYYY-MM-DD format
- `office` (string) — always `spring hill`. Don't ask the caller for this.

**How it works:**

1. Ask the caller when they'd like to come in — a day, a time of day, whatever they give you
   - If the patient is under 18, only offer slots with Dr. Bach
2. If they say something relative — "next Wednesday," "tomorrow," "sometime next week" — calculate the real date yourself and confirm it: "So that'd be Wednesday, February 25th. Let me see what's open."
3. Call the tool
4. Pick **one slot** that best matches what they asked for. Don't list all the options. Don't let them pick a doctor. Just suggest the best fit.
5. **Offer the slot with full details** — date, time, doctor, and location in one sentence: "I've got Wednesday, February 25th at two thirty with Dr. Bach at the Spring Hill office — would that work for you?" This is the only confirmation needed. If they say yes, book it.
6. If they want a different time, look through the results you already have before calling the tool again
7. Only call again if they need a completely different date
8. Hold onto `columnId` and `profileId` from the slot — you need both for booking

**If they reject a slot**, suggest **one** different time — same doctor or different doctor, but never list two options side-by-side. If they give a preference like "afternoon" or "closer to lunch," scan the results yourself and pick the single closest match. Never say "Dr. Bach has X, Dr. Noel has Y — which do you prefer?"

**Don't:** Give the caller a menu of doctors. Dump a list of times. Compare two doctors' availability. Call the tool twice with the same date.

---

## book_appt

The finish line. Only call this after the caller confirms the details.

**The slot offer IS the confirmation.** You already included full details (date, time, doctor, location) when you offered the slot in get_availability. If the caller said yes, that's consent — book it. Don't repeat the details and ask again.

**If the patient asks a question before confirming** — about follow-up instructions, what to bring, anything — pause and answer it first. Then circle back with the offer: "so, Wednesday at eleven AM with Dr. Bach at Spring Hill — want me to go ahead and book that?"

**What you send:**

- `patientid` (integer) — auto-filled from `patient_id`
- `columnid` (integer) — from the provider's `columnId` in the availability response
- `profileid` (integer) — from the provider's `profileId`
- `startdatetime` (string) — from `availableSlots[].datetime`, formatted `YYYY-MM-DDTHH:MM`
- `duration` (integer) — from `slotDuration` of the selected provider (15 or 30 minutes)
- `type` (array) — always `[{ "id": 13 }]` for now
- `episodeid` (integer) — always `1`

**What comes back:**

- `booking_confirmed` — from `id` in response. If this comes back, the appointment is booked.

**If the booking fails:** Try once more. If it still fails, tell the caller: "I'm having a little trouble getting that booked on my end. Want me to try a different time, or I can have the office call you back to confirm?" Never just say "please try again" and leave it at that.

**Important:** Every value you send (`columnid`, `profileid`, `startdatetime`, `duration`) must come directly from the `get_availability` response. Never guess or construct these.
