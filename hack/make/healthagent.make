##@ Health Agent

HEALTH_AGENT_IMG ?= healthagent:$(TAG)
LOG_LEVEL ?= 3

.PHONY: run-health-agent
run-health-agent: manifests generate fmt vet install
	go run ./cmd/health/main.go \
	    --metrics-bind-address=:8080 \
	    --health-probe-bind-address=:8081 \
	    --zap-log-level=$(LOG_LEVEL)
