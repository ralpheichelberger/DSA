test:
	go test ./... -v -count=1

test-integration:
	go test ./... -v -count=1 -tags=integration

lint:
	go vet ./...

build:
	go build -o bin/agent ./cmd/agent
	cd frontend && npm run build

dev-backend:
	DEV_MODE=true go run ./cmd/agent

dev-frontend:
	cd frontend && npm run dev

dev:
	make dev-backend & make dev-frontend

create-data-dir:
	mkdir -p data

smoke-test:
	DEV_MODE=true go run ./cmd/smoke-test

full-test:
	$(MAKE) test && $(MAKE) smoke-test

.PHONY: test test-integration lint build dev dev-backend dev-frontend create-data-dir smoke-test full-test
