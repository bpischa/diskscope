.PHONY: build build-windows build-linux build-macos-arm run clean test

BINARY=diskscope

build:
	go build -o $(BINARY) ./cmd/diskscope.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY).exe ./cmd/diskscope.go

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux ./cmd/diskscope.go

build-macos-arm:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-arm64 ./cmd/diskscope.go

run: build
	./$(BINARY)

test:
	go test -v ./...

clean:
	rm -rf $(BINARY) $(BINARY).exe $(BINARY)-linux $(BINARY)-arm64
