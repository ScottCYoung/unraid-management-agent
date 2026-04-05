# Decision Brief -- For the Feature Decision Writer

Every technical decision made during the session, structured with context, alternatives considered, and outcome.

---

## UI Architecture Decisions

### 1. Tabs vs Accordion vs Scrolling

**Context:** The original page was a single vertical-scrolling form. Scott wanted everything to fit on one screen.
**Alternatives:**
- Vertical scrolling (status quo) -- simple but required scrolling past 8+ sections
- Accordion panels -- Unraid uses these elsewhere, but dense config would be many clicks
- Tabbed layout with persistent header -- fits dashboard pattern

**Decision:** Tabbed layout with persistent status header bar.
**Rationale:** Gemini proposed this after analyzing a screenshot of the page plus Unraid frontend docs. Tabs keep related settings grouped while the status bar provides at-a-glance service health without switching tabs.

### 2. Three Tabs vs Four Tabs

**Context:** Initial layout had sections that needed grouping into tabs.
**Alternatives:**
- 3 tabs: Dashboard, Collectors, Advanced
- 4 tabs: Dashboard, Collectors, MQTT, Advanced
- 4 tabs: Dashboard, Collectors, Advanced, Log

**Decision:** Started with 4 (MQTT as separate tab), settled on 3 (MQTT folded into Advanced).
**Rationale:** Scott asked "should MQTT be moved to advanced?" -- MQTT is a power-user feature, not something most users configure frequently. Keeping it in Advanced reduces tab count and groups all "expert" settings together. The Advanced tab uses a 2-column top row (Security left, MCP info right) with full-width MQTT section below.

### 3. Log Display: Drawer vs Inline

**Context:** Log tail needed to be visible without leaving the dashboard.
**Alternatives:**
- Slide-out drawer from the side
- Modal overlay
- Inline panel on the Dashboard tab
- Separate Log tab

**Decision:** Inline panel on the Dashboard tab, below Watchdog and Endpoints.
**Rationale:** The log is a primary monitoring tool -- it should be visible at a glance on the main Dashboard, not hidden behind a click. The `<pre>` element gets a fixed max-height with overflow-y scroll. Auto-refresh every 10 seconds when the Dashboard tab is active and the "Auto" checkbox is checked.

### 4. Getting Connected: Inline Table vs Modal

**Context:** Users need copy-paste snippets for Claude Code MCP, Claude Desktop, HA, and Prometheus.
**Alternatives:**
- Inline table always visible on Dashboard
- Modal triggered by a "Connect" button in the header
- Separate "Connections" tab

**Decision:** Modal triggered by a header button.
**Rationale:** Connection snippets are used once during setup, not monitored continuously. A modal keeps the Dashboard clean while making the info accessible from any tab via the persistent header. Each row has a Copy button with clipboard API + fallback.

### 5. Status Bar: Full Section vs Compact Header

**Context:** Service status (running/stopped, uptime, version, crash count) needs to be always visible.
**Alternatives:**
- Full "Status" section as the first tab content
- Compact header bar above the tab row
- Status indicators within the tab bar itself

**Decision:** Compact persistent header bar above tabs.
**Rationale:** The header shows the essentials (status orb, RUNNING/STOPPED text, uptime, version, crash count) plus Start/Stop/Restart buttons and a Connect button. It persists across tab switches. Meta info (uptime, version, crashes) hides when the service is stopped since those values are meaningless in that state.

---

## Collector Management Decisions

### 6. Collector Disable Mechanism: DISABLE_COLLECTORS vs interval=0 vs Both Synced

**Context:** The binary supports two ways to disable a collector: the `--disable-collectors gpu,ups` flag, and setting an interval to 0 (which some collectors interpret as "don't run").
**Alternatives:**
- Use only `DISABLE_COLLECTORS` comma-separated list
- Use only interval=0 to signal disabled
- Sync both: toggling a collector off sets interval=0 AND adds to DISABLE_COLLECTORS

**Decision:** Sync both directions.
**Rationale:** The binary checks both mechanisms internally. If a user sets interval=0 via the dropdown, the collector should also appear in DISABLE_COLLECTORS (and vice versa). The PHP code rebuilds `$disabled_arr` to include any collector whose interval is 0. The JS `rebuildDisableCollectors()` builds the comma-separated list from unchecked toggles. When a toggle is unchecked, JS also sets the interval dropdown to 0 and disables it. When a dropdown is changed to 0, JS unchecks the toggle. This bidirectional sync prevents the confusing state where a collector appears "enabled" in the toggle but has interval=0.

