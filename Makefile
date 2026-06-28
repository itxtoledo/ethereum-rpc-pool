.PHONY: build test test-unit test-integration lint docker-build docker-run clean

APP_NAME := ethereum-rpc-pool

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o app .

test: test-unit test-integration

test-unit:
	CGO_ENABLED=0 go test ./handlers/ ./utils/ -v -count=1

test-integration:
	CGO_ENABLED=0 go test ./... -v -count=1 -tags=integration -timeout 120s

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run -d -p 8080:8080 \
		-e RPC_LIST="http://host.docker.internal:8545" \
		$(APP_NAME):latest

dev:
	go run .

clean:
	rm -f app
