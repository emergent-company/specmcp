.PHONY: build run test seed clean fmt vet tidy install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BINARY  := specmcp
OUTDIR  := dist

build:
	@mkdir -p $(OUTDIR)
	@echo "Building $(BINARY) $(VERSION)..."
	go build -ldflags "-X main.Version=$(VERSION)" -o $(OUTDIR)/$(BINARY) ./cmd/specmcp/

run: build
	$(OUTDIR)/$(BINARY)

install:
	go install -ldflags "-X main.Version=$(VERSION)" ./cmd/specmcp/

test:
	go test ./...

seed:
	@echo "Seeding template pack..."
	go run ./scripts/seed.go

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf $(OUTDIR)

tidy:
	go mod tidy
