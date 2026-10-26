#!/bin/sh
# NodeDesk E-Ink client for jailbroken Kindles.
#
#   dashboard.sh start   take over the screen and refresh in the background
#   dashboard.sh stop    give the screen back to the Kindle UI
#   dashboard.sh once    fetch and draw a single frame (for testing)
#
# While it runs, press the power button to stop it and get the Kindle UI back.

DIR=$(dirname "$(readlink -f "$0")")
. "$DIR/config.sh"

PIDFILE=/tmp/nodedesk-eink.pid
LOG=/tmp/nodedesk-eink.log
IMG=/tmp/nodedesk-eink.png
FBINK=/mnt/us/libkh/bin/fbink
[ -x "$FBINK" ] || FBINK=$(command -v fbink)

log() { echo "$(date '+%F %T') $*" >> "$LOG"; }

screen_size() {
    if [ -z "$WIDTH" ] || [ -z "$HEIGHT" ]; then
        WIDTH=$(eips -i 2>/dev/null | sed -n 's/.*xres: *\([0-9]*\).*/\1/p' | head -1)
        HEIGHT=$(eips -i 2>/dev/null | sed -n 's/.*yres: *\([0-9]*\).*/\1/p' | head -1)
    fi
    [ -n "$WIDTH" ] || WIDTH=1236
    [ -n "$HEIGHT" ] || HEIGHT=1648
}

# draw FILE [flash]
draw() {
    if [ -n "$FBINK" ]; then
        if [ "$2" = flash ]; then "$FBINK" -q -c -f -g file="$1"; else "$FBINK" -q -g file="$1"; fi
    else
        [ "$2" = flash ] && eips -c
        eips -g "$1"
    fi
}

# Small status line at the bottom, drawn over the last frame.
notice() {
    if [ -n "$FBINK" ]; then "$FBINK" -q -m -y -2 "$1"; else eips 1 -2 "$1"; fi
}

wait_wifi() {
    i=0
    while [ "$(lipc-get-prop com.lab126.wifid cmState 2>/dev/null)" != CONNECTED ]; do
        i=$((i + 1))
        [ $i -gt 30 ] && return 1
        [ $i -eq 1 ] && lipc-set-prop com.lab126.cmd wirelessEnable 1 2>/dev/null
        sleep 1
    done
}

fetch() {
    battery=$(lipc-get-prop com.lab126.powerd battLevel 2>/dev/null)
    charging=$(lipc-get-prop com.lab126.powerd isCharging 2>/dev/null)
    url="$SERVER/dashboard.png?w=$WIDTH&h=$HEIGHT&lang=$LANGUAGE&battery=$battery&charging=$charging"
    [ -n "$TOKEN" ] && url="$url&token=$TOKEN"
    curl -fsS -m 30 -o "$IMG.tmp" "$url" 2>> "$LOG" && mv "$IMG.tmp" "$IMG"
}

run() {
    screen_size
    log "start ${WIDTH}x${HEIGHT} every ${INTERVAL}s from $SERVER"
    n=0
    while :; do
        if wait_wifi && fetch; then
            if [ $((n % FULL_REFRESH_EVERY)) -eq 0 ]; then draw "$IMG" flash; else draw "$IMG"; fi
        else
            log "update failed"
            notice "NodeDesk: no connection to $SERVER ($(date +%H:%M))"
        fi
        n=$((n + 1))
        sleep "$INTERVAL"
    done
}

# Stop when the power button is pressed. The GUI is down, so powerd only
# reports the event; nobody draws the screensaver.
watch_power() {
    lipc-wait-event com.lab126.powerd goingToScreenSaver,outOfScreenSaver > /dev/null 2>&1 && sh "$0" stop
}

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; }

start_dashboard() {
    if running; then
        echo "already running"
        return 0
    fi
    : > "$LOG"
    lipc-set-prop com.lab126.powerd preventScreenSaver 1
    initctl stop lab126_gui > /dev/null 2>&1
    sleep 3 # let the framework release the screen
    setsid sh "$0" run < /dev/null >> "$LOG" 2>&1 &
    echo $! > "$PIDFILE"
    setsid sh "$0" watch < /dev/null > /dev/null 2>&1 &
    echo $! > "$PIDFILE.watch"
}

stop_dashboard() {
    for f in "$PIDFILE" "$PIDFILE.watch"; do
        [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null
        rm -f "$f"
    done
    log "stop"
    lipc-set-prop com.lab126.powerd preventScreenSaver 0
    initctl start lab126_gui > /dev/null 2>&1
}

case "$1" in
    start) start_dashboard ;;
    stop) stop_dashboard ;;
    run) run ;;
    watch) watch_power ;;
    once) screen_size && fetch && draw "$IMG" flash ;;
    *) echo "usage: $0 start|stop|once" >&2; exit 2 ;;
esac
