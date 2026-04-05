# Code Review: feature/watchdog

**Reviewer:** Senior Engineer (independent review)
**Date:** 2026-04-04
**Files reviewed:** 8 files across `meta/plugin/`
**Scope:** Watchdog feature, security hardening, UI rewrite, code simplification

---

## 1. Executive Summary

This is a substantial and mostly well-executed rewrite of the plugin's settings page, adding a watchdog with crash-loop protection, a tabbed dashboard, MQTT configuration, and collector toggle controls -- alongside a meaningful security hardening pass that closed two critical injection vectors. The code is clean, the architecture is reasonable for the Unraid plugin ecosystem, and the security work was done seriously rather than performatively. However, the complete absence of automated testing and the `form markdown="1"` time-bomb mean a few items still need attention before merge. One functional bug (log refresh calling GET on a POST-only endpoint with no server-side handler) was found and fixed during this review, along with a subtler ANSI color inconsistency caught by the cross-review process.

---

## 2. Commit-by-Commit Review

### Commit 1: feat -- Watchdog, hidden flags, UI rewrite

**The good:** This is the meat of the work. The watchdog (`scripts/watchdog`) is well-designed: cron-driven, crash-loop throttled (5 restarts per 5 minutes), log rotation at 500 lines with truncation to 200. The `scripts/apply` smart-restart logic (hash-compare config excluding WATCHDOG_ lines, only restart if non-watchdog settings changed) is a genuinely thoughtful piece of engineering that prevents unnecessary agent restarts when the user is just toggling watchdog settings.

The `.page` file is a full rewrite from what was presumably a flat form into a tabbed dashboard with a persistent header, connection modal, and live log viewer. This is production-quality plugin UI by Unraid standards.

