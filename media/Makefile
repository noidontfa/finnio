# Change "myapp" to your preferred binary name
BINARY_NAME=media
BUILD_DIR=bin

.PHONY: all build run clean test hello segment segment-rtmp segment-rtmp-test abr abr-consumer play

all: build

build: 
	@echo "Building the Go binary..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) cmd/cli/main.go

run: build 
	@echo "Running the application..."
	./$(BUILD_DIR)/$(BINARY_NAME) 
	
build-dev: build 
	@echo "Building the Go binary..."
	
dev:
	@echo "Starting the development server..."
	go tool air
	
clean:
	@echo "Cleaning up build artifacts..."
	rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	go test ./...

# Commands

hello: build
	@echo "Running hello command..."
	@./$(BUILD_DIR)/$(BINARY_NAME) hello -n Thinh
	
segment: build
	@echo "Running segment command..."
	@./$(BUILD_DIR)/$(BINARY_NAME) segment -i ../tmp/sample/file_example_MP4_1920_18MG.mp4 -o ../deployments/tmp/segment/9e325fda-f425-47f8-afa1-6e80a8992e4d -t 2 -s 9e325fda-f425-47f8-afa1-6e80a8992e4d

segment-rtmp: build
	@echo "Running segment-rtmp command..."
	@./$(BUILD_DIR)/$(BINARY_NAME) segment-rtmp -i rtmp://127.0.0.1:1935/$(KEY) -o ../deployments/tmp/segment-rtmp/$(KEY) -t 2 -s $(KEY)

# Local RTMP smoke test: MP4 → FFmpeg publish → MediaMTX → segment-rtmp
# Requires: docker, ffmpeg. Uses a throwaway MediaMTX on :1936 (no auth / on_ready).
#
#   make segment-rtmp-test
#   make segment-rtmp-test DURATION=20 SAMPLE_MP4=/path/to/file.mp4
SAMPLE_MP4 ?= ../tmp/sample/file_example_MP4_1920_18MG.mp4
SEGMENT_RTMP_PORT ?= 1936
SEGMENT_RTMP_PATH ?= segment-test
SEGMENT_RTMP_URL := rtmp://127.0.0.1:$(SEGMENT_RTMP_PORT)/$(SEGMENT_RTMP_PATH)
SEGMENT_RTMP_OUT ?= ../deployments/tmp/segment-rtmp/$(SEGMENT_RTMP_PATH)
SEGMENT_RTMP_SOURCE_ID ?= $(SEGMENT_RTMP_PATH)
MEDIAMTX_TEST_CTR ?= mediamtx-segment-test
DURATION ?= 10

segment-rtmp-test: build
	@command -v docker >/dev/null || (echo "docker is required"; exit 1)
	@command -v ffmpeg >/dev/null || (echo "ffmpeg is required"; exit 1)
	@test -f "$(SAMPLE_MP4)" || (echo "missing sample: $(SAMPLE_MP4)"; exit 1)
	@mkdir -p "$(SEGMENT_RTMP_OUT)"
	@rm -rf "$(SEGMENT_RTMP_OUT)"/*
	@docker rm -f $(MEDIAMTX_TEST_CTR) >/dev/null 2>&1 || true
	@echo "Starting MediaMTX on :$(SEGMENT_RTMP_PORT)..."
	@docker run -d --name $(MEDIAMTX_TEST_CTR) -p $(SEGMENT_RTMP_PORT):1935 bluenviron/mediamtx:latest >/dev/null
	@sleep 1
	@echo "Publishing $(DURATION)s of $(SAMPLE_MP4) → $(SEGMENT_RTMP_URL)"
	@ffmpeg -hide_banner -loglevel error -re -t $(DURATION) -i "$(SAMPLE_MP4)" \
		-c copy -f flv "$(SEGMENT_RTMP_URL)" & \
		echo $$! > "$(SEGMENT_RTMP_OUT)/.publish.pid"
	@echo "Running segment-rtmp..."
	@set +e; \
		./$(BUILD_DIR)/$(BINARY_NAME) segment-rtmp \
			-i "$(SEGMENT_RTMP_URL)" \
			-o "$(SEGMENT_RTMP_OUT)" \
			-s "$(SEGMENT_RTMP_SOURCE_ID)" \
			-t 2 \
			--wait 15; \
		status=$$?; \
		if [ -f "$(SEGMENT_RTMP_OUT)/.publish.pid" ]; then \
			kill $$(cat "$(SEGMENT_RTMP_OUT)/.publish.pid") 2>/dev/null || true; \
			rm -f "$(SEGMENT_RTMP_OUT)/.publish.pid"; \
		fi; \
		docker rm -f $(MEDIAMTX_TEST_CTR) >/dev/null 2>&1 || true; \
		echo "--------------------------------"; \
		echo "Output:"; \
		ls -la "$(SEGMENT_RTMP_OUT)"; \
		exit $$status

CONSUMERS ?= 2
ARB_IN ?= ../deployments/tmp/segment-rtmp/segment-test/seg_00003.ts
ARB_OUT ?= ../deployments/tmp/abr-test

abr: build
	@echo "Running arb command..."
	@./$(BUILD_DIR)/$(BINARY_NAME) abr -c $(CONSUMERS) -i $(ARB_IN) -o $(ARB_OUT)
	
abr-consumer: build
	@echo "Running abr-consumer command..."
	@./$(BUILD_DIR)/$(BINARY_NAME) abr-consumer -c $(CONSUMERS) -o $(ARB_OUT)

# Local HLS player for an ABR folder that already has master.m3u8
# (browsers block HLS from file:// — this serves player + playlists over http).
#
#   make play
#   make play PLAY_FOLDER=../deployments/tmp/hls/abr/<key> PLAY_PORT=8081
PLAY_FOLDER ?= ../deployments/tmp/abr-test/segment-test
PLAY_PORT ?= 8080

play:
	@test -f "$(PLAY_FOLDER)/master.m3u8" || (echo "missing master.m3u8 in $(PLAY_FOLDER)"; exit 1)
	@echo "Playing $(PLAY_FOLDER) at http://127.0.0.1:$(PLAY_PORT)/"
	@python3 ../tmp/play.py "$(PLAY_FOLDER)" -p $(PLAY_PORT)