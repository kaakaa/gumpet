VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build run test vet fmt clean windows licenses notices

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/ ./cmd/...

run:
	go run ./cmd/gumpet

# Cross-compiling to Windows works from any host: Ebitengine needs no cgo
# there. -H=windowsgui keeps a console window from opening alongside the pet.
windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS) -H=windowsgui" -o bin/windows-amd64/gumpet.exe ./cmd/gumpet
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/windows-amd64/gumpetctl.exe ./cmd/gumpetctl

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# Both need go-licenses: go install github.com/google/go-licenses/v2@v2.0.1
licenses:
	.github/licenses/check.sh

notices:
	@mkdir -p bin
	.github/licenses/notices.sh $$(go env GOOS) $$(go env GOARCH) > bin/THIRD_PARTY_NOTICES

clean:
	rm -rf bin
