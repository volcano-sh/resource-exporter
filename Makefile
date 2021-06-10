REL_OSARCH="linux/amd64"
BIN_DIR=_output/bin
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

lint:
	golint ./pkg/...
	golint ./main.go
   
image: init fmt vet lint
	CGO_ENABLED=0 go build -ldflags ${LD_FLAGS} -o ${BIN_DIR}/numatopo ./
	cp ${BIN_DIR}/numatopo ./docker/numatopo
	docker build --no-cache -t volcanosh/numatopo:${TAG} ./docker/
	rm -rf ./docker/numatopo