**The concerning:** The volume of change in one commit is high. A 1000+ line `.page` file with interleaved PHP, HTML, CSS, and JavaScript is hard to review atomically. This is partly just the nature of Unraid plugin development (you don't get to split into separate files), but it means bugs hide easily.

### Commit 2: security -- Injection hardening

**The good:** This commit specifically addresses the two critical findings (C-1: command injection via WATCHDOG_INTERVAL in cron, C-2: command injection via config values in `bash -c`). The fixes are correct:

- `sanitize_int()` using `tr -cd '0-9'` (line 68 of `scripts/start`) is the right approach -- strip everything, not allowlist specific values.
- The `case` statement for WATCHDOG_INTERVAL (lines 105-108 of `scripts/start`) provides belt-and-suspenders validation: even if `sanitize_int` passed something unexpected, only `1|2|5|10|15|30` survive.
- The `env(1)` launch pattern (lines 125-160 of `scripts/start`) replacing `sudo bash -c "..."` is the correct fix for C-2. Environment variables passed through `env` cannot break out of their value context the way shell interpolation can.
- `htmlspecialchars()` applied consistently in the PHP for all user-derived values rendered to HTML.

**The concerning:** The `sanitize_str()` function (line 73 of `scripts/start`) strips `' " \` $ \` but does not strip semicolons, pipes, ampersands, or parentheses. This is fine *only* because `sanitize_str` values are passed through `env(1)` and never interpolated into a shell command. If someone later refactors the launch to use shell interpolation again, `sanitize_str` would be insufficient. A comment noting this dependency would be prudent.

### Commit 3: refactor -- Code simplification

**The good:** The simplification pass is disciplined. The `toggleSection` helper (line 922-926 of `.page`) consolidates four near-identical functions into wrappers. The CSS consolidation removes inline styles in favor of classes (`.uma-endpoint-row`, `.uma-endpoint-code`, `.uma-collector-spacer`). The `setConnModal(open)` pattern (line 957-963) replacing separate open/close functions is cleaner.

**The eval concern:** The sanitization loops at lines 84-97 of `scripts/start`:

```bash
for var in INTERVAL_SYSTEM INTERVAL_ARRAY ...; do
    eval "$var=\$(sanitize_int \"\$$var\")"
done
```

The brief correctly identifies this as safe because (a) variable names are hardcoded string literals, (b) the values pass through `sanitize_int` which strips everything except digits before assignment. The `eval` expands to something like `INTERVAL_SYSTEM=$(sanitize_int "$INTERVAL_SYSTEM")`. There is no path from user input to the variable *name* side of the assignment. I agree this is fine, but I would want a one-line comment above the loop: `# Safe: variable names are hardcoded literals, not user input`.

---

## 3. Security Posture

### Fixed (verified in code)

| ID | Fix | Verification |
|----|-----|-------------|
| C-1 | `sanitize_int` + `case` allowlist for WATCHDOG_INTERVAL | `scripts/start` lines 103-108 -- correct, double-validated |
| C-2 | `env(1)` launch replacing `bash -c` | `scripts/start` lines 125-160 -- correct, no shell interpolation of values |
| H-3 | `chmod 0600` on config file | `scripts/apply` lines 12-17 -- runs immediately after config write |
| M-1 | Variable quoting | `scripts/stop` line 9: `pidof $PLUGIN` still unquoted (see below) |
| M-2 | Crash log HTML escaping | `.page` line 390: `htmlspecialchars($last_crash, ...)` -- correct |
| M-4 | DISABLE_COLLECTORS sanitized | `scripts/start` line 82: `sanitize_csv` strips to `[a-zA-Z0-9,_-]` -- correct |
| L-1 | Output escaping on uptime/crash_count | `.page` lines 321, 389: correct |
| L-2 | Log rotation | `scripts/watchdog` lines 20-26: correct |

### Partially Fixed

**M-1 (variable quoting):** The brief says this is fixed, but `scripts/stop` line 9 still has `pidof $PLUGIN` and line 11 has `killall $PLUGIN` without quotes. Since `PLUGIN` is hardcoded on line 2 as `"unraid-management-agent"` (no spaces, no special characters), this is not exploitable. But the *finding* was about quoting hygiene as a maintenance concern, and it's still inconsistent. Lines 9 and 11 of `scripts/stop` should use `"$PLUGIN"` for consistency with the rest of the codebase.

Similarly, `scripts/start` line 22: `killall $PLUGIN` is unquoted. Line 9: `mkdir -p /boot/config/plugins/$PLUGIN` is unquoted. These are all safe because the value is a hardcoded alphanumeric string, but the inconsistency weakens the "we've fixed quoting" claim.

### Not Fixed (by design)

**H-1 (CSRF/auth):** Correctly left unfixed. The `exec.php` comment on line 5 documents the trust delegation to Unraid's `local_prepend.php`. This is the standard Unraid plugin pattern. Adding a custom CSRF token would be non-standard and fragile (Unraid could change its session mechanism). The residual risk is real but is a platform-level concern, not a plugin-level one.

**M-3 (CORS wildcard):** I disagree with the "security theater" framing in the brief. CORS `*` combined with no auth means any malicious website can read server metrics from a user's browser. This is a real CSRF-adjacent attack (technically it's a cross-origin data theft, not CSRF). The brief argues that non-browser clients can access the API anyway, which is true -- but CORS specifically exists to protect *browser* users, and those are the users most likely to visit malicious sites. The default should be the Unraid server's own origin, not `*`. This is the single security issue I would push back on.

### Risk Assessment

The security posture is **good for the ecosystem**. The critical injection vectors are closed. The remaining items are either ecosystem conventions (H-1) or deliberate choices (M-3). The `chmod 0600` on `/boot` FAT filesystem is noted in the brief as potentially ineffective -- FAT doesn't support Unix permissions, so the `chmod` will succeed but may not actually restrict access. This is worth documenting but not blocking.

---

## 4. Architecture Assessment

### Tab-based Dashboard

