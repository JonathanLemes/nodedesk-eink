package metrics

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// Mounts that are never interesting on a dashboard.
var skipMounts = []string{"/boot", "/snap", "/run", "/var/lib/docker", "/var/snap"}

const minDiskSize = 4 << 30 // ignore tiny partitions (EFI, recovery...)

// readDisks reports the pinned disks, or every real filesystem when none is
// pinned. Bind mounts of the same device are listed once.
func readDisks(ctx context.Context, pinned []DiskSpec) []Disk {
	if len(pinned) > 0 {
		var out []Disk
		for _, p := range pinned {
			if d, ok := usage(ctx, p.Mount, ""); ok {
				d.Label = p.Label
				out = append(out, d)
			}
		}
		return out
	}

	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Disk
	for _, p := range parts {
		if seen[p.Device] || skipped(p.Mountpoint) || p.Fstype == "squashfs" {
			continue
		}
		d, ok := usage(ctx, p.Mountpoint, p.Fstype)
		if !ok || d.Total < minDiskSize {
			continue
		}
		seen[p.Device] = true
		d.Label = defaultLabel(p.Mountpoint)
		out = append(out, d)
	}
	return out
}

func usage(ctx context.Context, mount, fs string) (Disk, bool) {
	u, err := disk.UsageWithContext(ctx, mount)
	if err != nil || u.Total == 0 {
		return Disk{}, false
	}
	if fs == "" {
		fs = u.Fstype
	}
	return Disk{Mount: mount, FS: fs, Used: u.Used, Total: u.Total, Percent: u.UsedPercent}, true
}

func skipped(mount string) bool {
	for _, s := range skipMounts {
		if mount == s || strings.HasPrefix(mount, s+"/") {
			return true
		}
	}
	return false
}

// defaultLabel is "" for "/" (the renderer names it "System") and the last
// path element otherwise ("/mnt/wd1tb" -> "wd1tb").
func defaultLabel(mount string) string {
	if mount == "/" {
		return ""
	}
	return filepath.Base(mount)
}
