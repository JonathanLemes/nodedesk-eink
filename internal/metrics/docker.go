package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// readDocker counts containers through the Engine API on a Unix socket.
// Plain net/http keeps the dependency list short; we only need one call.
func readDocker(ctx context.Context, socket string) (*Docker, error) {
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker: %s", resp.Status)
	}

	var containers []struct {
		Names  []string `json:"Names"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}
	d := &Docker{Total: len(containers)}
	for _, c := range containers {
		if c.State == "running" {
			d.Running++
		}
		if c.State == "restarting" || (c.State == "running" && strings.Contains(c.Status, "unhealthy")) {
			name := "?"
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			d.Unhealthy = append(d.Unhealthy, name)
		}
	}
	return d, nil
}
