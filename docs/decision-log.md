# Decision Log

Technical decisions made during the feature-watchdog development session, documented with full context so that future contributors understand not just what was decided but why, and what happened as a result.

---

## UI Architecture

### Tabbed Layout with Persistent Status Header

**Context:** The original plugin settings page was a single vertical-scrolling form. With 8+ configuration sections (collectors, MQTT, security, watchdog, fan control, log viewer, connection snippets, and status indicators), the page required significant scrolling and offered no way to see service health without being at the top. Scott wanted everything to fit on one screen.

**Options considered:**
- Vertical scrolling (the status quo) -- simple, but required scrolling past 8+ sections, and status was only visible at the top of the page
- Accordion panels -- Unraid uses these elsewhere in its UI, but the density of configuration fields would mean many clicks to expand/collapse, and the user could never see two sections simultaneously
- Tabbed layout with a persistent header bar -- fits the dashboard pattern, keeps related settings grouped, and allows an always-visible status region

**Decision:** Tabbed layout with a persistent status header bar above the tab row.

**Rationale:** Gemini proposed this approach after analyzing a screenshot of the existing page alongside Unraid's frontend documentation. Tabs keep related settings grouped (you see all collector config at once, or all advanced settings at once) while the status bar provides at-a-glance service health -- running/stopped, uptime, version, crash count -- without switching tabs. The persistent header also provides a natural home for action buttons (Start/Stop/Restart) and a "Connect" button for integration snippets. This was the highest-leverage single change: it eliminated the scrolling problem while introducing a monitoring surface that hadn't existed before.

**Outcome:** The tabbed layout became the foundation for all subsequent decisions about where to place features. Every new UI element was evaluated as "which tab does this belong on?" rather than "where in the scroll order should this go?"

---

### Four Tabs Collapsed to Three

**Context:** After adopting the tabbed layout, the initial grouping produced four tabs: Dashboard, Collectors, MQTT, and Advanced. The question was whether MQTT deserved its own tab.

**Options considered:**
- 3 tabs: Dashboard, Collectors, Advanced (with MQTT folded into Advanced)
- 4 tabs: Dashboard, Collectors, MQTT, Advanced
- 4 tabs: Dashboard, Collectors, Advanced, Log (with the log viewer as its own tab)

**Decision:** Started with 4 tabs (MQTT as a separate tab), then settled on 3 tabs (MQTT folded into Advanced).

**Rationale:** Scott asked directly: "should MQTT be moved to advanced?" The answer was yes. MQTT is a power-user feature -- most users configure it once during initial setup and then never touch it again. It doesn't warrant dedicated tab real estate competing with things users interact with regularly. The Advanced tab was restructured to use a 2-column top row (Security settings on the left, MCP connection info on the right) with a full-width MQTT section below. This grouping made semantic sense: Advanced is where all "expert" settings live, and MQTT is squarely in that category.

**Outcome:** Three tabs proved to be the right number. The tab bar remains clean and scannable. The Advanced tab is dense but coherent -- everything there is something you configure once and forget about.

---

### Log Display: Inline Panel Instead of Drawer or Modal

**Context:** Users need to see the agent's log output to debug issues, verify collectors are running, and confirm configuration changes took effect. The log tail needed to be visible without leaving the primary monitoring view.

**Options considered:**
- Slide-out drawer from the side -- common in modern web UIs, but adds animation complexity and doesn't feel native to Unraid's aesthetic
- Modal overlay -- would obscure the dashboard content the user is likely cross-referencing against the log
- Inline panel on the Dashboard tab, below the Watchdog and Endpoints sections
- Separate "Log" tab -- was briefly considered as a fourth tab candidate

**Decision:** Inline panel on the Dashboard tab, below the Watchdog and Endpoints cards.

**Rationale:** The log is a primary monitoring tool, not a secondary reference. It should be visible at a glance on the main Dashboard, not hidden behind a click. A `<pre>` element with a fixed `max-height` and `overflow-y: scroll` provides a contained, scrollable log view that coexists with other dashboard content. Auto-refresh runs every 10 seconds when the Dashboard tab is active and the "Auto" checkbox is checked. This design treats the Dashboard as a monitoring console: status header at the top, key metrics in the middle, live log at the bottom.