### 7. Collector Grid Layout

**Context:** 13 togglable collectors plus 4 internal (interval-only) collectors need an efficient layout.
**Alternatives:**
- Single column list (original)
- Two-column grid
- Three-column grid with semantic grouping

**Decision:** Three-column grid with section headers.
**Rationale:** Scott explicitly requested: "can we make it wider, use 3 columns?" Collectors are grouped semantically:
- Column 1: System Monitoring (system, array, disk, network) + Containers & VMs (docker, vm)
- Column 2: Hardware (gpu, ups, nut, hardware) + Internal (notifications, license, fan control, tuning)
- Column 3: Storage (shares, zfs, unassigned)

Internal collectors (notifications, license, fan control, tuning) have only interval dropdowns, no enable/disable toggle, because they're core to the agent's operation. They use a spacer span instead of a toggle to maintain visual alignment.

### 8. Low Power Mode Placement

**Context:** Low power mode multiplies all intervals by 4x. Initially placed on the Dashboard.
**Alternatives:**
- Dashboard quick-config section
- Advanced tab
- Collectors tab header

**Decision:** Collectors tab header, right-aligned with a "Low Power (4x intervals)" label.
**Rationale:** Scott caught this: "and low power?" after asking about fan management placement. Low power directly affects collector intervals, so it belongs on the Collectors tab. Placing it in the tab header (not inside a card) makes it a global modifier visible at the top of the collector configuration.

---

## Watchdog Decisions

### 9. Watchdog Implementation: Cron vs Systemd vs Unraid's update_cron

**Context:** Unraid doesn't use systemd. The plugin needs a way to periodically check if the agent is alive.
**Alternatives:**
- Systemd timer -- not available on Unraid (SysV init)
- Unraid's `update_cron` API function
- Direct cron file in `/etc/cron.d/` with crond HUP

**Decision:** Direct cron file `/etc/cron.d/unraid-management-agent-watchdog` with `killall -HUP crond`.
**Rationale:** Writing directly to `/etc/cron.d/` is simpler and gives full control over the interval. The `update_cron` mechanism would work but adds a dependency on Unraid's internal API. The cron file is created/removed by `scripts/start` and `scripts/apply`, and removed first in `scripts/stop` to prevent the race condition.

### 10. Stop Script Cron Removal Ordering (Race Prevention)

**Context:** If the watchdog cron fires while the stop script is killing the process, the watchdog would see the agent as dead and restart it.
**Alternatives:**
- Remove cron after stopping the process
- Remove cron before stopping the process
- Use a PID lock file to signal "deliberate stop"

**Decision:** Remove cron BEFORE stopping the process.
**Rationale:** This is the simplest race-free approach. The stop script removes the cron file and HUPs crond as its very first action, before calling `killall`. This ensures the watchdog can't fire during the shutdown window. No lock files or signal coordination needed.

### 11. Watchdog Throttle Parameters

**Context:** Need to prevent infinite restart loops if the binary is broken or misconfigured.
**Alternatives:**
- No throttle (let cron keep restarting forever)
- Simple counter with no time window
- Time-windowed counter (N crashes in M minutes)
- Exponential backoff

**Decision:** 5 crashes within a 5-minute window triggers throttle.
**Rationale:** `MAX_CRASHES=5`, `THROTTLE_WINDOW=300` seconds. The watchdog reads the crash log, counts entries with "not running" in the last 300 seconds, and skips the restart if the count exceeds 5. This allows occasional transient crashes to be recovered while preventing a broken binary from causing an infinite restart loop. The throttle logs its decision to the crash log for debugging.

### 12. Apply Script: Always Restart vs Smart Restart

**Context:** Clicking Apply triggers the apply script. If only the watchdog interval changed, restarting the agent is unnecessary (and causes fan spin-up).
**Alternatives:**
- Always restart on Apply
- Never restart, require manual restart
- Smart restart: compare config hashes excluding WATCHDOG fields

