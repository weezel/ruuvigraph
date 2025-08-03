name		?= you-didnt-define-migration-name

GO		?= go
DOCKER		?= docker
DOCKER_BUILDKIT ?= 1
VERSION		?= $(shell git log --pretty=format:%h -n 1)
BUILD_TIME	?= $(shell date)
# -s removes symbol table and -ldflags -w debugging symbols
LDFLAGS		?= -trimpath -ldflags \
		   "-X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'"
GOARCH		?= amd64
GOOS		?= linux
# CGO_ENABLED=0 == static by default
CGO_ENABLED	?= 0


#all: test-unit lint build-all
all: lint build-all

_build: dist/$(APP_NAME)

build-proto:
	@buf generate

build-all:
	make -C cmd

.PHONY: cross-compile
cross-compile:
	@for target in "linux amd64" "linux arm" "openbsd amd64"; do \
		GOOS=$$(echo $$target | cut -d' ' -f1); \
		GOARCH=$$(echo $$target | cut -d' ' -f2); \
		echo "Building for $$GOOS/$$GOARCH..."; \
		$(MAKE) GOOS=$$GOOS GOARCH=$$GOARCH _build; \
	done

dist/$(APP_NAME):
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(LDFLAGS) \
		-o dist/$(APP_NAME)-$(GOOS)-$(GOARCH) \
		main.go

.PHONY: clean
clean:
	-rm -rf dist/

install-dependencies:
	@go get -d -v ./...

lint:
	@golangci-lint run ./...

vulncheck:
	@govulncheck ./...

escape-analysis:
	$(GO) build -gcflags="-m" 2>&1 ./...

test-coverage:
	go test -failfast -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-unit:
	go test -short -failfast -race ./...

