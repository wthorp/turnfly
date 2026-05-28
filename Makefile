# Build turnfly

APP_NAME  ?= turnfly
GO        ?= go
GOFMT     ?= gofmt
VET       ?= go vet

.PHONY: all build test vet fmt lint tidy clean

all: fmt vet test build

build:
	$(GO) build -o ./turnfly ./cmd/turnfly

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GOFMT) -w .
	@$(GOFMT) -l . | grep -q . && exit 1 || true

lint:
	golangci-lint run ./... || true

tidy:
	$(GO) mod tidy

clean:
	rm -f ./turnfly

check: fmt vet test
	@echo "All checks passed"

# Docker targets
docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run --rm \
		-p 3478:3478/udp \
		-p 8080:8080 \
		-p 9090:9090 \
		-e TURN_REALM=turnfly.local \
		-e TURN_SHARED_SECRET=dev-secret-change-me \
		-e ADMIN_API_TOKEN=dev-token-change-me \
		$(APP_NAME):latest serve-turn
