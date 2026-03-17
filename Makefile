BINARY    := lore
BUILD_DIR := bin

.PHONY: build install test clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/lore

install:
	go install ./cmd/lore

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
