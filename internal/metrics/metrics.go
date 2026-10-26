// Package metrics samples the host (CPU, memory, GPU, disks, network, Docker)
// in the background and hands out immutable snapshots for rendering.
package metrics

import "time"

// Snapshot is everything the dashboard shows, captured at Time.
// Temperatures are in °C and are 0 when the sensor is not available.
type Snapshot struct {
	Time     time.Time     `json:"time"`
	Hostname string        `json:"hostname"`
	OS       string        `json:"os"`
	Uptime   time.Duration `json:"uptime"`
	Load     [3]float64    `json:"load"`

	CPU    CPU     `json:"cpu"`
	Memory Memory  `json:"memory"`
	GPU    *GPU    `json:"gpu,omitempty"`
	Disks  []Disk  `json:"disks"`
	Net    Net     `json:"net"`
	Docker *Docker `json:"docker,omitempty"`

	History History `json:"history"`
}

type CPU struct {
	Percent float64 `json:"percent"`
	Temp    float64 `json:"temp"`
	Model   string  `json:"model"`
	Threads int     `json:"threads"`
}

type Memory struct {
	Percent float64 `json:"percent"`
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
}

type GPU struct {
	Name     string  `json:"name"`
	Percent  float64 `json:"percent"`
	Temp     float64 `json:"temp"`
	MemUsed  uint64  `json:"memUsed"`
	MemTotal uint64  `json:"memTotal"`
}

type Disk struct {
	Label   string  `json:"label"`
	Mount   string  `json:"mount"`
	FS      string  `json:"fs"`
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

// Net rates are in bytes per second, summed over physical interfaces.
type Net struct {
	RxRate float64 `json:"rxRate"`
	TxRate float64 `json:"txRate"`
}

type Docker struct {
	Running int `json:"running"`
	Total   int `json:"total"`
	// Unhealthy lists containers that are restarting or failing their
	// health check.
	Unhealthy []string `json:"unhealthy,omitempty"`
}

// History holds one point per sample interval, oldest first, in percent.
type History struct {
	Interval time.Duration `json:"interval"`
	CPU      []float64     `json:"cpu"`
	Memory   []float64     `json:"memory"`
	GPU      []float64     `json:"gpu,omitempty"`
}