The three-tab layout (Dashboard, Collectors, Advanced) is a sensible information architecture. The Dashboard shows operational status (service state, watchdog, endpoints, log); Collectors shows what's being polled and at what rate; Advanced holds security settings and MQTT.

The implementation uses a single `<form>` wrapping all three tabs (line 347), which is the correct approach for Unraid's `update.php` form submission mechanism. All inputs from all tabs submit together. The hidden `DISABLE_COLLECTORS` input (line 351) is synced by JavaScript when collector toggles change. This works but is fragile -- see Section 6.

### Form-within-Tabs

The `overflow: hidden` issue that clipped the Apply button (documented in the brief as "The Apply Button Incident") was fixed by moving the footer to `position: fixed; bottom: 30px` (line 301 of `.page`). The `bottom: 30px` is hardcoded to account for Unraid's own page footer. This will break if Unraid changes its footer height, but there's no dynamic way to detect that height without JavaScript measurement on every page load. Acceptable pragmatism.

### CSS Approach

All CSS is scoped with the `uma-` prefix, which prevents collisions with Unraid's global styles. The use of CSS custom properties (`var(--green-500,#4CAF50)`) with fallback values means the UI will look correct on both themed and unthemed Unraid installations. The `!important` on `.uma-toggle` dimensions (line 202) is necessary because Unraid's global input styles would otherwise override the toggle sizing. This is the correct escape hatch for a plugin fighting against the host page's CSS.

The CSS is entirely in a single `<style>` block (~100 lines). For a plugin `.page` file, this is the only option. It's well-organized with section comments.

### The `form markdown="1"` Fragility

This is real and the brief is right to flag it. Line 347:

```html
<form markdown="1" id="agentConfigForm" ...>
```

Parsedown processes the content of elements with `markdown="1"`. Currently this works because:
1. PHP `<?= ?>` blocks render before Parsedown runs, so dynamic content is already HTML.
2. The HTML structure avoids Markdown triggers (no bare `_underscores_` or `*asterisks*` or `# headings`).
3. `<style>` and `<script>` blocks are passed through by Parsedown.

But the inline help `<blockquote>` blocks contain `<strong>` tags and `<em>` tags in flowing text. If anyone edits these to use Markdown-style bold (`**text**`) instead of HTML tags, Parsedown might double-process them. The current content is safe because it uses HTML tags exclusively.

**My recommendation:** Add a comment at line 347:

```
<!-- WARNING: Parsedown processes this form's content as Markdown.
     Use HTML tags (<strong>, <em>) not Markdown syntax (**, _).
     Bare underscores and asterisks WILL be mangled. -->
```

This costs nothing and prevents a future developer from wasting hours debugging a rendering issue.

---

## 5. What's Solid

**The watchdog design** (`scripts/watchdog`, lines 1-80) is the best piece of engineering in this PR. The crash-loop throttle (count recent "not running" entries by parsing timestamps from the log, compare against window) is robust. The log rotation prevents RAM exhaustion on tmpfs. The heartbeat file (`/var/run/$PLUGIN-watchdog.heartbeat`) lets the UI show when the watchdog last confirmed the agent was alive. This is well-thought-out.

**The smart-apply logic** (`scripts/apply`, lines 36-55) correctly solves a real UX problem. The md5sum comparison of config files (excluding WATCHDOG_ lines) means toggling the watchdog doesn't restart the agent. The running config snapshot (`/tmp/$PLUGIN-running.cfg`) is created on every apply (line 58), so the first apply after boot will always restart (because the snapshot doesn't exist yet), which is the safe default.

**The `env(1)` launch pattern** (`scripts/start`, lines 125-160) is a genuinely good security improvement. Passing all configuration as environment variables through `env` means the values never enter a shell expansion context. This is more secure than any amount of escaping.

**The MQTT password preservation** (`include/update.php`, lines 14-20) is a clean solution to the "form doesn't send back stored passwords" problem. The pre-save hook reads the existing password from the config file when the form submits an empty string. Simple, correct, no edge cases.

