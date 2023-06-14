##@ Check
## Targets to be used to ensure code quality
.PHONY: verify-code
verify-code: vet ## Verify code formatting
	@diff -u <(echo -n) <(gofmt -d `find . -type f -name '*.go' -not -path "./vendor/*"`)

.PHONY: verify-manifests
verify-manifests: manifests ## Verify manifests update.
	git diff --exit-code ./config
	[ -z "$$(git ls-files --other --exclude-standard --directory --no-empty-directory ./config)" ]