**Decision:** Smart restart with hash comparison.
**Rationale:** The apply script copies the config to `/tmp/unraid-management-agent-running.cfg` after each apply. On the next apply, it compares the new config (minus WATCHDOG lines) against the running snapshot. If they differ, it restarts; if only WATCHDOG fields changed, it just updates the cron and skips the restart. This avoids the fan spin-up issue Scott noticed.

---

## Security Decisions

### 13. `env(1)` vs `bash -c` for Launching the Binary

**Context:** The original start script used `sudo -H bash -c "..."` with double-quoted shell interpolation of config values. The security audit flagged this as Critical (C-2).
**Alternatives:**
- Keep `bash -c` but add quoting/escaping
- Use `env` command to set environment variables before the binary
- Write a temporary env file and source it

**Decision:** Replace entire launch block with `nohup env VAR=val ... "$PROG" args`.
**Rationale:** `env(1)` passes variables as key=value pairs without shell interpretation. No interpolation into a shell string, no quote-escaping needed. The binary reads its config from environment variables, so this is the natural approach. Comments in the script note: "Use env(1) to pass variables safely instead of shell interpolation."

### 14. Input Sanitization Approach

**Context:** The security audit found config values flowing unsanitized from the form POST into shell commands.
**Alternatives:**
- Validate in PHP before writing config (update.php)
- Validate in bash after sourcing config (scripts/start)
- Validate in both places
- Use an allowlist in the binary itself

**Decision:** Validate in bash in `scripts/start` using four helper functions.
**Rationale:** Four sanitization helpers were added:
- `sanitize_int()` -- strips everything except digits via `tr -cd '0-9'`
- `sanitize_csv()` -- strips everything except `a-zA-Z0-9,_-`
- `sanitize_str()` -- strips shell metacharacters `'"\`$\`
- `sanitize_bool()` -- only "true" passes, everything else becomes "false"

All config values pass through the appropriate sanitizer before use. WATCHDOG_INTERVAL gets an additional `case` statement clamping to the allowlist `1|2|5|10|15|30`. Interval variables are sanitized in a `for` loop using `eval` to avoid 17 repetitive lines.

### 15. ANSI Log Colors: Strip vs Convert to HTML Spans

**Context:** The agent's log file contains ANSI escape codes for colored output. Displaying raw ANSI in a `<pre>` tag shows garbage characters.
**Alternatives:**
- Strip all ANSI codes, show plain text
- Convert ANSI codes to HTML `<span>` tags with inline color styles
- Use a JS library like ansi_up

**Decision:** Convert to HTML spans, both server-side (PHP) and client-side (JS).
**Rationale:** Colors in logs are useful -- red for errors, green for success, blue for info. The PHP side uses `preg_replace_callback` to map ANSI codes to HTML spans after first running `htmlspecialchars` on the raw text (preventing XSS). The JS side has an `ansiToHtml()` function that does the same for AJAX log refreshes. A hardcoded color map covers codes 31-36 (red, green, yellow, blue, magenta, cyan).

### 16. CORS Wildcard Default

**Context:** The security audit flagged `CORS_ORIGIN="*"` as Medium severity (M-3).
**Alternatives:**
- Change default to empty string (most restrictive)
- Change default to Unraid's own address
- Keep `*` as default with a warning in the UI

**Decision:** Left as-is (`*` default).
**Rationale:** The agent has no built-in authentication -- it's designed to be accessed by other tools on the local network. Restricting CORS by default would break the most common use case (Claude Code on a different machine hitting the API). The Advanced tab includes a security info box noting: "The agent has no built-in authentication. For external access, use a reverse proxy with Basic Auth or SSO."

---

## Form and Rendering Decisions

### 17. `form markdown="1"` Compatibility with Raw HTML

**Context:** Unraid uses Parsedown (a Markdown parser) to process `.page` files. The `form markdown="1"` attribute tells Parsedown to process the form's content as Markdown, which can mangle raw HTML.
**Alternatives:**
- Remove `markdown="1"` and use pure HTML
- Keep `markdown="1"` and carefully structure HTML to avoid Parsedown interference
- Use PHP to render everything before Parsedown sees it

**Decision:** Keep `markdown="1"` for compatibility with Unraid's expected patterns.
**Rationale:** Removing `markdown="1"` could break the `#file`, `#include`, and `#command` hidden inputs that Unraid's `/update.php` processes. The page uses PHP to render all dynamic content before Parsedown runs, and the HTML structure avoids triggering Markdown interpretation (no bare underscores, no lines starting with `#` outside PHP blocks). The `blockquote class="inline_help"` blocks work correctly because Parsedown recognizes them as HTML.