**The connection modal** (lines 839-868) with copy-to-clipboard for Claude Code, Claude Desktop, Home Assistant, and Prometheus is genuinely useful for end users. The Claude Desktop snippet uses the `mcp-stdio` mode correctly.

**PHP output escaping is thorough and consistent.** Every user-derived value rendered to HTML goes through `htmlspecialchars($val, ENT_QUOTES, 'UTF-8')`. The ANSI-to-HTML conversion (lines 193-197) escapes first, then injects hardcoded color spans. The JavaScript `ansiToHtml` function (lines 968-974) follows the same pattern. No XSS vectors found. One ANSI inconsistency was caught during the cross-review process: `exec.php` was stripping ANSI codes before returning log text, but the JS `ansiToHtml()` function expected them to be present for color conversion. This meant auto-refreshed logs were monochrome while the initial PHP-rendered page load was colorful. The fix was straightforward -- `exec.php` now returns raw log text with ANSI codes intact, and the JS handles conversion. This is a good example of why cross-review catches things single-pass review misses: the PHP and JS were each individually correct, but the contract between them was broken.

---

## 6. What's Fragile

**The log refresh was broken (now fixed).** The original `refreshLog()` function used `$.get` against a POST-only endpoint, and `exec.php` had no `case 'log':` handler. Both issues were caught during this review and fixed: the JS now uses `$.post` (line 1021) and `exec.php` has a `case 'log':` handler (line 84) that returns the last 20 lines of the agent log as raw text with ANSI codes intact for the JS `ansiToHtml()` function to convert. Without the fix, the "Auto" checkbox and "Refresh" button on the Dashboard tab were completely non-functional -- silent 405s every 10 seconds.

**The collector sync has the edge case the brief predicted.** If a user manually edits `/boot/config/plugins/unraid-management-agent/config.cfg` to set `DISABLE_COLLECTORS=gpu` but leaves `INTERVAL_GPU=60`, then opens the page:
- PHP line 141: `isCollectorEnabled('gpu', $disabled_arr)` returns `false` (gpu is in disabled_arr)
- The toggle renders unchecked, the select renders disabled
- BUT the select's value is still 60, not 0

If the user then clicks Apply without changing anything, the form submits `INTERVAL_GPU=60` and `DISABLE_COLLECTORS=gpu,...`. The agent receives both `--disable-collectors gpu` and `INTERVAL_GPU=60` as an env var. The binary presumably honors `--disable-collectors` over the interval, so this works. But if the user unchecks another collector, `rebuildDisableCollectors()` (line 933-939) rebuilds the list from checkboxes only -- it doesn't check interval values. So the resulting `DISABLE_COLLECTORS` will include `gpu` (toggle unchecked) plus whatever else was unchecked. This is actually correct behavior, just potentially confusing if the user expects the interval dropdown to also show 0.

**The `bottom: 30px` footer positioning** (line 301) assumes Unraid's footer is 30px tall. If Unraid 7.x changes the footer height, the Apply button either overlaps content or floats above the Unraid footer with a visible gap. There's no way to fix this dynamically without measuring the host page's footer in JavaScript.

**The stop script's kill loop** (`scripts/stop`, lines 11-19) sends `killall` in a loop, sleeping 1 second between attempts, for up to 30 seconds before escalating to `SIGKILL`. But `killall` sends `SIGTERM` by default, and if the process is stuck in an uninterruptible sleep (D state), `SIGTERM` will never work and `SIGKILL` might not either. The 30-second timeout is generous, but there's no logging of the escalation, so a hung shutdown will be invisible until someone wonders why the stop took 30+ seconds.

