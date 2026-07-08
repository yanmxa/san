# Autopilot

## Overview

Autopilot is San's configurable **copilot**: a second driver that steers the
session at fixed points — proposing or rewriting input, approving gray-zone
tool calls, answering a command's interactive prompts, answering
`AskUserQuestion` on your behalf, and auto-continuing finished turns toward a
mission. Each of those points is a **steer** you toggle independently; only the
gray-zone permission judging is on by default.

Enter AutoPilot mode with `shift+tab` (cycle until the amber
`⏵⏵ autopilot on`), and configure it with the `/autopilot` panel. A resumed
session (`san -r <id>`) comes back in the mode it was saved in.

## The six steers

Steers are à-la-carte toggles, ordered by increasing autonomy. None fire unless
AutoPilot mode is engaged.

| Steer | Default | What it does |
|---|---|---|
| **Suggest** | off | Fills the input hint (ghost text) with the copilot's proposed next step — toward the mission when one is set, the generic prediction otherwise. `tab` accepts, `enter` sends. It suggests; it never acts. With Suggest *off*, AutoPilot suppresses the hint entirely so the copilot doesn't nudge. |
| **Start** | off | Owns the turn's input: rewrites each message you send into a clearer, mission-aligned instruction, and — when you enter AutoPilot with a mission set and an empty composer — kicks off the mission by deriving and submitting the first step itself. |
| **Permission** | **on** | Auto-approves gray-zone tool calls the static rules couldn't resolve, judging reversibility, blast radius, and data exfiltration. Fails closed: any error escalates to you. |
| **Bash** | off | Answers an already-approved command's interactive prompt (`Continue? [Y/n]`) when the answer just continues the approved action; skips anything that would widen scope. |
| **Question** | off | Answers `AskUserQuestion` for you when the mission makes the choice clear and low-risk; defers to you otherwise. Option labels are validated verbatim — a partial or invented answer becomes a defer. |
| **End** | off | After a finished turn, decides whether to continue toward the mission and types the next instruction itself. Bounded by **Continue at most N times** (default 20); the counter resets on every human turn. |

## Mission

The mission is what the copilot drives toward this session — briefed
conversationally in the `/autopilot` panel's Mission dialog (`enter` sends,
`ctrl+r` clears, `esc` saves back). The steering steers (Suggest, Start,
Question, End) read it; the safety steers (Permission, Bash) deliberately never
see it, so an action's risk is judged independently of intent.

When the End steer decides the mission is **fully accomplished**, it retires
it: the mission is cleared and the steers reset to the passive baseline
(Permission + Bash) — AutoPilot stays on, you take the wheel back with the
auto-approve safety net intact.

## Reading the transcript

| Mark | Meaning |
|---|---|
| green `↖ autopilot · 2/5` | the `❭` line above was typed by the copilot (continuation 2 of 5) |
| green `↖ autopilot · refined` | the `❭` line above is your message, rewritten by the Start steer |
| green `↳ auto-approved · <reason>` | the permission judge let the tool call above through |
| amber `↳ escalated · <reason>` | the judge sent the call back to you |
| green `⏵ autopilot · answered for you` | the copilot answered an `AskUserQuestion` |
| amber `↩ autopilot · this question is yours` | it deferred the question to you |
| amber `↩ autopilot · over to you` | it stopped and handed control back (a decide error rides after it) |
| green `✓ autopilot · mission complete` | the mission is done and retired |

While a decision is in flight the mode line reads `⏵⏵ autopilot · thinking…`;
approvals tally there too (`· 3 approved · 1 escalated`).

## Configuration

The panel edits the live session config and saves it to `settings.json` as the
default seed for new sessions. The session's config and mode also persist with
the transcript and restore on `/resume`.

```jsonc
{
  "autoPilot": {
    "model": "anthropic/claude-haiku-4-5", // steer decisions; empty = session model
    "systemPrompt": "…",                   // "how it drives" — shared by all steers
    "systemPromptFile": "~/prompts/pilot.md", // used when systemPrompt is empty
    "mission": "…",                        // usually set via the panel, per session
    "maxContinuations": 20,
    "steers": {
      "suggest": true,
      "turnStart": true,   // the Start steer
      "permission": true,  // omit for the default (on); false escalates everything
      "bashPrompt": true,  // the Bash steer
      "question": true,
      "turnEnd": true      // the End steer
    }
  }
}
```

Named presets: the panel's **▲ Export / ▼ Import** save and load whole configs
under `~/.san/autopilot/<name>.json`.

## Relationship to other features

- [Permission model](../concepts/permission-model.md) — the static rules whose
  gray zone the Permission steer judges; hard-blocked actions never reach it.
- The judge component lives in `internal/reviewer` (`reviewer.Judge`); the
  steers and panel live in `internal/app` / `internal/app/input`.
