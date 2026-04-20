# NPC AI Admin — backend verification targets
#
# Quick start:
#   make tools       # one-time: install golangci-lint + govulncheck
#   make verify      # run lint + vuln + test + fmt-check
#
# Individual targets:
#   make lint        # golangci-lint
#   make vuln        # govulncheck (dependency CVE scan)
#   make test        # go test with coverage
#   make fmt-check   # gofmt -l (fail if any unformatted file)
#   make cover       # print coverage summary
#
# Not yet wired (needs OpenAPI doc first):
#   spectral lint docs/openapi.yaml   # API contract lint

BACKEND_DIR := backend
COVER_FILE  := $(BACKEND_DIR)/coverage.out

# Resolve GOPATH/bin so `go install`ed tools work even when the user's shell PATH
# doesn't include it (common on Windows + git bash).
GOBIN_DIR       := $(shell go env GOPATH)/bin
GOLANGCI_LINT   := $(GOBIN_DIR)/golangci-lint
GOVULNCHECK     := $(GOBIN_DIR)/govulncheck

.PHONY: tools lint vuln test fmt-check cover verify clean-cover

tools:
	@echo ">> installing golangci-lint + govulncheck"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	go install golang.org/x/vuln/cmd/govulncheck@latest

lint:
	@echo ">> golangci-lint"
	cd $(BACKEND_DIR) && "$(GOLANGCI_LINT)" run ./...

vuln:
	@echo ">> govulncheck"
	cd $(BACKEND_DIR) && "$(GOVULNCHECK)" ./...

test:
	@echo ">> go test -cover"
	cd $(BACKEND_DIR) && go test -coverprofile=coverage.out ./...

# -race requires cgo (gcc/mingw on Windows). Kept separate so default test target
# stays portable. Run on Linux/CI or on Windows after installing a C toolchain.
test-race:
	@echo ">> go test -race -cover"
	cd $(BACKEND_DIR) && go test -race -coverprofile=coverage.out ./...

# Note: on Windows with core.autocrlf=true, working-tree files are CRLF while
# git stores LF, so `gofmt -l` flags every file. golangci-lint's gofmt linter
# (enabled in .golangci.yml) handles this correctly, so fmt-check is excluded
# from `verify` and kept only as an explicit target for CI/Linux use.
fmt-check:
	@echo ">> gofmt -l (run on Linux/CI; on Windows use 'make lint' instead)"
	@cd $(BACKEND_DIR) && out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "unformatted files:"; echo "$$out"; exit 1; fi

cover:
	@cd $(BACKEND_DIR) && go tool cover -func=coverage.out | tail -n 1

verify: lint vuln test cover
	@echo ">> verify: all checks passed"

clean-cover:
	rm -f $(COVER_FILE)
