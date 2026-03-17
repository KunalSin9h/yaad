BINARY    := yaad
BUILD_DIR := bin

.PHONY: build install test clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/yaad

install:
	go install ./cmd/yaad

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
