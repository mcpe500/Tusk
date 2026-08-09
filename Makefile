.PHONY: build test race vet fmt fmt-check check build-android clean

GO ?= go

build:
	$(GO) build -o bin/tusk ./cmd/tusk

test:
	$(GO) test -cover ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

fmt-check:
	@test -z "$$(gofmt -l .)"

build-android:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=android GOARCH=arm64 $(GO) build -buildvcs=false -o dist/tusk-android-arm64 ./cmd/tusk

check: fmt-check vet test race build-android

clean:
	rm -rf bin dist coverage.out
