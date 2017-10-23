BINARY = unifi-captive-portal
GOARCH = amd64

VERSION?=?
COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

# Setup the -ldflags option for go build here, interpolate the variable values
LDFLAGS = -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Branch=${BRANCH}"

# Build the project
all: test fmt linux darwin windows

linux:
	GOOS=linux GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BINARY}-linux-${GOARCH}

darwin:
	GOOS=darwin GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BINARY}-darwin-${GOARCH}

windows:
	GOOS=windows GOARCH=${GOARCH} go build ${LDFLAGS} -o ${BINARY}-windows-${GOARCH}.exe

test:
	go test -v

fmt:
	go fmt $$(go list ./... | grep -v /vendor/)

.PHONY: linux darwin windows test fmt
