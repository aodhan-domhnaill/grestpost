DOCKER ?= podman

POSTGRES_DB = "postgres"
POSTGRES_PASSWORD = "postgres"
POSTGRES_USER = "postgres"

POSTGRES_CONTAINER_NAME = grest-test-postgres

all: build test

build:
	go mod tidy
	$(DOCKER) build -t grest .
	$(DOCKER) build -t grest_test --target testing .

launch-postgres:
	$(DOCKER) build -t grest_testdb --target testdb .
	$(DOCKER) network create $(POSTGRES_CONTAINER_NAME) || $(MAKE) failclean
	$(DOCKER) run -d --rm \
		--name $(POSTGRES_CONTAINER_NAME) \
		--network $(POSTGRES_CONTAINER_NAME) \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-e POSTGRES_USER=$(POSTGRES_USER) \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		grest_testdb|| $(MAKE) failclean

check-postgres: launch-postgres
	$(DOCKER) run -d --rm \
		--name grest-check-postgres \
		--network $(POSTGRES_CONTAINER_NAME) \
		busybox sh -c 'until nc -z $(POSTGRES_CONTAINER_NAME) 5432; do sleep 1; done' \
		|| $(MAKE) failclean

test: build launch-postgres check-postgres
	$(DOCKER) run --rm \
		--network $(POSTGRES_CONTAINER_NAME) \
		-e GREST_INTEG_TEST=true \
		-e GREST_AUTHENTICATION=basic \
		-e GREST_USER_TABLE=users \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-e POSTGRES_USER=$(POSTGRES_USER) \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		grest_test || $(MAKE) failclean
	$(MAKE) clean

clean:
	$(DOCKER) stop grest-check-postgres || echo Stopped
	$(DOCKER) rm -f grest-check-postgres || echo Removed
	$(DOCKER) stop $(POSTGRES_CONTAINER_NAME) || echo Stopped
	$(DOCKER) rm -f $(POSTGRES_CONTAINER_NAME) || echo Removed
	$(DOCKER) network rm $(POSTGRES_CONTAINER_NAME) || echo Removed
failclean: clean
	exit 1
