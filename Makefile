.DEFAULT_GOAL := help
.PHONY: help build release-dry release

# Local secrets, gitignored. See .env.example. This is the source of truth for
# GITEA_TOKEN and deliberately overrides any value in the ambient environment.
-include .env
export GITEA_TOKEN

help:
	@echo "targets:"
	@echo "  build                 build ./lantern for this machine"
	@echo "  release-dry           build all release archives into dist/, publish nothing"
	@echo "  release TAG=vX.Y.Z    tag, push, and publish the release to Codeberg"

build:
	go build -o lantern .

release-dry:
	goreleaser release --snapshot --clean --skip=publish

release:
	@if [ -z "$(TAG)" ]; then echo "error: TAG is required, e.g. make release TAG=v0.3.0" >&2; exit 1; fi
	@case "$(TAG)" in v*) ;; *) echo "error: TAG must start with 'v' (got '$(TAG)')" >&2; exit 1 ;; esac
	@if [ -z "$$GITEA_TOKEN" ]; then echo "error: GITEA_TOKEN is not set (Codeberg token with write:repository)" >&2; exit 1; fi
	@if [ -n "$$(git status --porcelain)" ]; then echo "error: working tree is dirty" >&2; exit 1; fi
	go test ./...
	goreleaser check
	git tag -a $(TAG) -m $(TAG)
	git push origin $(TAG)
	goreleaser release --clean