**Outcome:** The inline log became one of the most-used parts of the UI during testing. It revealed real-time feedback about collector behavior, watchdog events, and configuration changes without any navigation. The auto-refresh checkbox gave users control over whether they wanted a live view or a frozen snapshot for analysis.

---

### Connection Snippets as a Modal

**Context:** Users need copy-paste configuration snippets for integrating the agent with Claude Code MCP, Claude Desktop, Home Assistant, and Prometheus. These are reference material used once during setup.

**Options considered:**
- Inline table always visible on the Dashboard -- would consume significant space for information used only during initial setup
- Modal triggered by a "Connect" button in the persistent header
- Separate "Connections" tab

**Decision:** Modal triggered by a header button.

**Rationale:** Connection snippets are used once during setup and then essentially never again. Putting them on the Dashboard would waste space every single time the user opens the page. A modal keeps the Dashboard clean while making the information accessible from any tab (since the header persists across tab switches). Each row in the modal has a Copy button using the clipboard API with a fallback for older browsers.

**Outcome:** The "Connect" button in the header serves double duty: it signals to new users that integrations are available, and it provides instant access to the snippets without cluttering any tab's layout.

---

### Status Bar: Compact Persistent Header

**Context:** Service status information -- running or stopped, uptime, version, crash count -- needs to be always visible regardless of which tab the user is on. The question was how much screen real estate to dedicate to this.

**Options considered:**
- Full "Status" section as the first content block inside the Dashboard tab -- visible only on one tab
- Compact header bar above the tab row, persistent across all tabs
- Status indicators embedded within the tab bar itself -- clever but cramped

**Decision:** Compact persistent header bar above the tabs.

**Rationale:** The header shows the essentials: a status orb (green/red), RUNNING/STOPPED text, uptime, version, and crash count, plus Start/Stop/Restart buttons and the Connect button. It persists across tab switches so the user always knows the service state. A key detail: meta information (uptime, version, crash count) hides when the service is stopped, because those values are meaningless in that state. This prevents confusion -- a stopped service showing "uptime: 3h 42m" from the last run would be misleading.

**Outcome:** The persistent header became the anchor of the entire UI. It eliminated the need for status checks elsewhere and made the Start/Stop/Restart buttons always one click away, regardless of which configuration tab the user was browsing.

---

## Collector Management

### Bidirectional Sync Between Toggle and Interval

**Context:** The agent binary supports two independent mechanisms for disabling a collector: the `--disable-collectors gpu,ups` command-line flag (which takes a comma-separated list), and setting an individual collector's interval to 0 (which some collectors interpret as "don't run"). The UI exposes both: a toggle switch (enable/disable) and an interval dropdown. The problem is that these two mechanisms can contradict each other -- a collector could appear "enabled" by its toggle but have interval=0, or appear in the disable list but have a non-zero interval.

**Options considered:**
- Use only `DISABLE_COLLECTORS` -- simpler, but loses the granularity of per-collector intervals and doesn't match the binary's internal behavior
- Use only interval=0 to signal disabled -- but the binary also checks the disable list, so both need to be correct
- Sync both directions: toggling off sets interval=0 AND adds to the disable list; setting interval to 0 unchecks the toggle AND adds to the disable list

**Decision:** Full bidirectional sync between the toggle, the interval dropdown, and the `DISABLE_COLLECTORS` list.

**Rationale:** The binary checks both mechanisms internally, so the UI must keep them in agreement. The PHP code rebuilds `$disabled_arr` to include any collector whose interval is 0. The JavaScript function `rebuildDisableCollectors()` builds the comma-separated list from unchecked toggles. When a toggle is unchecked, JS also sets the interval dropdown to 0 and disables it (graying it out). When a dropdown is changed to 0, JS unchecks the toggle. This bidirectional sync prevents the confusing state where a collector appears "enabled" by its toggle but won't actually run because its interval is 0. It's more complex to implement, but it eliminates an entire class of user confusion.

**Outcome:** The sync works smoothly in practice. Users can disable collectors via either the toggle or the dropdown and the other control updates immediately. The `DISABLE_COLLECTORS` hidden input always reflects the true state. No reports of inconsistent collector behavior since implementing the sync.

---

### Three-Column Collector Grid with Semantic Grouping

**Context:** The Collectors tab needs to display 13 togglable collectors (system, array, disk, network, docker, vm, gpu, ups, nut, hardware, shares, zfs, unassigned) plus 4 internal collectors that only have interval dropdowns (notifications, license, fan control, tuning). The original layout was a single-column list that required scrolling.

