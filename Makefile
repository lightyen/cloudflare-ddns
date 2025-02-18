NAME := cloudflare-ddns
IMAGE_NAME := ${NAME}
VERSION := v0.0.0
DATE := $(shell date +%Y-%m%d-%H%M)

GO_BUILD := go build -mod=vendor
ifeq (,$(wildcard ./vendor))
	GO_BUILD = go build
endif


ifeq ($(shell git diff-index --quiet HEAD 2> /dev/null || echo fail), fail)
	VERSION := untracked.$(shell git rev-parse --verify HEAD --short)
else ifeq ($(shell git rev-parse HEAD), $(shell git rev-list --max-count=1 $(shell git describe --abbrev=0)))
	VERSION := $(shell git describe --abbrev)
else
	VERSION := $(shell git rev-parse --abbrev-ref HEAD).$(shell git rev-parse --verify HEAD --short)
endif

GO_FLAGS := "-tags=nomsgpack"

LDFLAGS := -s -w -X github.com/lightyen/${NAME}/config.Version=${VERSION}-${DATE}

all: binary

binary:
	GOTOOLCHAIN=auto GOFLAGS=${GO_FLAGS} ${GO_BUILD} -ldflags="${LDFLAGS}" -o app

docker: binary
	docker buildx build -t ${IMAGE_NAME}:${VERSION} .
	@mkdir -p build
	@docker save -o build/${IMAGE_NAME}-${VERSION}.tar ${IMAGE_NAME}:${VERSION}
	# docker push ${IMAGE_NAME}:${VERSION}

clean:
	@docker system prune -a

check:
	GOFLAGS=${GO_FLAGS} golangci-lint run
#	syft scan ddns:0.0.0
#	grype ddns:0.0.0
