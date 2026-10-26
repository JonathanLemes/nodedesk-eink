VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
KINDLE  ?= kindle

.PHONY: build test demo install kindle-install clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/nodedesk-eink ./cmd/nodedesk-eink

test:
	go vet ./...
	go test ./...

# Regenerate the README screenshots from demo data.
demo: build
	./bin/nodedesk-eink render --demo --battery 82 -o examples/dashboard.png
	go run ./tools/mockup -in examples/dashboard.png -out examples/kindle.png

# Install the server for the current user (systemd --user, no root).
install: build
	install -Dm755 bin/nodedesk-eink $(HOME)/.local/bin/nodedesk-eink
	install -Dm644 deploy/nodedesk-eink.service $(HOME)/.config/systemd/user/nodedesk-eink.service
	test -f $(HOME)/.config/nodedesk-eink/env || install -Dm600 deploy/env.example $(HOME)/.config/nodedesk-eink/env
	systemctl --user daemon-reload
	systemctl --user enable --now nodedesk-eink
	systemctl --user restart nodedesk-eink

# Copy the Kindle client over SSH (KINDLE=user@host). Keeps an existing config.sh.
kindle-install:
	ssh $(KINDLE) 'mkdir -p /mnt/us/nodedesk-eink'
	scp kindle/nodedesk-eink/dashboard.sh $(KINDLE):/mnt/us/nodedesk-eink/
	ssh $(KINDLE) 'test -f /mnt/us/nodedesk-eink/config.sh' || scp kindle/nodedesk-eink/config.sh $(KINDLE):/mnt/us/nodedesk-eink/
	scp "kindle/NodeDesk E-Ink.sh" "$(KINDLE):/mnt/us/documents/NodeDesk E-Ink.sh"

clean:
	rm -rf bin
