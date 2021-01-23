DOCKER ?= podman

test:
	go mod tidy
	gofmt -l .
	go test -cover ./...

build: test
	$(DOCKER) build -t grest .
