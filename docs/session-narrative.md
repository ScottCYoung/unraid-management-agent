# Seven Hours, Three AIs, and a Dashboard That Didn't Exist Yet

*A story about what happens when you sit down to add a watchdog script and stand up having redesigned an entire plugin.*

---

It started, as these things always do, with a small ask.

I wanted a watchdog. That's it. The Unraid Management Agent -- a Go binary that collects system metrics, exposes an API, talks MQTT -- had no crash recovery. If it died, it stayed dead until someone noticed. I'd been running it on my home server for a while, a fork of Ruaan Deysel's original project, and I'd learned the hard way that "it just runs" is an optimistic assumption when you're dealing with a plugin that monitors everything from disk temps to Docker containers.

So on a Friday evening in April, I opened Claude Code and typed out what I thought was a well-scoped request: three phases. Phase one, add a watchdog. Phase two, expose all the binary flags that the Go daemon already supported but the settings page didn't surface. Phase three, rewrite the UI. Clean. Structured. Reasonable.

Seven hours later, I had a complete dashboard with tabbed navigation, a live log viewer with ANSI color rendering, a security audit that found shell injection vulnerabilities I didn't know existed, a code simplification pass, a fan speed investigation involving live SSH into the server, and a three-AI writing pipeline to document what had just happened.

The watchdog was in there too, somewhere.

---

## The First Surprise: A Security Hook That Doesn't Trust PHP