### 18. Version Detection: version.txt vs VERSION vs .plg Parsing

**Context:** The version showed "Unknown" in the header because no version file existed at the expected path.
**Alternatives:**
- Parse the `.plg` XML file to extract the version
- Look for a `VERSION` file (all caps)
- Look for a `version.txt` file
- Hardcode the version in the PHP

**Decision:** Fallback chain: check `VERSION` first, then `version.txt`.
**Rationale:** The `.plg` file lives on `/boot` and may not be efficiently accessible from the page PHP. A simple file check at two common paths covers the case where the plugin's build process creates either file name. The version is escaped with `htmlspecialchars` before rendering.

### 19. Footer Positioning

**Context:** The Apply/Default/Done buttons were invisible because `overflow: hidden` on the form wrapper clipped them.
**Alternatives:**
- Put buttons inside the scrollable content area
- Use `position: sticky` at the bottom
- Use `position: fixed` at the bottom of the viewport

**Decision:** `position: fixed` with `bottom: 30px` and a top border.
**Rationale:** Fixed positioning ensures the buttons are always visible regardless of tab content height or scroll position. The `bottom: 30px` accounts for Unraid's own fixed footer bar. A `border-top` and matching background provide visual separation.

---

## Log and Monitoring Decisions

### 20. Server Clock Ticker

**Context:** Scott asked "for the logs: is that server time? or was that an hour ago?" -- the log timestamps were in server time but the user was in a different timezone.
**Alternatives:**
- Convert log timestamps to browser-local time
- Show server time somewhere on the page
- Show both server and local time

**Decision:** Add a "Server: YYYY/MM/DD HH:MM:SS" ticker next to the log header that updates every second.
**Rationale:** Converting individual log timestamps would require parsing and is fragile. Instead, the page embeds the server's Unix epoch in PHP (`<?= time() ?>`) and ticks it forward locally in JS. The ticker gives the user a reference point to interpret log timestamps. A separate `setInterval` also ticks the uptime display in the header.

### 21. Auto-Refresh Behavior

**Context:** Scott asked "does this refresh automatically at all? if so is it a soft refresh in the log viewer box?"
**Alternatives:**
- No auto-refresh (manual only)
- WebSocket-based live stream
- AJAX poll every N seconds

**Decision:** AJAX poll every 10 seconds, togglable with an "Auto" checkbox, only when Dashboard tab is active.
**Rationale:** WebSocket would require changes to the agent binary. AJAX polling is simple and uses the existing `include/exec.php` endpoint (action: 'log'). The "Auto" checkbox defaults to checked. The refresh preserves scroll position (if the user was scrolled to the bottom, it stays at the bottom after refresh). The 10-second interval balances freshness against load.

### 22. Crash Log Rotation

**Context:** The security audit flagged unbounded crash log growth as a Low severity issue (L-2). `/var/log` on Unraid is tmpfs (RAM).
**Alternatives:**
- No rotation (original)
- Rotate by file size
- Rotate by line count
- Use logrotate

**Decision:** Rotate by line count in the watchdog script itself.
**Rationale:** `MAX_LOG_LINES=500`. At the start of each watchdog run, if the crash log exceeds 500 lines, it's truncated to the most recent 200 lines. This runs inside the watchdog (which is already reading the file for throttle checks), requires no external logrotate configuration, and keeps memory usage bounded.

---

## Fan Control Investigation

### 23. Fan Speed Incident

**Context:** Scott reported "fans went crazy" after toggling the watchdog. Investigation via SSH and Unraid MCP showed CPU temps were normal (low 30s C).
**Alternatives considered for root cause:**
- Watchdog cron causing CPU spike
- Fan control collector resetting fans on agent restart
- BIOS fan profile change
- Coincidental background task

**Decision:** Not a bug in our code. The fan_control collector re-initializes when the agent restarts, which can briefly set fans to a different speed. The agent was not restarting (only the watchdog cron was toggled), so the fans were likely responding to the Dynamix Auto Fan plugin's own behavior. No code change needed.
**Rationale:** SSH investigation showed CPU temps normal, no unusual processes. The fan_control collector interval of 5 seconds means fans settle quickly. The user confirmed fans returned to normal.