**Options considered:**
- Single-column list (original) -- simple but long
- Two-column grid -- better, but still requires scrolling with 17 items
- Three-column grid with semantic section headers

**Decision:** Three-column grid with section headers, after Scott explicitly requested: "can we make it wider, use 3 columns?"

**Rationale:** The three columns allow semantic grouping that maps to how users think about their system:
- Column 1: System Monitoring (system, array, disk, network) and Containers & VMs (docker, vm)
- Column 2: Hardware (gpu, ups, nut, hardware) and Internal (notifications, license, fan control, tuning)
- Column 3: Storage (shares, zfs, unassigned)

Internal collectors (notifications, license, fan control, tuning) have only interval dropdowns with no enable/disable toggle, because they're core to the agent's operation -- you can change how often they run but you can't turn them off. A spacer `<span>` replaces the toggle to maintain visual alignment with the togglable collectors above them.

**Outcome:** All 17 collectors fit on a single screen without scrolling. The semantic grouping means users can quickly find the collector they're looking for without scanning a long list. The internal collectors' toggle-less design subtly communicates their non-optional nature.

---

### Low Power Mode Placement on the Collectors Tab Header

**Context:** Low Power Mode multiplies all collector intervals by 4x, effectively reducing the agent's resource consumption. It was initially placed on the Dashboard as a quick-config option. The question was where it really belongs.

**Options considered:**
- Dashboard quick-config section -- visible on the main page, but conceptually disconnected from the intervals it modifies
- Advanced tab -- grouped with other expert settings
- Collectors tab header, right-aligned

**Decision:** Collectors tab header, right-aligned with a "Low Power (4x intervals)" label.

**Rationale:** Scott caught this placement issue directly, asking "and low power?" after discussing fan management. Low Power Mode directly affects collector intervals -- it's a global modifier on the very settings displayed on the Collectors tab. Placing it in the tab header (not inside a card, but above the collector grid) makes it a visible global modifier. The header placement communicates: "this affects everything below." Right-alignment keeps it out of the way of the tab title while remaining prominent.

**Outcome:** The placement is intuitive. Users adjusting collector intervals can see immediately whether Low Power Mode is active, which is critical since it multiplies every interval they're setting by 4. No reports of users being confused about why their intervals seemed "wrong."

---

## Watchdog

### Direct Cron File Instead of Unraid's update_cron API

**Context:** Unraid doesn't use systemd (it runs SysV init), so there's no `systemd timer` available for periodic tasks. The plugin needs a mechanism to periodically check if the agent process is alive and restart it if it has crashed.

**Options considered:**
- Systemd timer -- not available on Unraid; a non-starter
- Unraid's `update_cron` API function -- the "blessed" way to add cron jobs on Unraid, but adds a dependency on Unraid's internal PHP API from shell scripts
- Direct cron file in `/etc/cron.d/` with `killall -HUP crond` to reload

**Decision:** Direct cron file at `/etc/cron.d/unraid-management-agent-watchdog` with `killall -HUP crond` to reload the cron daemon.

**Rationale:** Writing directly to `/etc/cron.d/` is simpler and gives full control over the cron interval without depending on Unraid's internal API. The cron file is a single line that runs the watchdog script at the configured interval. The file is created by `scripts/start` and `scripts/apply`, and removed by `scripts/stop`. The `killall -HUP crond` after writing or removing the file ensures crond picks up the change immediately. This approach has no PHP dependency, works on any Linux system with crond, and is trivially debuggable (`cat /etc/cron.d/unraid-management-agent-watchdog`).

**Outcome:** The cron-based watchdog works reliably. The direct file approach proved simpler to reason about than the `update_cron` API would have been, and the stop-script ordering (discussed next) cleanly prevents race conditions.

---

### Stop Script: Remove Cron BEFORE Killing the Process

**Context:** A race condition exists between the watchdog cron and the stop script. If the watchdog cron fires during the window between when the stop script starts and when the process actually dies, the watchdog would see the agent as dead and restart it -- defeating the stop command.

**Options considered:**
- Remove cron after stopping the process -- creates a window where the watchdog can fire and undo the stop
- Remove cron before stopping the process -- eliminates the race
- Use a PID lock file to signal "deliberate stop" -- more complex, requires the watchdog to check the lock file

