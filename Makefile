.PHONY: all build frontend backend clean dev

APP_NAME := agent-hive
BACKEND_DIR := backend
FRONTEND_DIR := frontend
EMBED_DIR := $(BACKEND_DIR)/internal/static/dist
OUTPUT_DIR := dist

all: build

# Build frontend, copy to embed dir, build Go binary
build: frontend backend

frontend:
	cd $(FRONTEND_DIR) && npm ci && npm run build
	rm -rf $(EMBED_DIR)
	cp -r $(FRONTEND_DIR)/dist $(EMBED_DIR)

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

backend:
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go build -ldflags "-X main.version=$(VERSION)" -o ../$(OUTPUT_DIR)/$(APP_NAME) ./cmd/server/

# Cross-platform builds
build-linux:
	cd $(BACKEND_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o ../$(OUTPUT_DIR)/$(APP_NAME)-linux-amd64 ./cmd/server/

build-darwin:
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o ../$(OUTPUT_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/server/
	cd $(BACKEND_DIR) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o ../$(OUTPUT_DIR)/$(APP_NAME)-darwin-arm64 ./cmd/server/

# Dev mode: run backend + frontend dev servers
dev:
	@echo "Starting dev servers..."
	@echo "Backend: http://localhost:8090"
	@echo "Frontend: http://localhost:5173"
	cd $(BACKEND_DIR) && CGO_ENABLED=1 go run ./cmd/server/ -dev &
	cd $(FRONTEND_DIR) && npm run dev

clean:
	rm -f $(OUTPUT_DIR)/$(APP_NAME) $(OUTPUT_DIR)/$(APP_NAME)-*
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	touch $(EMBED_DIR)/.gitkeep
