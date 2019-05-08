BINARY_NAME=sfn-status-notifier

all: build
run:
	CONFIG_PATH=test.yml go run *.go
build:
	go build -o bin/$(BINARY_NAME) -v
test:
	go test -v ./...