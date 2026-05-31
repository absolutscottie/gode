.PHONY: build clean run test

# Module path
MODULE := gode

# Build the cosmochat2 binary
build:
	go fmt ./...
	go build -o gode gode/cmd/gode

# Clean build artifacts
clean:
	rm -f gode

# Run the application
run: build
	./gode

# Run tests
test:
	go test -v ./...
