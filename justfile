PKG_NAME := `basename $(go list)`
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo dev`
BUILD_FLAGS := "-trimpath -ldflags=\"-s -w -X github.com/AbeEstrada/tuit/constants.AppVersion=" + VERSION + "\""

PREFIX := env("PREFIX", "/usr/local")

default: install

build:
    go build {{BUILD_FLAGS}} -o {{PKG_NAME}} .

install: build
    mkdir -p {{PREFIX}}/bin/
    mv {{PKG_NAME}} {{PREFIX}}/bin/

uninstall:
    rm -f {{PREFIX}}/bin/{{PKG_NAME}}

clean:
    rm -f {{PKG_NAME}} {{PKG_NAME}}-race

test:
    go test ./...

lint:
    test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
    go vet ./...

fmt:
    gofmt -w .

race:
    go build -race -o {{PKG_NAME}}-race .
