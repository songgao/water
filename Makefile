.PHONY: default ci test vet gofmt build clean

default:
	@echo 'This makefile is for CI. Targets: ci, build, test, vet, gofmt.'
	@exit 1

ci: gofmt vet build test-waterutil test

# Creating a TUN/TAP device needs CAP_NET_ADMIN, so build as the normal user and
# run the tests as root.
test: water.test
	sudo ./water.test -test.v

# waterutil is pure packet parsing: no device, no privilege.
test-waterutil:
	go test -race ./waterutil/...

# Cross-compiling every supported target is the cheapest way to keep the
# platform-specific files honest.
build:
	GOOS=linux   GOARCH=amd64 go build ./...
	GOOS=linux   GOARCH=arm64 go build ./...
	GOOS=darwin  GOARCH=amd64 go build ./...
	GOOS=darwin  GOARCH=arm64 go build ./...
	GOOS=windows GOARCH=amd64 go build ./...
	GOOS=windows GOARCH=386   go build ./...
	GOOS=windows GOARCH=arm64 go build ./...

# -unsafeptr=false: water converts the address Wintun hands back for a packet in
# its shared ring buffers into a slice. That memory belongs to the driver rather
# than to the Go heap, so it never moves and the collector has no stake in it,
# but vet cannot tell it apart from a stale Go pointer. See ringMemory in
# wintun_windows.go.
vet:
	GOOS=linux   go vet -unsafeptr=false ./...
	GOOS=darwin  go vet -unsafeptr=false ./...
	GOOS=windows go vet -unsafeptr=false ./...

gofmt:
	@unformatted=$$(gofmt -s -l .); \
	if [ -n "$$unformatted" ]; then echo "not gofmt'd:"; echo "$$unformatted"; exit 1; fi

water.test: *.go
	go test -c -race -o water.test .

clean:
	rm -f water.test water.test.exe