Claude read through the existing codebase -- the `.page` file (Unraid's PHP-based UI format), the start and stop scripts, the `default.cfg` -- and delegated the initial code generation to Gemini via MCP. This was the workflow I'd asked for: Claude orchestrates, Gemini does the heavy lifting on generation, Claude verifies and integrates.

The first draft came back fast. Watchdog script with crash throttling, expanded config file with sixteen new keys, a complete page rewrite with sections for everything the binary supported. Time to write it to disk.

Except it wouldn't write.

The Write tool just... refused. A security reminder hook -- something I'd set up ages ago for a different project -- detected PHP shell calls in the `.page` file and blocked the operation. The hook was designed to catch dangerous patterns in TypeScript projects. It had never encountered legitimate Unraid PHP before, where calling out to the shell is how you read config files and check process status. It saw the pattern and slammed the door.

Claude pivoted without missing a beat. Instead of Write, it used Edit -- matching the entire existing file content as one big string and replacing it wholesale. It's the kind of lateral thinking that makes you realize the AI isn't just following a script. It hit a wall, understood why, and found the gap in the wall.

We were ten minutes in.

## "I Didn't Do Anything. You Tell Me What To Do Next."

This is the moment the session's dynamic locked in.

Claude had written the files, explained the rsync workflow to deploy them to the live Unraid server, and was waiting for me to take action. I pushed back: "I didn't do anything, you tell me what I'm supposed to do next?"

It's a small thing, but it changed everything that followed. From that point on, Claude was proactive. It didn't explain steps and wait. It deployed, checked, and reported. When something needed my input -- like looking at a browser window or clicking a toggle -- it asked specifically for that and nothing more.

This is the collaboration mode that actually works. Not "here are the instructions, good luck" but "I've done everything I can do, here's the one thing I need from you." The difference is enormous when you're iterating on a UI at 10 PM on a Friday.

## The Screenshot That Changed Everything

Claude rsynced the files to the server. I opened the page. It worked -- sort of. It was a long vertical form. Every setting exposed, every toggle present, but you had to scroll forever. It looked like a settings dump, not a dashboard.

I said something like: "Review this page using the Chrome plugin to get screenshots. Share those with the front end designer along with some knowledge links on Unraid's front end setup. Then let it go to town figuring out how to make this a dashboard that fits on a single screen vs vertical scrolling."

This kicked off the most interesting collaboration of the evening. Claude took a screenshot of the live page through Chrome DevTools MCP -- an actual pixel-level capture of what I was seeing in my browser. It sent that screenshot to Gemini along with Unraid's frontend documentation for design advice. Gemini proposed a tabbed dashboard layout with a persistent status header.

This was the moment the scope expanded from "add a watchdog" to a full dashboard redesign. The vertical-scrolling form becomes a three-tab layout: Dashboard (status + watchdog + endpoints + log tail), Collectors (toggle grid), and Advanced (security + MQTT).

## The Tab Count Debate and the Misplaced Widgets

The tab structure went through rapid iteration. Started with three tabs, briefly considered four (MQTT as its own tab), then back to three when I asked "should MQTT be moved to advanced?" -- yes, obviously, it should.

But the real catches came from actually looking at the page. I opened the Dashboard tab and saw fan management controls. "Ok, why did we get the fan management in there?" Then low power mode. "And low power?" These belonged on the Collectors tab, not cluttering up the main dashboard view.

This is the thing about UI work with AI: the code can be syntactically perfect and still be wrong in ways that only become obvious when you look at the rendered result. No amount of code review would have caught "fan control interval doesn't belong on the Dashboard tab." You have to see it. You have to be the user clicking through the tabs and feeling the friction of something being in the wrong place.

The feedback loop was tight. I'd spot something, say it in plain English, Claude would fix it, rsync, and I'd refresh. Under two minutes per cycle. We did this dozens of times.

## The Invisible Button

This one nearly drove me crazy.

"I click enable and then refresh and it doesn't save."

I was toggling settings, hitting refresh, and they'd revert. Nothing was persisting. The page looked complete, the toggles worked, but changes just... vanished.

Claude dove into Chrome DevTools and ran JavaScript against the live page. The form had Default, Apply, and Done buttons -- standard Unraid UI chrome -- but they were rendered below the visible viewport. The new dashboard layout used `overflow: hidden` on the form wrapper, which clipped the buttons. They existed in the DOM. They were technically there. But no human could see them or click them.

I could change every setting on the page and never save a single one because the Apply button was invisible.

Claude fixed it by making the footer `position: fixed` at the bottom of the viewport. Simple CSS. The kind of bug that takes thirty seconds to fix and thirty minutes to find if you don't have a user sitting there saying "it doesn't work."

This is why you test with real people. This is why screenshots matter. This is why "it compiles" means nothing.

## The Help Text Sprint and the CSS Dance

Once saving worked, I wanted inline help. Unraid has a system for it -- a help icon that expands a `blockquote` with `class="inline_help"`. I asked Claude to use Gemini to research how it worked, then write help text for all thirteen collectors.

Gemini researched the Unraid docs, came back with the pattern, and Claude wrote the help descriptions. Then the layout iteration started:

"Can we make it wider, use 3 columns?"

Fix. Rsync. Refresh.

"It should stretch wider."

Fix. Rsync. Refresh.

"Now it's just left aligned."

Fix. Rsync. Refresh.

Three rounds of CSS tweaking for a collector grid that needed to be three columns, full width, evenly distributed. Each round was a different flavor of "close but not right." This is the reality of frontend work. The code is trivial. Getting it to look correct on the actual screen takes iteration.

## vUnknown

"Is there a version to pull? vUnknown is in the header."

The status bar at the top of the dashboard was proudly displaying "vUnknown" because the version detection code looked for a `version.txt` file that didn't exist. The version actually lived in the `.plg` XML file -- the plugin manifest. Claude added a fallback chain: check a `VERSION` file first, then `version.txt`, with proper escaping. Small fix. The kind of thing that makes your dashboard look unfinished if you don't catch it.

## "Looks Like Our Solution Didn't Survive Reboot"

I took a break. Came back a few hours later. Rebooted the server for unrelated reasons.

Everything was gone.

"Looks like our solution didn't survive reboot."

Claude explained immediately: "That's expected. `/usr/local/emhttp/plugins/` is a RAM disk -- tmpfs. It gets rebuilt from the plugin's `.tgz` bundle on every boot. Our rsync was only for live testing."

This is one of the things about Unraid that trips people up. The plugin system extracts everything from a tarball on boot. Any files you put there manually are ghosts. They exist until the next reboot, then they're gone. The real fix means rebuilding the plugin package -- a different workflow entirely.

Claude re-rsynced to restore the working state. Lesson learned: what we were doing was prototyping, not deploying.

## The Security Audit Nobody Asked For (But Everyone Needed)

I was ready to push to my fork. Then I paused.

"Can you do a security scan on it first?"

Claude dispatched a sub-agent -- a dedicated security auditor -- that went through every file. It came back with eleven findings. Two critical. Three high. Four medium. Two low.

The two critical findings were both in the start script. The original code launched the Go binary inside a `sudo -H bash -c "..."` heredoc that interpolated user-controlled config values directly into a shell string. If someone could manipulate a config value -- say, through the web UI -- they could inject arbitrary shell commands that would run as root.

The watchdog interval variable was the other critical: it went straight from the config file into a cron entry without validation. Craft the right interval string and you've got code running via cron.

These weren't bugs I introduced. They were in the original code. They'd been there the whole time. The settings page rewrite just made me think about security for a moment, and that moment was enough.

Claude fixed it in a second commit: replaced the heredoc with `env(1)` for safe variable passing, added sanitization helper functions for integers, strings, booleans, and CSV values, validated the watchdog interval against an allowlist of acceptable values, and added `chmod 0600` to the apply script so config files with potentially sensitive values (MQTT passwords) weren't world-readable.

This is the commit I'm most glad we made. Not the dashboard. Not the tabs. The security fixes. Because nobody was going to find those injection vectors by looking at the UI.

## Twenty-Eight Lines Lighter

"Run it through quick code simplification tooling."

Another sub-agent. This one was a code simplifier. It found duplicated CSS selectors and consolidated them. It created JavaScript helper functions -- `toggleSection`, `setConnModal` -- to replace repeated inline patterns. It removed dead code that referenced DOM IDs that didn't exist anymore (leftovers from the old single-page layout). Net result: twenty-eight fewer lines, same functionality, cleaner to maintain.

The value of a simplification pass after a big rewrite is underrated. When you're building fast, you accumulate cruft. You write the same pattern three times because you're solving three problems and don't stop to notice the pattern. A dedicated cleanup pass catches that.

A code critic agent in the same review pipeline caught something worse: the log refresh was completely broken. The `exec.php` backend had no handler for log requests, and the JavaScript was using GET against a POST-only endpoint. Every time the live log viewer tried to auto-refresh, it silently failed. Nobody had caught it during manual testing because the initial page load worked fine -- only the AJAX refresh path was dead. The multi-agent review found a shipping bug that clicking through the UI hadn't.

## The Fans Go Crazy

It was close to midnight. I was doing final testing. I toggled the watchdog off and back on in the new UI.

"Well that's interesting, I just restarted it and the fans went crazy."

The server fans spun up hard. Not gradually -- immediately. The kind of spin-up that makes you look at the machine and wonder what you just broke.

"I meant I just restarted the watchdog. Disabled and enabled it and the fans immediately kicked in."

Claude went into investigation mode. SSH into the server. Check CPU temps (fine). Check fan speeds (elevated). Check running processes (normal). Check the agent PID -- unchanged. The agent hadn't restarted. Toggling the watchdog only modifies the cron file and sends HUP to crond. The Go binary never stopped running.

So why were the fans screaming?

I had Claude dig deeper via IPMI data and the hwmon sysfs interface. The answer was embarrassingly mundane: CHA_FAN3 on my ASUS Z790-V AX had a BIOS fan curve with a 60% minimum floor. The fans were *always* running that hard. I'd just never noticed during the day. At midnight, in a quiet house, with my attention focused on the server, the noise was suddenly obvious.

The actual fix was two commands: `echo 1 > pwm5_enable` to switch to manual mode, then `echo 100 > pwm5` to set it to ~40%. Not a software bug. A BIOS configuration that had been there the whole time.

But the investigation had productive consequences. We discovered that the Apply script unconditionally restarted the agent for *any* config change -- even ones that only touched the watchdog cron. That hadn't caused the fan incident (the agent stayed running this time), but it was a latent problem. A future Apply that did trigger a restart would cause the fan_control collector to re-initialize, briefly disrupting fan speeds. So we built a smart restart: the Apply script now checks what actually changed and only restarts the components that need it. Preventive medicine, not a fix for the immediate symptom.

## What We Actually Built

Here's the gap between what I asked for and what we shipped:

I asked for a watchdog script. We built a watchdog with crash throttling (five crashes in five minutes before it backs off), log rotation, a heartbeat file, and cron status display in the UI.

I asked to expose hidden binary flags. We wired up sixteen new config keys with input validation, sanitization functions, and a launch mechanism that uses environment variables instead of shell interpolation.

I asked for a settings page rewrite. We built a tabbed dashboard with a status bar, live log viewer with ANSI color rendering, server clock, auto-refresh, connection modal for copying integration snippets, and a three-column collector grid with inline help for every collector.

We also did a security audit (eleven findings, eight fixed), a code simplification pass (twenty-eight lines removed), a fan speed investigation, and a smart apply script that knows whether to restart the full agent or just update the watchdog cron.

None of the last four items were in the original ask.

---

## What This Actually Looks Like

There's a narrative forming about AI-assisted development that goes something like: "You describe what you want, the AI builds it, you're done." That narrative is wrong. Or at least, it's missing the interesting parts.

Here's what actually happened over seven hours:

**The AI hit a wall and had to improvise.** The security hook blocking writes wasn't in any playbook. Claude had to recognize the problem, understand why it was happening, and find a different path. That's not code generation. That's problem-solving.

**The user caught things the AI couldn't.** Misplaced widgets, invisible buttons, left-aligned grids, fans spinning up -- these all required a human looking at a real screen and saying "that's wrong." The AI could fix every one of them in seconds. But it couldn't find them.

**The scope expanded because the work revealed opportunities.** I didn't plan a security audit. But once we'd rewritten the start script, the old patterns became visible, and the audit became obvious. The work created the conditions for more work.

**Multiple AIs played different roles.** Claude orchestrated and coded. Gemini consulted on design and researched docs. Chrome DevTools provided eyes on the live page. Sub-agents handled specialized tasks. This isn't one AI doing everything. It's a team with different strengths.

**The iteration speed was the real unlock.** Not the code generation -- the cycle time. Spot a bug, describe it in English, get a fix, deploy it, verify it, move on. Two minutes per cycle. Over seven hours, that's a lot of cycles. A lot of refinement. A lot of "close but not right" becoming "right."

**The messy parts were the valuable parts.** The invisible Apply button taught me about viewport clipping. The reboot wipe taught me about tmpfs plugin architecture. The fan incident taught me about collector initialization. The security audit taught me about shell interpolation risks. None of these were in the plan. All of them made the final result better.

---

## The Moment at the End

Somewhere around midnight, with the dashboard working, the security fixes committed, the fans investigated and understood, I asked Claude to summarize the journey. Write it like a story, I said. Like The Phoenix Project. Not a dry technical report.

That request -- the one that produced this document -- was itself a collaboration. A researcher agent to gather the timeline and quotes. A writer to shape it into narrative. The tools building the tools building the documentation.

I started the evening wanting a watchdog script. I ended it with a dashboard, a security audit, a fan speed mystery solved, and a story about what happens when you let the work take you where it needs to go.

The watchdog works, by the way. If the agent crashes, it comes back. Five retries in five minutes, then it backs off. Heartbeat file, log rotation, the whole thing.

It was the simplest part of the whole night.
