package render

import (
	"testing"
	"time"

	"github.com/JonathanLemes/nodedesk-eink/internal/metrics"
)

func TestRenderSizes(t *testing.T) {
	snap := metrics.Demo(time.Date(2026, 10, 26, 9, 41, 0, 0, time.UTC))
	for _, size := range [][2]int{{1236, 1648}, {1072, 1448}, {758, 1024}, {600, 800}} {
		img := Render(snap, Options{Width: size[0], Height: size[1], Lang: "pt", Battery: 50})
		if b := img.Bounds(); b.Dx() != size[0] || b.Dy() != size[1] {
			t.Errorf("Render(%v) = %v", size, b)
		}
	}
}

// A machine without GPU, Docker, disks or history must still render.
func TestRenderEmptySnapshot(t *testing.T) {
	Render(metrics.Snapshot{Time: time.Now()}, Options{Battery: -1})
}

func TestFormatting(t *testing.T) {
	pt, en := LangFor("pt-BR"), LangFor("en_US")
	cases := []struct{ got, want string }{
		{pt.bytes(14_300_000_000), "14,3 GB"},
		{en.bytes(14_300_000_000), "14.3 GB"},
		{en.bytes(983_000_000_000), "983 GB"},
		{en.bytes(512), "512 B"},
		{pt.rate(96_000), "96,0 KB/s"},
		{en.rate(0), "0 B/s"},
		{pt.date(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)), "Segunda, 26 out"},
		{en.date(time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)), "Monday, Oct 26"},
		{duration(76 * time.Hour), "3d 4h"},
		{duration(90 * time.Minute), "1h 30min"},
		{LangFor("xx").System, "System"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestAlerts(t *testing.T) {
	l := LangFor("en")
	s := metrics.Snapshot{
		CPU:    metrics.CPU{Temp: 91},
		Disks:  []metrics.Disk{{Label: "", Percent: 95}, {Label: "media", Percent: 40}},
		Docker: &metrics.Docker{Unhealthy: []string{"jellyfin"}},
	}
	got := alerts(l, s)
	want := []string{"CPU is warm (91 °C)", "System is 95% full", "jellyfin is struggling"}
	if len(got) != len(want) {
		t.Fatalf("alerts = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("alerts[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
