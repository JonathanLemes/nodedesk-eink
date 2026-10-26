// Command nodedesk-eink serves a home-server dashboard as a PNG for e-ink
// readers such as a jailbroken Kindle.
//
//	nodedesk-eink                      # serve on :8421
//	nodedesk-eink render -o out.png    # draw one frame and exit
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/JonathanLemes/nodedesk-eink/internal/metrics"
	"github.com/JonathanLemes/nodedesk-eink/internal/render"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "render" {
		if err := renderCmd(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println(version)
		return
	}
	if err := serve(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func serve() error {
	addr := env("NODEDESK_EINK_ADDR", ":8421")
	token := os.Getenv("NODEDESK_EINK_TOKEN")
	lang := env("NODEDESK_EINK_LANG", "en")

	interval, err := time.ParseDuration(env("NODEDESK_EINK_INTERVAL", "10s"))
	if err != nil {
		return fmt.Errorf("NODEDESK_EINK_INTERVAL: %w", err)
	}
	disks, err := parseDisks(os.Getenv("NODEDESK_EINK_DISKS"))
	if err != nil {
		return err
	}

	col := metrics.NewCollector(metrics.Options{
		Interval:     interval,
		Disks:        disks,
		DockerSocket: dockerSocket(),
		Name:         os.Getenv("NODEDESK_EINK_NAME"),
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go col.Run(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /dashboard.png", func(w http.ResponseWriter, r *http.Request) {
		snap := col.Snapshot()
		if r.URL.Query().Has("demo") {
			snap = metrics.Demo(time.Now())
		}
		img := render.Render(snap, screenOptions(r, lang))
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		if err := png.Encode(w, img); err != nil {
			slog.Warn("encode png", "err", err)
		}
	})
	mux.HandleFunc("GET /metrics.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, col.Snapshot())
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		q := ""
		if token != "" {
			q = "?token=" + url.QueryEscape(r.URL.Query().Get("token"))
		}
		fmt.Fprintf(w, previewHTML, q)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           withToken(token, mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("nodedesk-eink listening", "addr", addr, "version", version)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func renderCmd(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	out := fs.String("o", "dashboard.png", "output file")
	demo := fs.Bool("demo", false, "use demo data instead of this machine")
	lang := fs.String("lang", env("NODEDESK_EINK_LANG", "en"), "language (en, pt)")
	width := fs.Int("w", 1236, "width in pixels")
	height := fs.Int("h", 1648, "height in pixels")
	battery := fs.Int("battery", -1, "battery level to show in the footer")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var snap metrics.Snapshot
	if *demo {
		// A fixed time keeps the README screenshot stable.
		snap = metrics.Demo(time.Date(2026, time.October, 26, 9, 41, 0, 0, time.Local))
	} else {
		disks, err := parseDisks(os.Getenv("NODEDESK_EINK_DISKS"))
		if err != nil {
			return err
		}
		col := metrics.NewCollector(metrics.Options{
			Interval:     time.Second,
			Disks:        disks,
			DockerSocket: dockerSocket(),
			Name:         os.Getenv("NODEDESK_EINK_NAME"),
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()
		col.Run(ctx) // two samples, so CPU and network rates are real
		snap = col.Snapshot()
	}

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	img := render.Render(snap, render.Options{Width: *width, Height: *height, Lang: *lang, Battery: *battery})
	return png.Encode(f, img)
}

// screenOptions reads the query string the Kindle sends:
// w, h (pixels), lang, battery (0-100) and charging (1/0).
func screenOptions(r *http.Request, lang string) render.Options {
	q := r.URL.Query()
	o := render.Options{Width: 1236, Height: 1648, Lang: lang, Battery: -1}
	if v, err := strconv.Atoi(q.Get("w")); err == nil && v >= 200 && v <= 4000 {
		o.Width = v
	}
	if v, err := strconv.Atoi(q.Get("h")); err == nil && v >= 200 && v <= 4000 {
		o.Height = v
	}
	if v := q.Get("lang"); v != "" {
		o.Lang = v
	}
	if v, err := strconv.Atoi(q.Get("battery")); err == nil && v >= 0 && v <= 100 {
		o.Battery = v
	}
	o.Charging = q.Get("charging") == "1"
	return o
}

// withToken requires ?token= (or a Bearer header) when a token is configured.
// /healthz stays open for monitoring.
func withToken(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("token")
		if got == "" {
			got = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		if r.URL.Path != "/healthz" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parseDisks reads "/=System,/mnt/media=Media". A mount without "=label"
// keeps its default label.
func parseDisks(v string) ([]metrics.DiskSpec, error) {
	var out []metrics.DiskSpec
	for item := range strings.SplitSeq(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		mount, label, _ := strings.Cut(item, "=")
		if !strings.HasPrefix(mount, "/") {
			return nil, fmt.Errorf("NODEDESK_EINK_DISKS: %q is not an absolute mount point", mount)
		}
		if label == "" && mount != "/" {
			label = mount[strings.LastIndex(mount, "/")+1:]
		}
		out = append(out, metrics.DiskSpec{Mount: mount, Label: label})
	}
	return out, nil
}

// dockerSocket returns NODEDESK_EINK_DOCKER, or the default socket when it
// exists. Set NODEDESK_EINK_DOCKER=off to hide the Docker card.
func dockerSocket() string {
	v := env("NODEDESK_EINK_DOCKER", "/var/run/docker.sock")
	if v == "off" {
		return ""
	}
	if _, err := os.Stat(v); err != nil {
		return ""
	}
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		slog.Warn("encode json", "err", err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

const previewHTML = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>NodeDesk E-Ink</title>
<style>
  body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #1b1c1f; }
  img { max-height: 96vh; max-width: 96vw; border-radius: 18px; box-shadow: 0 20px 60px #0008; }
</style>
<img id="d" src="/dashboard.png%[1]s" alt="NodeDesk E-Ink dashboard">
<script>
  const q = %[1]q;
  setInterval(() => { d.src = "/dashboard.png" + (q || "?") + (q ? "&" : "") + "t=" + Date.now(); }, 30000);
</script>
`
