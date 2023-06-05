# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get

# Output binary name
BINARY_NAME = main

# Build flags
BUILD_FLAGS = -ldflags="-s -w" # Optional: Add any custom build flags here

all: clean build

build:
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) ./main.go

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

test:
	$(GOTEST) -v ./...

run: build
	./$(BINARY_NAME)

.PHONY: all build clean test run

