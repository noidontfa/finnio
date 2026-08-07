# Change "myapp" to your preferred binary name
BINARY_NAME=api
BUILD_DIR=bin

.PHONY: all build run clean test swagger

all: build

build: swagger sqlc-generate
	@echo "Building the Go binary..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/server/main.go

run: build 
	@echo "Running the application..."
	./$(BUILD_DIR)/$(BINARY_NAME)
	
build-dev: build 
	@echo "Building the Go binary..."
	
dev:
	@echo "Starting the development server..."
	go tool air
	
swagger:
	@echo "Generating swagger docs..."
	go tool swag init -g cmd/server/main.go -o gen/swagger --parseDependency --parseInternal

sqlc-generate:
	@echo "Generating SQLC code..."
	go tool sqlc generate

clean:
	@echo "Cleaning up build artifacts..."
	rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	go test ./...
	
setup:
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/air-verse/air@latest