**Decision:** Remove cron BEFORE stopping the process.

**Rationale:** This is the simplest race-free approach, requiring zero coordination mechanisms. The stop script's very first action is removing the cron file and sending HUP to crond. Only after crond has reloaded (and the watchdog cron entry is gone) does the script proceed to `killall` the agent process. The ordering guarantee means there is no window during which the watchdog could fire and restart the agent. No lock files, no signal coordination, no special state management -- just correct ordering of two operations.

**Outcome:** No reports of the "stop command doesn't stick" problem that plagues plugins with incorrect cron/process shutdown ordering. The simplicity of the fix makes it easy to verify by code review.

---

### Watchdog Throttle: 5 Crashes in 5 Minutes

**Context:** A watchdog that unconditionally restarts the agent on every check creates a dangerous failure mode: if the binary is broken or misconfigured, the watchdog enters an infinite restart loop, potentially consuming resources and filling logs indefinitely.

**Options considered:**
- No throttle -- let cron keep restarting forever. Simple but dangerous.
- Simple counter with no time window -- a counter that increments on each crash and stops after N. But transient failures would permanently exhaust the counter.
- Time-windowed counter: N crashes within M minutes triggers throttle. Allows recovery from transient failures while catching persistent ones.
- Exponential backoff -- increasing delays between restart attempts. More complex, harder to reason about.

**Decision:** 5 crashes within a 5-minute window triggers throttle mode, halting restarts.

**Rationale:** The watchdog uses `MAX_CRASHES=5` and `THROTTLE_WINDOW=300` seconds. On each run, it reads the crash log, counts entries containing "not running" within the last 300 seconds, and skips the restart if the count exceeds 5. This design allows occasional transient crashes (a collector hitting a race condition, a brief resource exhaustion) to be recovered automatically, while catching a fundamentally broken binary that crashes immediately on startup. The time window is critical: it means the throttle resets after 5 minutes of stability, so a transient issue at 2 AM doesn't prevent recovery from an unrelated issue at 6 AM. The throttle logs its own decision to the crash log, so the user can see why the agent isn't being restarted.

**Outcome:** The throttle parameters proved well-calibrated during testing. Normal operation never triggers the throttle, while a deliberately broken config hit it within 30 seconds of being applied.

---

### Smart Restart on Apply: The Fan Incident

**Context:** Clicking "Apply" on the settings page triggers the apply script. The initial implementation always restarted the agent on Apply, regardless of what changed. Scott reported that the fans "went crazy" after toggling the watchdog interval -- a change that doesn't affect the running agent at all, only the cron schedule. Investigation showed the agent had NOT restarted (PID unchanged) -- the fan spin-up was caused by the BIOS fan curve for CHA_FAN3 (pwm5) on the ASUS Z790-V AX, which has a 60% minimum floor at 20C. But the incident prompted a closer look at the Apply script, revealing that it unconditionally restarted the agent on every Apply. That meant any future Apply touching non-watchdog settings WOULD restart the agent and trigger fan_control re-initialization. The question became: should Apply always restart the agent?

**Options considered:**
- Always restart on Apply -- simple, guarantees the agent picks up all changes, but causes unnecessary disruption (including the fan spin-up) when only non-agent settings changed
- Never restart, require manual restart -- safe but terrible UX; users would forget and wonder why their config changes aren't taking effect
- Smart restart: compare config hashes excluding WATCHDOG fields, only restart if agent-relevant settings changed

**Decision:** Smart restart with hash comparison.

**Rationale:** The apply script maintains a snapshot of the running configuration at `/tmp/unraid-management-agent-running.cfg`. On each apply, it compares the new configuration (minus WATCHDOG-prefixed lines) against this snapshot. If they differ, the agent settings have actually changed and a restart is needed. If only WATCHDOG fields changed, the script updates the cron schedule and skips the restart. This approach directly addresses the fan incident: toggling the watchdog interval now updates the cron file without touching the agent process, so no fan spin-up, no collector re-initialization, no brief gap in monitoring.

**Outcome:** The smart restart eliminated the unnecessary fan disruption and reduced apply-time restarts significantly. In practice, most "quick tweaks" involve watchdog or cron settings and no longer cause a full agent restart. The `/tmp/` snapshot is on tmpfs and is recreated on each boot, so there's no stale-config risk after a reboot.

---

## Security

### env(1) Instead of bash -c for Launching the Binary

