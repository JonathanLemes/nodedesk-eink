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
    # Take the screen from the Kindle UI. Done here, not in start, because
    # stopping the UI also kills the Library launcher that called start.
    lipc-set-prop com.lab126.powerd preventScreenSaver 1
    # The UI reopens the last active app when it comes back. Started from
    # the Library, that is the blank script launcher, so go Home first.
    go_home
    sleep 3
    initctl stop lab126_gui > /dev/null 2>&1
    sleep 3 # let the framework release the screen
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

# The input device of the power key, e.g. /dev/input/event0 ("bd71828-pwrkey").
power_key() {
    for d in /sys/class/input/event*; do
        case "$(cat "$d/device/name" 2>/dev/null)" in
            *pwrkey* | *power* | *Power*) echo "/dev/input/${d##*/}"; return ;;
        esac
    done
}

# Stop on the first power key event. With the UI down powerd does not report
# the press, so read the key device directly (one input_event is 16 bytes).
watch_power() {
    key=$(power_key)
    [ -n "$key" ] || { log "power key not found, use: $0 stop"; return 1; }
    dd if="$key" of=/dev/null bs=16 count=1 2> /dev/null || return 1
    log "power key pressed"
    setsid sh "$0" stop < /dev/null > /dev/null 2>&1
}

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; }

gui_stopped() { initctl status lab126_gui 2>/dev/null | grep -q stop/; }

start_dashboard() {
    if running; then
        echo "already running"
        return 0
    fi
    : > "$LOG"
    # setsid makes each job its own process group, so stop can end it with
    # everything it spawned (sleep, dd, curl).
    setsid sh "$0" run < /dev/null >> "$LOG" 2>&1 &
    echo $! > "$PIDFILE"
    setsid sh "$0" watch < /dev/null >> "$LOG" 2>&1 &
    echo $! > "$PIDFILE.watch"
}

stop_dashboard() {
    for f in "$PIDFILE" "$PIDFILE.watch"; do
        [ -f "$f" ] && kill -TERM -"$(cat "$f")" 2>/dev/null
        rm -f "$f"
    done
    log "stop"
    lipc-set-prop com.lab126.powerd preventScreenSaver 0
    gui_stopped || return 0
    if [ -n "$FBINK" ]; then
        "$FBINK" -q -c -f
        if [ "$LANGUAGE" = pt ]; then msg="Voltando para o Kindle..."; else msg="Back to the Kindle..."; fi
        "$FBINK" -q -m -M "$msg"
    else
        eips -c
    fi
    initctl start lab126_gui > /dev/null 2>&1
    setsid sh "$0" repaint < /dev/null > /dev/null 2>&1 &
}

# The UI takes about a minute to come back. Xorg kept running the whole time
# and never saw the dashboard, so it may skip repainting some areas. Once the
# UI has drawn its status bar (the top rows are no longer blank), ask every
# window to repaint.
go_home() { lipc-set-prop com.lab126.appmgrd start app://com.lab126.booklet.home 2> /dev/null; }

repaint() {
    i=0
    while [ "$(dd if=/dev/fb0 bs="$WIDTH" count=80 2> /dev/null | tr -d '\377' | wc -c)" -lt 500 ]; do
        i=$((i + 1))
        [ $i -gt 60 ] && break
        sleep 2
    done
    sleep 5
    # Safety net: never leave the user on the launcher's blank page.
    case "$(lipc-get-prop com.lab126.appmgrd activeApp 2> /dev/null)" in
        *shell_integration*) go_home && sleep 5 ;;
    esac
    DISPLAY=:0 xrefresh > /dev/null 2>&1
    log "UI back after $((i * 2 + 5))s ($(lipc-get-prop com.lab126.appmgrd activeApp 2> /dev/null))"
}

case "$1" in
    start) start_dashboard ;;
    stop) stop_dashboard ;;
    run) run ;;
    watch) watch_power ;;
    repaint) screen_size && repaint ;;
    once) screen_size && fetch && draw "$IMG" flash ;;
    *) echo "usage: $0 start|stop|once" >&2; exit 2 ;;
esac
