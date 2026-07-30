REL_OSARCH="linux/amd64"
BIN_DIR=_output/bin
IMAGE_PREFIX ?= volcanosh
IMAGE_NAME ?= numatopo
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64
BUILDX_OUTPUT_TYPE ?= docker
include Makefile.def
.EXPORT_ALL_VARIABLES:

init:
	mkdir -p ${BIN_DIR}

fmt:
	go fmt ./pkg/...
	go fmt ./main.go

vet:
	go vet ./pkg/...
	go vet ./main.go

GOLANGCI_LINT ?= golangci-lint

lint:
	$(GOLANGCI_LINT) run --default=none --enable=ineffassign --enable=unused ./...

verify: fmt vet lint

build: init
	CGO_ENABLED=0 go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/numatopo ./

unit-test:
	go test -v ./pkg/...

image: init fmt vet lint
	docker build --no-cache \
		--build-arg LD_FLAGS=${LD_FLAGS} \
		-t ${IMAGE_PREFIX}/${IMAGE_NAME}:${TAG} \
		-f ./docker/Dockerfile .

release: init
	docker buildx build \
		--platform ${DOCKER_PLATFORMS} \
		--output=type=${BUILDX_OUTPUT_TYPE} \
		--build-arg LD_FLAGS=${LD_FLAGS} \
		-t ${IMAGE_PREFIX}/${IMAGE_NAME}:${TAG} \
		-t ${IMAGE_PREFIX}/${IMAGE_NAME}:${RELEASE_VER} \
		-f ./docker/Dockerfile .

clean:
	rm -rf ${BIN_DIR}
