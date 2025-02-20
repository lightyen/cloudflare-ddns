VERSION := 0.0.0

ifeq ($(shell git rev-parse --git-dir 2>&1 >/dev/null && echo 0), 0)
	TAG := $(shell git describe --tags --abbrev=0 2> /dev/null)
	ifeq (,$(TAG))
		TAG := 0.0.0
	endif

	CURRENT := $(shell git rev-parse --verify HEAD 2> /dev/null)
	ifneq (,$(shell git status --short 2> /dev/null))
		ifeq (,$(CURRENT))
			VERSION := $(TAG)-untracked
		else
			VERSION := $(TAG)-untracked+$(shell git rev-parse --verify HEAD --short)
		endif
	else
		ifneq ($(CURRENT), $(shell git rev-list --max-count=1 $(shell git describe --tags --abbrev=0 2> /dev/null) 2> /dev/null))
			VERSION := $(TAG)-$(shell git rev-parse --abbrev-ref HEAD)+$(shell git rev-parse --verify HEAD --short)
		else
			VERSION := $(TAG)
		endif
	endif
endif

NAME := cloudflare-ddns
IMAGE_NAME := ${NAME}

DATE := $(shell date --rfc-3339=seconds)

GO_FLAGS := "-tags=nomsgpack"

LDFLAGS := -s -w -X 'github.com/lightyen/${NAME}/settings.Version=${VERSION}' -X 'github.com/lightyen/${NAME}/settings.BuildTime=${DATE}'

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
