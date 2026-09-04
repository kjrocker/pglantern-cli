.DEFAULT_GOAL := help
.PHONY: help build release-dry release

# Local secrets, gitignored. See .env.example. This is the source of truth for
# GITHUB_TOKEN and deliberately overrides any value in the ambient environment.
-include .env
export GITHUB_TOKEN

help:
	@echo "targets:"
	@echo "  build                 build ./lantern for this machine"
	@echo "  release-dry           build all release archives into dist/, publish nothing"
	@echo "  release TAG=vX.Y.Z    tag, push, and publish the release to GitHub"

build:
	go build -o lantern .

release-dry:
	goreleaser release --snapshot --clean --skip=publish

release:
	@if [ -z "$(TAG)" ]; then echo "error: TAG is required, e.g. make release TAG=v0.3.0" >&2; exit 1; fi
	@case "$(TAG)" in v*) ;; *) echo "error: TAG must start with 'v' (got '$(TAG)')" >&2; exit 1 ;; esac
	@if [ -z "$$GITHUB_TOKEN" ]; then echo "error: GITHUB_TOKEN is not set (GitHub PAT with contents:write on kjrocker/pglantern-cli)" >&2; exit 1; fi
	@if [ -n "$$(git status --porcelain)" ]; then echo "error: working tree is dirty" >&2; exit 1; fi
	go test ./...
	goreleaser check
	@# Retry-safe: reuse the tag if it already points at HEAD, refuse if it
	@# points somewhere else (moving a published tag breaks anyone who has it).
	@if git rev-parse -q --verify refs/tags/$(TAG) >/dev/null; then \
		if [ "$$(git rev-parse refs/tags/$(TAG)^{commit})" != "$$(git rev-parse HEAD)" ]; then \
			echo "error: tag $(TAG) exists but does not point at HEAD" >&2; exit 1; \
		fi; \
		echo "tag $(TAG) already exists at HEAD, reusing it"; \
	else \
		git tag -a $(TAG) -m $(TAG); \
	fi
	git push origin $(TAG)
	@# origin mirrors to GitHub asynchronously; push the tag there directly so
	@# goreleaser never races ahead of the mirror. No-op once the mirror lands.
	git push github $(TAG)
	goreleaser release --clean
