ifneq ($(shell git rev-parse --git-dir 2>&1 >/dev/null && echo 0), 0)
	VERSION := 0.0.0
else
	ifneq (,$(shell git status --short 2> /dev/null))
		hash := $(shell git rev-parse --verify HEAD --short 2> /dev/null)
		ifneq (,$(hash))
			VERSION := untracked.$(hash)
		else
			VERSION := untracked
		endif
	else
		ifneq ($(shell git rev-parse HEAD), $(shell git rev-list --max-count=1 2> /dev/null $(shell git describe --abbrev=0 2> /dev/null)))
			VERSION := $(shell git rev-parse --abbrev-ref HEAD).$(shell git rev-parse --verify HEAD --short)
		else
			VERSION := $(shell git describe --abbrev)
		endif
	endif
endif

NAME := cloudflare-ddns
IMAGE_NAME := ${NAME}
DATE := $(shell date +%Y-%m%d-%H%M)

GO_FLAGS := "-tags=nomsgpack"

LDFLAGS := -s -w -X github.com/lightyen/${NAME}/config.Version=${VERSION}-${DATE}

all: binary

binary:
	GOTOOLCHAIN=auto GOFLAGS=${GO_FLAGS} go build -ldflags="${LDFLAGS}" -o app

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
