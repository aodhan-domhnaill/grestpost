DOCKER = podman

all: build

build:
	${DOCKER} build -t grest .
