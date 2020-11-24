FROM docker.io/library/golang:latest

WORKDIR /code/
COPY go.* .
RUN go mod download

COPY . .
RUN go test ./...
