FROM docker.io/library/postgres:latest as testdb

RUN apt-get update \
    && apt-get install -y postgresql-contrib

COPY ./init-test-db.sh /docker-entrypoint-initdb.d/


FROM docker.io/library/golang:latest as testing

WORKDIR /code/
COPY go.* .
RUN go mod download

COPY . .
RUN gofmt -l .

RUN go test -cover ./...
