SHELL := /bin/bash
BASEDIR = $(shell pwd)

export GO111MODULE=on
export GOPROXY=https://goproxy.cn,direct
export GOPRIVATE=*.gitlab.com,git.makeblock.com
export GOSUMDB=off

# params pass from cmd
APP_BRANCH = "main"

# params stable
APP_NAME=`cat package.json | grep name | head -1 | awk -F: '{ print $$2 }' | sed 's/[\",]//g' | tr -d '[[:space:]]'`
APP_VERSION=`cat package.json | grep version | head -1 | awk -F: '{ print $$2 }' | sed 's/[\",]//g' | tr -d '[[:space:]]'`
COMMIT_ID=`git rev-parse HEAD`
IMAGE_PREFIX="registry.cn-hangzhou.aliyuncs.com/makeblock/${APP_NAME}:v${APP_VERSION}"

all: fmt imports mod lint test
install-pre-commit:
	brew install pre-commit
install-git-hooks:
	pre-commit install --hook-type commit-msg
	pre-commit install
run-pre-commit:
	pre-commit run --all-files
fmt:
	gofmt -w .
imports:
ifeq (, $(shell which goimports))
	go install golang.org/x/tools/cmd/goimports@latest
endif
	goimports -w .
mod:
	go mod tidy
lint: mod
ifeq (, $(shell which golangci-lint))
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1
endif
	golangci-lint run -c .golangci.yml
.PHONY: test
test: mod
	go test -gcflags=-l -coverpkg=./... -coverprofile=coverage.data ./...
.PHONY: build
build:
	IMAGE_NAME="${IMAGE_PREFIX}-${APP_BRANCH}"; \
	sh build/package/build.sh ${COMMIT_ID} $$IMAGE_NAME
build-main:
	make build APP_BRANCH=main
build-release:
	make build APP_BRANCH=release
cleanup:
	sh scripts/cleanup.sh
help:
	@echo "fmt - format the source code"
	@echo "imports - goimports"
	@echo "mod - go mod tidy"
	@echo "lint - run golangci-lint"
	@echo "test - unit test"
	@echo "build - build docker image"
	@echo "build-main - build docker image for main branch"
	@echo "build-release - build docker image for release branch"
	@echo "cleanup - clean up the build binary"
