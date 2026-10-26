package metrics

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Options configures a Collector. Zero values pick sensible defaults.
type Options struct {
	// Interval between samples (default 10s).
	Interval time.Duration
	// HistoryLen is how many samples are kept for the charts (default 1 h worth).
	HistoryLen int
	// Disks overrides automatic disk discovery: mount point -> label.
	Disks []DiskSpec
	// DockerSocket is the Docker Engine socket; empty disables Docker.
	DockerSocket string
	// Name replaces the hostname on the dashboard.
	Name string
}

// DiskSpec pins a mount point to a display label.
type DiskSpec struct {
	Mount string
	Label string
}

// Collector samples the host every Interval and keeps the latest snapshot.
type Collector struct {
	opts Options

	mu   sync.RWMutex
	last Snapshot
	hist History

	prevNet  net.IOCountersStat
	prevTime time.Time
}

func NewCollector(opts Options) *Collector {
	if opts.Interval <= 0 {
		opts.Interval = 10 * time.Second
	}
	if opts.HistoryLen <= 0 {
		opts.HistoryLen = int(time.Hour / opts.Interval)
	}
	return &Collector{opts: opts, hist: History{Interval: opts.Interval}}
}

// Run samples until ctx is cancelled. The first sample is taken immediately.
func (c *Collector) Run(ctx context.Context) {
	// cpu.Percent(0) compares against the previous call, so prime it once.
	_, _ = cpu.PercentWithContext(ctx, 0, false)
	c.sample(ctx)
	t := time.NewTicker(c.opts.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.sample(ctx)
		}
	}
}

// Snapshot returns a copy of the latest sample.
func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s := c.last
	s.Disks = append([]Disk(nil), s.Disks...)
	s.History = History{
		Interval: c.hist.Interval,
		CPU:      append([]float64(nil), c.hist.CPU...),
		Memory:   append([]float64(nil), c.hist.Memory...),
		GPU:      append([]float64(nil), c.hist.GPU...),
	}
	return s
}

func (c *Collector) sample(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.Interval)
	defer cancel()

	now := time.Now()
	s := Snapshot{Time: now}
	s.Hostname = c.opts.Name
	if s.Hostname == "" {
		s.Hostname, _ = os.Hostname()
	}

	if info, err := host.InfoWithContext(ctx); err == nil {
		s.OS = strings.TrimSpace(titleCase(info.Platform) + " " + info.PlatformVersion)
		s.Uptime = time.Duration(info.Uptime) * time.Second
	}
	if l, err := load.AvgWithContext(ctx); err == nil {
		s.Load = [3]float64{l.Load1, l.Load5, l.Load15}
	}

	if p, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(p) > 0 {
		s.CPU.Percent = p[0]
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		s.CPU.Model = strings.TrimSpace(infos[0].ModelName)
	}
	s.CPU.Threads, _ = cpu.CountsWithContext(ctx, true)
	s.CPU.Temp = cpuTemp(ctx)

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		s.Memory = Memory{Percent: vm.UsedPercent, Used: vm.Used, Total: vm.Total}
	}

	s.GPU = readGPU(ctx)
	s.Disks = readDisks(ctx, c.opts.Disks)
	s.Net = c.netRates(ctx, now)
	if c.opts.DockerSocket != "" {
		d, err := readDocker(ctx, c.opts.DockerSocket)
		if err != nil {
			slog.Debug("docker unavailable", "err", err)
		}
		s.Docker = d
	}

	c.mu.Lock()
	c.last = s
	c.hist.CPU = push(c.hist.CPU, s.CPU.Percent, c.opts.HistoryLen)
	c.hist.Memory = push(c.hist.Memory, s.Memory.Percent, c.opts.HistoryLen)
	if s.GPU != nil {
		c.hist.GPU = push(c.hist.GPU, s.GPU.Percent, c.opts.HistoryLen)
	}
	c.mu.Unlock()
}

func (c *Collector) netRates(ctx context.Context, now time.Time) Net {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return Net{}
	}
	var sum net.IOCountersStat
	for _, n := range counters {
		if isVirtualIface(n.Name) {
			continue
		}
		sum.BytesRecv += n.BytesRecv
		sum.BytesSent += n.BytesSent
	}
	var r Net
	if !c.prevTime.IsZero() && sum.BytesRecv >= c.prevNet.BytesRecv && sum.BytesSent >= c.prevNet.BytesSent {
		dt := now.Sub(c.prevTime).Seconds()
		r.RxRate = float64(sum.BytesRecv-c.prevNet.BytesRecv) / dt
		r.TxRate = float64(sum.BytesSent-c.prevNet.BytesSent) / dt
	}
	c.prevNet, c.prevTime = sum, now
	return r
}

func isVirtualIface(name string) bool {
	for _, p := range []string{"lo", "docker", "br-", "veth", "virbr", "tailscale", "tun", "wg"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// cpuTemp prefers the package sensor and falls back to the hottest core.
func cpuTemp(ctx context.Context) float64 {
	temps, _ := sensors.TemperaturesWithContext(ctx) // partial results come with warnings
	preferred := []string{"coretemp_package_id_0", "k10temp_tctl", "zenpower_tctl", "cpu_thermal"}
	for _, key := range preferred {
		for _, t := range temps {
			if t.SensorKey == key || strings.HasPrefix(t.SensorKey, key) {
				return t.Temperature
			}
		}
	}
	var hottest float64
	for _, t := range temps {
		if strings.HasPrefix(t.SensorKey, "coretemp") || strings.HasPrefix(t.SensorKey, "k10temp") {
			hottest = max(hottest, t.Temperature)
		}
	}
	return hottest
}

func push(xs []float64, v float64, limit int) []float64 {
	xs = append(xs, v)
	if len(xs) > limit {
		xs = xs[len(xs)-limit:]
	}
	return xs
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