**Context:** The original start script used `sudo -H bash -c "..."` with double-quoted shell interpolation to pass configuration values as environment variables to the agent binary. A security audit flagged this as Critical severity (C-2): any config value containing shell metacharacters (backticks, `$()`, semicolons) would be executed as shell commands.

**Options considered:**
- Keep `bash -c` but add quoting and escaping around each variable -- fragile, error-prone, easy to miss one
- Use the `env` command to set environment variables before the binary -- passes key=value pairs without shell interpretation
- Write a temporary env file and source it -- adds file management complexity and another potential attack surface

**Decision:** Replace the entire launch block with `nohup env VAR=val ... "$PROG" args`.

**Rationale:** `env(1)` passes variables as literal key=value pairs without any shell interpretation. There is no interpolation into a shell string, so no quote-escaping is needed and no metacharacters are dangerous. The binary reads its configuration from environment variables, so this is the natural and correct approach. The fix is both more secure and simpler than the original code. A comment in the script documents the rationale: "Use env(1) to pass variables safely instead of shell interpolation."

**Outcome:** The critical injection vulnerability was eliminated. The start script became shorter and easier to audit. This is a case where the secure approach was also the simpler approach -- a rare and welcome alignment.

---

### Input Sanitization in Shell with Four Helper Functions

**Context:** The security audit found that configuration values flow from the web form POST, through PHP's `update.php` which writes them to a config file, and then into shell scripts that source the config file. Any malicious or malformed value in the config file could affect shell behavior.

**Options considered:**
- Validate in PHP before writing the config file (in `update.php`) -- catches issues at the source, but doesn't protect against manual config file edits
- Validate in bash after sourcing the config file (in `scripts/start`) -- protects against all sources of config values
- Validate in both places -- belt and suspenders, but doubles the maintenance burden
- Use an allowlist in the binary itself -- the binary already validates some values, but the shell scripts run before the binary starts

**Decision:** Validate in bash in `scripts/start` using four helper functions.

**Rationale:** The shell scripts are the last line of defense before values are used, so that's where sanitization must happen. Four functions cover all value types in the config:
- `sanitize_int()` strips everything except digits via `tr -cd '0-9'`
- `sanitize_csv()` strips everything except `a-zA-Z0-9,_-` (for comma-separated collector lists)
- `sanitize_str()` strips shell metacharacters: single quotes, double quotes, backticks, dollar signs
- `sanitize_bool()` returns "true" only if the input is literally "true"; everything else becomes "false"

All config values pass through the appropriate sanitizer immediately after sourcing. `WATCHDOG_INTERVAL` gets an additional `case` statement clamping to the allowlist `1|2|5|10|15|30`. Interval variables are sanitized in a `for` loop using `eval` to avoid 17 repetitive lines of identical sanitization calls. This is a pragmatic compromise -- `eval` in a sanitization loop is slightly unusual, but the alternative was 17 copy-pasted lines that would inevitably drift.

**Outcome:** All config values are sanitized before use. The helper functions are reusable and testable. The `eval` loop is documented with a comment explaining why it's used. No injection vectors remain through the config file path.

---

### ANSI Log Colors: Convert to HTML Spans (Not Strip)

**Context:** The agent's log file contains ANSI escape codes for colored output (red for errors, green for success, blue for info). Displaying raw ANSI in a `<pre>` tag shows garbage characters like `[31m`. The log needed to render cleanly in the browser.

**Options considered:**
- Strip all ANSI codes and show plain text -- simplest, but loses useful color information
- Convert ANSI codes to HTML `<span>` tags with inline color styles -- preserves the visual meaning of colors
- Use a JavaScript library like `ansi_up` -- adds a dependency for a relatively simple task

**Decision:** Convert ANSI codes to HTML spans, implemented both server-side (PHP) and client-side (JavaScript).

**Rationale:** Colors in logs carry meaning. Red errors, green successes, and blue info lines are significantly easier to scan than a wall of monochrome text. Stripping colors would degrade the user experience. A dedicated JS library would be overkill for the 6 color codes the agent actually uses. The PHP side uses `preg_replace_callback` to map ANSI codes to HTML `<span>` tags, but critically runs `htmlspecialchars` on the raw text first -- before processing ANSI codes -- to prevent XSS through crafted log messages. The JavaScript side has a matching `ansiToHtml()` function for AJAX log refreshes. A hardcoded color map covers codes 31-36 (red, green, yellow, blue, magenta, cyan).

