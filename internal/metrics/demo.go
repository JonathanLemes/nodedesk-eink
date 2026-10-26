package metrics

import (
	"math"
	"time"
)

// Demo returns a plausible, deterministic snapshot. It feeds the README
// screenshot and lets you work on the layout without real hardware.
func Demo(now time.Time) Snapshot {
	const gb = 1_000_000_000
	interval := 10 * time.Second
	n := int(time.Hour / interval)
	h := History{Interval: interval, CPU: make([]float64, n), Memory: make([]float64, n), GPU: make([]float64, n)}
	for i := range n {
		t := float64(i) / float64(n)
		h.CPU[i] = 14 + 9*math.Sin(t*19) + 6*math.Sin(t*53) + 30*math.Exp(-math.Pow((t-0.62)*9, 2))
		h.Memory[i] = 41 + 3*t + 1.5*math.Sin(t*7)
		h.GPU[i] = 3 + 55*math.Exp(-math.Pow((t-0.3)*12, 2)) + 2*math.Sin(t*31)
	}
	return Snapshot{
		Time:     now,
		Hostname: "homelab",
		OS:       "Ubuntu 26.04",
		Uptime:   3*24*time.Hour + 4*time.Hour,
		Load:     [3]float64{1.63, 1.25, 1.16},
		CPU:      CPU{Percent: h.CPU[n-1], Temp: 52, Model: "Intel Core i9-10900K", Threads: 20},
		Memory:   Memory{Percent: h.Memory[n-1], Used: 14_300_000_000, Total: 33_400_000_000},
		GPU:      &GPU{Name: "NVIDIA GeForce RTX 3060", Percent: h.GPU[n-1], Temp: 41, MemUsed: 1_900_000_000, MemTotal: 12_900_000_000},
		Disks: []Disk{
			{Label: "", Mount: "/", FS: "ext4", Used: 288 * gb, Total: 500 * gb, Percent: 57.6},
			{Label: "Media", Mount: "/mnt/media", FS: "ext4", Used: 615 * gb, Total: 983 * gb, Percent: 62.6},
		},
		Net:     Net{RxRate: 1_840_000, TxRate: 96_000},
		Docker:  &Docker{Running: 14, Total: 17},
		History: h,
	}
}
