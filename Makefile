.PHONY: all build web test dist publish clean help

VERSION ?= $(shell cat VERSION 2>/dev/null | tr -d ' \r\n')
# Support 'make publish 0.2.0' where target argument is passed as next goal
VERSION_ARG := $(filter-out publish,$(MAKECMDGOALS))
ifeq ($(strip $(VERSION_ARG)),)
  TARGET_VERSION := $(VERSION)
else
  TARGET_VERSION := $(strip $(VERSION_ARG))
  # Treat extra goal as no-op so make doesn't attempt to build it
  %:
	@:
endif

# Clean leading 'v' from version string if present
CLEAN_VERSION := $(patsubst v%,%,$(TARGET_VERSION))

LDFLAGS := -s -w

all: build

help:
	@echo "hos-code make targets:"
	@echo "  make build             - build the hos-code binary for this platform"
	@echo "  make web               - bundle web assets (JS/CSS/themes)"
	@echo "  make test              - run go test suite"
	@echo "  make test-short        - run fast unit tests (short mode)"
	@echo "  make check             - bundle web assets and run fast test suite"
	@echo "  make dist              - build release archives (binary + licences) into dist/"
	@echo "  make publish <version> - bump VERSION, commit, tag, and build dist archives"
	@echo "  make clean             - remove build artifacts"

web:
	@echo "Bundling web assets..."
	@node ./scripts/build-web.js

build: web
	@echo "Building hos-code for local system..."
	go build -trimpath -ldflags="$(LDFLAGS)" -o hos-code .
	@ln -sf hos-code px0 # until the brain's land script installs hos-code by name
	@echo "Built ./hos-code ($$(du -h hos-code | cut -f1))"

dist:
	@./build.sh

test: web
	go test -v .

web-check:
	@node ./scripts/check-web.js

test-short:
	go test -short .

check: web-check web test-short

bench:
	go test -bench=. -benchmem -run=^$$ .

benchmark: build
	./benchmark.sh

publish:
	@if [ -z "$(CLEAN_VERSION)" ]; then \
		echo "Error: Version cannot be empty. Usage: make publish <version> (e.g. make publish 0.2.0)"; \
		exit 1; \
	fi
	@echo "==> Preparing release v$(CLEAN_VERSION) (previous: $$(cat VERSION))"
	@echo "$(CLEAN_VERSION)" > VERSION
	@echo "==> Bundling web assets and building dist binaries..."
	@./build.sh
	@echo "==> Updating git repository..."
	@git add VERSION
	@git commit -m "Release v$(CLEAN_VERSION)" || true
	@git tag -fa "v$(CLEAN_VERSION)" -m "Release v$(CLEAN_VERSION)"
	@echo ""
	@echo "✓ Successfully prepared release v$(CLEAN_VERSION) and updated dist/!"
	@echo "Tag v$(CLEAN_VERSION) has been created."
	@echo ""
	@echo "The Release build workflow (run by hand) rebuilds these archives; nothing is published automatically."

clean:
	rm -f hos-code px0 web/app.js
	touch web/app.js
	rm -rf dist/