**Outcome:** Log output is colorful and readable in the browser. The XSS prevention ordering (escape HTML first, then process ANSI codes) is correct and was verified during the security audit. The dual implementation (PHP for initial page load, JS for AJAX refreshes) ensures colors work consistently.

---

### CORS Wildcard Default Left As-Is

**Context:** The security audit flagged `CORS_ORIGIN="*"` as Medium severity (M-3), recommending a more restrictive default. The agent's API accepts requests from any origin by default.

**Options considered:**
- Change the default to an empty string (most restrictive) -- would break the most common use case out of the box
- Change the default to the Unraid server's own address -- would require auto-detection and wouldn't cover the primary use case of remote tool access
- Keep `*` as the default with a warning in the UI

**Decision:** Left as-is with `*` as the default.

**Rationale:** This was a pragmatic compromise. The agent has no built-in authentication -- it's designed to be accessed by tools on the local network (Claude Code running on a laptop, Home Assistant on another device, Prometheus on a monitoring server). Restricting CORS by default would break the most common use case and generate support requests from users who can't figure out why their MCP connection fails. Instead, the Advanced tab includes a security info box noting: "The agent has no built-in authentication. For external access, use a reverse proxy with Basic Auth or SSO." This is honest about the security posture rather than pretending a CORS restriction on an unauthenticated API provides meaningful security.

**Outcome:** The default works out of the box for the target audience. The security info box sets appropriate expectations. A future version may add token-based authentication, at which point tightening the CORS default would make more sense.

---

## Form and Rendering

### Keeping form markdown="1" for Unraid Compatibility

**Context:** Unraid uses Parsedown (a Markdown parser) to process `.page` files. The `form markdown="1"` attribute tells Parsedown to process the form's content as Markdown. This can mangle raw HTML -- Parsedown will interpret underscores as emphasis, lines starting with `#` as headers, and certain HTML structures in unexpected ways. However, removing the attribute might break Unraid's form processing machinery.

**Options considered:**
- Remove `markdown="1"` and use pure HTML -- risky because the `#file`, `#include`, and `#command` hidden inputs that Unraid's `/update.php` processes might depend on Parsedown's behavior
- Keep `markdown="1"` and carefully structure HTML to avoid triggering Parsedown interpretation
- Use PHP to render everything dynamically before Parsedown sees the output

**Decision:** Keep `markdown="1"` for compatibility with Unraid's expected form processing patterns.

**Rationale:** This was driven entirely by Unraid platform constraints. Removing `markdown="1"` is a risk with unknown consequences -- the `#file`, `#include`, and `#command` hidden inputs are processed by Unraid's `/update.php` handler, and their behavior may depend on Parsedown having already processed the form. Rather than experimenting with a change that could break form submission, the page uses PHP to render all dynamic content before Parsedown runs, and the HTML structure carefully avoids Parsedown triggers: no bare underscores outside PHP blocks, no lines starting with `#` outside PHP blocks, and HTML block elements that Parsedown recognizes as HTML (like `<blockquote class="inline_help">`) rather than trying to parse as Markdown.

**Outcome:** The form renders correctly and submits correctly. The inline help blocks work as expected. The main cost is that developers working on the page need to be aware of Parsedown's behavior and avoid triggering it inadvertently -- a constraint that is documented in code comments.

---

### Version Detection: Fallback Chain

**Context:** The version number showed "Unknown" in the status header because no version file existed at the expected path. The version needs to come from somewhere.

**Options considered:**
- Parse the `.plg` XML file to extract the version attribute -- the `.plg` file lives on `/boot` (the USB flash drive) and parsing XML in PHP for a single value is heavyweight
- Look for a `VERSION` file (all caps) at the plugin's install path
- Look for a `version.txt` file at the plugin's install path
- Hardcode the version in the PHP -- requires manual updates on every release

**Decision:** Fallback chain: check for `VERSION` first, then `version.txt`.

**Rationale:** Different plugin build processes create version files with different names. Rather than mandate one convention and risk breaking if it changes, the PHP checks both paths. The `.plg` file was rejected because it lives on `/boot` which may have slower access characteristics and because XML parsing for a single attribute is disproportionate. Hardcoding was rejected because it creates a maintenance burden and a guaranteed source of version mismatches. The version string is escaped with `htmlspecialchars` before rendering.

