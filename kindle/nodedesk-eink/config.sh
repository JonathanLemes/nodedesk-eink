# NodeDesk E-Ink: Kindle settings. This file is sourced by dashboard.sh.
# shellcheck disable=SC2034 # variables are read by dashboard.sh

# Where nodedesk-eink is running (no trailing slash).
SERVER="http://192.168.0.10:8421"

# Optional: must match NODEDESK_EINK_TOKEN on the server.
TOKEN=""

# Language of the dashboard: en or pt.
LANGUAGE="en"

# Seconds between refreshes. The Kindle stays awake (Wi-Fi on), so keep it
# plugged in for 24/7 use.
INTERVAL=60

# Do a full (flashing) refresh every N updates to clear e-ink ghosting.
FULL_REFRESH_EVERY=10

# Leave empty to detect the screen size automatically.
WIDTH=""
HEIGHT=""
