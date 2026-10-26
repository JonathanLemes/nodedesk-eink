package metrics

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// readGPU returns the first GPU found: NVIDIA through nvidia-smi (no cgo
// needed), then AMD through sysfs. It returns nil when there is none.
func readGPU(ctx context.Context) *GPU {
	if g := nvidiaGPU(ctx); g != nil {
		return g
	}
	return amdGPU()
}

func nvidiaGPU(ctx context.Context) *GPU {
	bin, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, bin,
		"--query-gpu=name,utilization.gpu,temperature.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	f := strings.Split(line, ",")
	if len(f) < 5 {
		return nil
	}
	const mib = 1 << 20
	return &GPU{
		Name:     strings.TrimSpace(f[0]),
		Percent:  atof(f[1]),
		Temp:     atof(f[2]),
		MemUsed:  uint64(atof(f[3]) * mib),
		MemTotal: uint64(atof(f[4]) * mib),
	}
}

func amdGPU() *GPU {
	cards, _ := filepath.Glob("/sys/class/drm/card[0-9]*/device/gpu_busy_percent")
	if len(cards) == 0 {
		return nil
	}
	dev := filepath.Dir(cards[0])
	g := &GPU{
		Name:     "AMD GPU",
		Percent:  readNumber(filepath.Join(dev, "gpu_busy_percent")),
		MemUsed:  uint64(readNumber(filepath.Join(dev, "mem_info_vram_used"))),
		MemTotal: uint64(readNumber(filepath.Join(dev, "mem_info_vram_total"))),
	}
	if hw, _ := filepath.Glob(filepath.Join(dev, "hwmon/hwmon*/temp1_input")); len(hw) > 0 {
		g.Temp = readNumber(hw[0]) / 1000
	}
	return g
}

func readNumber(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return atof(string(b))
}

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}