**Outcome:** The version displays correctly in the header. The fallback chain handles both naming conventions gracefully. If neither file exists, "Unknown" is shown -- which serves as a clear signal that the build process didn't create a version file, rather than silently showing a stale hardcoded value.

---

### Fixed Footer for Apply/Default/Done Buttons

**Context:** The Apply, Default, and Done buttons were invisible. Investigation revealed that `overflow: hidden` on the form wrapper (part of the tabbed layout implementation) was clipping the button footer, which was positioned at the natural end of the form content. Users could save no configuration changes because the buttons simply weren't visible.

**Options considered:**
- Put buttons inside the scrollable content area -- they would scroll out of view on long tab content, and their position would vary by tab
- Use `position: sticky` at the bottom -- browser support is good but sticky positioning interacts unpredictably with `overflow: hidden` on parent elements
- Use `position: fixed` at the bottom of the viewport -- always visible regardless of content or scroll position

**Decision:** `position: fixed` with `bottom: 30px` and a `border-top` separator.

**Rationale:** This decision was driven directly by a bug: the buttons were invisible, making the entire settings page non-functional. Fixed positioning was the only option that guaranteed visibility regardless of the `overflow: hidden` on the form wrapper, the varying content height of different tabs, and the user's scroll position. The `bottom: 30px` offset accounts for Unraid's own fixed footer bar (which contains the array status). A `border-top` and matching background color provide visual separation from the tab content above.

**Outcome:** The buttons are always visible and always accessible. The fixed positioning works correctly across all three tabs and at all viewport sizes. This was one of those fixes where the symptom (buttons invisible) was dramatic but the cause (overflow clipping) and fix (fixed positioning) were straightforward once diagnosed.

---

## Log and Monitoring

### Server Clock Ticker

