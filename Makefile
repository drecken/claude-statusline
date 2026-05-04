INSTALL_DIR ?= $(HOME)/.claude/bin
LDFLAGS := -ldflags=-s -w

.PHONY: build install clean

build:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin/statusline ./cmd/statusline
	go build -ldflags="-s -w" -o bin/subagent-statusline ./cmd/subagent-statusline

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 0755 bin/statusline $(INSTALL_DIR)/statusline
	install -m 0755 bin/subagent-statusline $(INSTALL_DIR)/subagent-statusline

clean:
	rm -rf bin