**The `$secs` variable scope leak.** Line 77 of the `.page` file assigns `$secs` inside a conditional block, but line 321 references it via `data-secs="<?= $running ? (int)$secs : 0 ?>"`. If `$running` is true but `$etimes_out` is empty (race condition where process dies between `pidof` and `ps`), `$secs` is undefined. PHP will emit a notice and treat it as 0, which is harmless but sloppy. The ternary on line 321 partially guards this (it checks `$running`), but there's a window where `$running` is true and `$secs` is unset.

---

## 7. What's Missing

### The Log Refresh Bug (Found and Fixed)

This was a functional bug caught during this review. The `exec.php` endpoint had no `case 'log':` handler, and the JavaScript called it via GET against a POST-only endpoint. The fix took option 3 from the original analysis: the JS now uses `$.post` and `exec.php` has a `case 'log':` handler returning the last 20 lines of the agent log with ANSI codes intact. This should have been caught by manual testing before the review (did nobody click the Refresh button?), but it was caught here and fixed promptly.

### Automated Testing

The brief is comprehensive about what's not tested. Let me be specific about what I would *actually* test and how:

**Shell script unit tests (using `bats` or `shunit2`):**
- `sanitize_int` with inputs: `"60"`, `"60; rm -rf /"`, `""`, `"abc"`, `"-1"`, `"99999999999"` -- verify only digits survive
- `sanitize_csv` with inputs: `"gpu,docker"`, `"gpu;docker"`, `"gpu|docker"`, `"$(whoami)"` -- verify only safe chars survive
- `sanitize_bool` with inputs: `"true"`, `"false"`, `"TRUE"`, `"1"`, `""`, `"true; echo pwned"` -- verify only `"true"` returns `"true"`
- Watchdog interval validation: verify the `case` statement rejects `3`, `0`, `60`, `"5; echo pwned"`, empty string
- Watchdog script: mock `pidof` to return failure, verify the script calls `start`, verify throttle kicks in after 5 crashes

**PHP tests (using PHPUnit):**
- `update.php` password preservation: submit empty password with existing config, verify password is restored
- `update.php` password preservation: submit new password, verify it's NOT overwritten
- `exec.php` action validation: verify unknown actions return 400, GET returns 405
- `isCollectorEnabled()`: test with collector in disabled list, test with interval=0, test with both

**Integration tests (manual, documented checklist):**
- Start/Stop/Restart buttons work and update the UI
- Watchdog toggle + Apply does NOT restart the agent (verify PID unchanged)
- Changing the port + Apply DOES restart the agent (verify PID changed)
- Kill the agent process, wait for watchdog interval, verify it restarts
- Kill the agent 6 times in 5 minutes, verify watchdog stops trying
- Set an MQTT password, click Apply, verify the password is preserved on next page load

### CI Pipeline

At minimum: `shellcheck` on the four shell scripts. This would catch the unquoted variables and any other issues. `phpcs` or `phpstan` on the PHP files would also be useful. None of this exists.

### Error Handling Edge Cases

- What happens if the binary at `$PROG` doesn't exist? The `scripts/start` will silently fail, and `nohup` will write an error to `/dev/null`. The watchdog will then try to restart it every interval, hit the throttle, and stop. The user will see "STOPPED" on the UI with no explanation. There should be a pre-flight check: `[ -x "$PROG" ] || { echo "Binary not found: $PROG" >&2; exit 1; }`.
- What happens if the config file is corrupted (not valid INI)? `parse_ini_file()` returns `false`, and all `$config['KEY'] ?? 'default'` calls will trigger warnings because you can't index into `false`. Line 18 of the `.page` file checks `file_exists` but not the return value of `parse_ini_file`.

---

## 8. The Fan Incident

### Root Cause Analysis

The brief's analysis is correct: the fan spin-up was **not caused by this code**. The evidence:

1. Toggling the watchdog only modifies the cron file and sends `HUP` to `crond`. It does not restart the agent.
2. CPU temps were normal (low 30s C).
3. The agent was running the entire time (PID unchanged).

