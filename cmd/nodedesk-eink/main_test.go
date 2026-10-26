package main

import (
	"reflect"
	"testing"

	"github.com/JonathanLemes/nodedesk-eink/internal/metrics"
)

func TestParseDisks(t *testing.T) {
	got, err := parseDisks(" /=, /mnt/media=Media ,/srv/data ")
	if err != nil {
		t.Fatal(err)
	}
	want := []metrics.DiskSpec{{Mount: "/"}, {Mount: "/mnt/media", Label: "Media"}, {Mount: "/srv/data", Label: "data"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDisks = %+v, want %+v", got, want)
	}
	if _, err := parseDisks("media=Media"); err == nil {
		t.Error("relative mount accepted")
	}
}
