FROM docker.io/library/golang:latest as testing

WORKDIR /code/
COPY go.* .
RUN go mod download

COPY . .
RUN gofmt -l .
RUN go test -cover ./...