**Context:** Scott asked while reviewing the log output: "for the logs: is that server time? or was that an hour ago?" The log timestamps are in server time (the Unraid box's local time), but the user was browsing from a machine in a potentially different timezone. Without a reference point, the timestamps were ambiguous.

**Options considered:**
- Convert individual log timestamps to the browser's local time -- would require parsing each timestamp from the log output, which is fragile and depends on the log format never changing
- Show a live server clock somewhere on the page -- gives the user a reference point to interpret all timestamps
- Show both server and local time -- adds clutter for a problem that's solved by either approach alone

**Decision:** Add a "Server: YYYY/MM/DD HH:MM:SS" ticker next to the log header that updates every second.

**Rationale:** Converting individual timestamps is fragile: log lines come from multiple sources with potentially different formats, and a regex that works today might break if the agent's log format changes. Instead, the page embeds the server's Unix epoch in PHP (`<?= time() ?>`) and ticks it forward locally in JavaScript with a 1-second `setInterval`. The ticker gives the user a single reference point from which they can interpret any timestamp in the log. A separate `setInterval` also ticks the uptime display in the status header, keeping both counters live without any AJAX calls.

**Outcome:** Users can immediately see the server's current time and compare it to log timestamps. The approach is zero-maintenance -- it doesn't depend on the log format, doesn't require any server-side changes, and the slight clock drift from JS-side ticking (a few seconds over hours) is inconsequential for this use case.

---

### Auto-Refresh: AJAX Polling with Toggle

**Context:** Scott asked: "does this refresh automatically at all? if so is it a soft refresh in the log viewer box?" The log panel was initially static -- it showed whatever was in the log at page load and never updated.

**Options considered:**
- No auto-refresh (manual only) -- simple but forces users to reload the page or click a refresh button to see new log entries
- WebSocket-based live stream -- would provide real-time updates but requires changes to the agent binary to support WebSocket connections from the UI
- AJAX poll every N seconds -- simple, uses existing infrastructure, no binary changes needed

**Decision:** AJAX poll every 10 seconds, togglable with an "Auto" checkbox, only active when the Dashboard tab is visible.

**Rationale:** WebSocket would require adding a WebSocket endpoint to the agent binary specifically for the settings page log viewer -- a significant change for a convenience feature. AJAX polling is simple and uses the existing Unraid `include/exec.php` endpoint (with an `action: 'log'` parameter). The "Auto" checkbox defaults to checked, giving users a live view by default while allowing them to freeze the log for analysis. Polling only runs when the Dashboard tab is active, avoiding unnecessary requests when the user is configuring collectors or advanced settings. The refresh preserves scroll position: if the user was scrolled to the bottom, the view stays at the bottom after refresh (tracking new entries); if the user scrolled up to examine older entries, the scroll position is preserved.

**Outcome:** The 10-second interval balances freshness against server load. The scroll-position preservation was a small detail that made a big UX difference -- without it, every refresh would jump the user back to the top or bottom, making it impossible to read older log entries.

---

### Crash Log Rotation in the Watchdog Script

**Context:** The security audit flagged unbounded crash log growth as a Low severity issue (L-2). On Unraid, `/var/log` is tmpfs (backed by RAM, not disk). An unbounded log file growing in RAM could, in theory, contribute to memory pressure on systems with limited RAM.

**Options considered:**
- No rotation (the original implementation) -- simple but risks unbounded growth
- Rotate by file size -- requires checking file size, slightly more complex
- Rotate by line count -- simple to implement, predictable memory usage
- Use `logrotate` -- adds a dependency on logrotate configuration, which may not persist across Unraid reboots (since `/etc` is also tmpfs in parts)

**Decision:** Rotate by line count inside the watchdog script itself.

**Rationale:** `MAX_LOG_LINES=500`. At the start of each watchdog run, if the crash log exceeds 500 lines, it's truncated to the most recent 200 lines using `tail -n 200`. This runs inside the watchdog script, which is already reading the file for throttle checks, so there's no additional file I/O overhead beyond what's already happening. It requires no external `logrotate` configuration (which would need to be recreated on each Unraid boot since parts of `/etc` are tmpfs). The 500/200 thresholds mean the file is never larger than 500 lines and retains the most recent 200 lines of history after rotation, providing enough context for debugging without unbounded growth.

**Outcome:** Crash log size is bounded. The rotation is invisible to the user -- it happens as a side effect of the watchdog's normal operation. The self-contained approach (no external config files, no dependencies) fits Unraid's ephemeral filesystem model where configuration that isn't on the boot USB doesn't survive a reboot.

---

## Case Study: The Fan Incident

### Should Apply Always Restart the Agent?

**Context:** Scott reported that the server's fans "went crazy" after toggling the watchdog interval in the settings UI and clicking Apply. This was alarming -- uncontrolled fan behavior suggests a thermal emergency. SSH investigation showed CPU temperatures were normal (low 30s Celsius) and no unusual processes were running. The fans returned to normal after about 30 seconds.

**Root cause investigation:**
- The agent did NOT restart during this incident -- the PID was unchanged. Scott only toggled the watchdog cron, which does not restart the agent process
- The fan spin-up was caused by the BIOS fan curve for CHA_FAN3 (pwm5) on the ASUS Z790-V AX, which enforces a 60% minimum floor at 20C -- a hardware-level behavior unrelated to the agent
- However, the investigation revealed a latent design flaw: the Apply script unconditionally restarted the agent on every Apply, even when only the watchdog interval changed. Any future Apply touching agent-relevant settings WOULD cause an unnecessary restart -- and the resulting fan_control collector re-initialization could cause a similar fan disruption

**Decision:** The fan spin-up was not caused by the agent, but the investigation revealed a design flaw in the Apply script. The fix was the smart restart mechanism (Decision #12): compare config hashes excluding WATCHDOG fields and only restart if agent-relevant settings actually changed. This was built preventively -- to avoid triggering fan disruption on future config changes -- not as a fix for this specific incident.

**Rationale:** A user changing the watchdog check interval from 2 minutes to 5 minutes should not cause any observable effect on the running system -- the agent doesn't even know or care about the watchdog interval; it's purely external supervision. Without smart restart, any Apply that touched agent-relevant settings would unconditionally restart the process, causing the fan_control collector to re-initialize and briefly set fans to a different speed profile before reading the configured settings. The smart restart ensures that supervisory-only changes don't disrupt the running agent, and that agent restarts only happen when genuinely needed.

**Outcome:** After implementing smart restart, toggling the watchdog interval applies the change to the cron schedule without restarting the agent. The fans during the original incident were explained by the BIOS fan curve on CHA_FAN3, not by agent behavior -- but the incident was a productive catalyst: it exposed the unconditional-restart pattern and motivated a fix that prevents unnecessary restarts on systems with physical actuators (fans, UPS, GPU power management) that respond to collector initialization.
