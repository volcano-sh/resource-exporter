REL_OSARCH = linux/amd64
BIN_DIR = _output/bin
IMAGE_REPO ?= volcanosh/numatopo

include Makefile.def

.EXPORT_ALL_VARIABLES:

.PHONY: init fmt lint test build image clean

init:
	@mkdir -p ${BIN_DIR}

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

test:
	go test -race ./...

build: init
	CGO_ENABLED=0 go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/numatopo ./

image:
	docker build --no-cache \
		--build-arg VERSION=${RELEASE_VER} \
		--build-arg GIT_SHA=${GitSHA} \
		--build-arg BUILD_DATE="${Date}" \
		-t ${IMAGE_REPO}:${TAG} \
		-f docker/Dockerfile .

clean:
	rm -rf ${BIN_DIR}