The most likely cause was the Dynamix Auto Fan plugin responding to a transient temperature spike from the rsync/SSH activity during the debugging session itself. The fan_control collector has a 5-second polling interval, but it reads fan state -- it doesn't *set* fan speeds. Fan control is the Dynamix plugin's job.

### Is the Smart Restart Fix Sufficient?

Yes, for the scenario described. The smart-apply logic (`scripts/apply` lines 36-55) correctly prevents agent restart when only WATCHDOG_ENABLED or WATCHDOG_INTERVAL change. This means toggling the watchdog will never cause a fan_control collector re-initialization.

However, there's a subtle gap: the smart-apply compares configs by hashing all non-WATCHDOG lines. If a user changes only the watchdog setting AND something triggers a config file difference on a non-WATCHDOG line (e.g., trailing whitespace, key reordering by `update.php`), the hash comparison would see a difference and restart anyway. The `sort` in the md5sum pipeline (line 47 of `scripts/apply`) handles key reordering, but not whitespace differences. This is unlikely in practice because Unraid's `update.php` writes configs consistently.

---

## 9. PR Readiness

### Would I approve this PR?

**Yes, with non-blocking requests.** The sole blocking issue -- the broken log refresh -- was found and fixed during this review. The ANSI color inconsistency between `exec.php` and the JS `ansiToHtml()` function was also caught by the cross-review process and fixed in the same pass. No blocking issues remain.

### Fixed During Review

1. **Log refresh bug (fixed).** `exec.php` had no `case 'log':` handler, and the JS used `$.get` against a POST-only endpoint. Both corrected: JS now uses `$.post`, `exec.php` has the handler.

2. **ANSI color inconsistency (fixed).** `exec.php` was stripping ANSI codes before returning log text, but the JS `ansiToHtml()` function expected them for color conversion. Result: initial page load was colorful (PHP-rendered), but every AJAX refresh was monochrome. `exec.php` now returns raw log text with ANSI codes intact; JS handles conversion. This is a good example of the cross-review process catching a subtle contract mismatch that neither the PHP nor the JS would reveal in isolation.

### Required Before Merge (non-blocking but expected)

2. **Add the Parsedown warning comment** at line 347 of the `.page` file. This costs nothing and prevents a maintenance disaster.

3. **Add a comment above the `eval` sanitization loops** in `scripts/start` (line 84) noting that variable names must remain hardcoded literals.

4. **Quote `$PLUGIN` consistently** in `scripts/stop` (lines 9, 11) and `scripts/start` (lines 9, 22). It's not a vulnerability, but it undermines the security hardening story if the reviewer finds unquoted variables after a commit titled "security."

5. **Add a pre-flight check** for the binary in `scripts/start`: `[ -x "$PROG" ] || { echo "Error: binary not found: $PROG" >&2; exit 1; }`. Without this, a missing binary causes silent failure and a confusing watchdog loop.

### Recommended (nice to have)

6. **Change `CORS_ORIGIN` default from `*` to the server's own address.** The brief's "security theater" argument doesn't hold up. CORS exists specifically to protect browser users, and `*` lets any malicious site read server metrics. The consumers listed (Claude Code, HA, Prometheus) are non-browser clients unaffected by CORS. Browser-based consumers (the Unraid UI itself) would work fine with a restrictive origin.

7. **Add `shellcheck` to the build process** (or at least run it once and fix the findings). The scripts are well-written, but shellcheck would catch the remaining quoting inconsistencies and any other subtle issues.

8. **Guard `parse_ini_file` return value** in the `.page` PHP: `$config = parse_ini_file(...) ?: [];`. This prevents warnings on a corrupted config file.

9. **Initialize `$secs`** before the conditional block in the `.page` (line 72): `$secs = 0;`. Prevents an undefined variable notice in edge cases.

### Verdict

This is good work. The security hardening is genuine, the watchdog is well-designed, and the UI is a significant improvement. The log refresh bug is the only thing I'd actually block on. Fix that, add the protective comments, and this is ready to merge.
