NAME=turbo
IMAGE_NAME=asim/$(NAME)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_TAG=$(shell git describe --abbrev=0 --tags --always --match "v*")
GIT_IMPORT=github.com/asim/turbo
BUILD_DATE=$(shell date +%s)
IMAGE_TAG=$(GIT_TAG)-$(GIT_COMMIT)

all: build

build:
	go build -a -installsuffix cgo -o $(NAME) ./cmd/turbo/main.go

docker:
	docker buildx build --platform linux/amd64 --platform linux/arm64 --tag $(IMAGE_NAME):$(IMAGE_TAG) --tag $(IMAGE_NAME):latest --push .

vet:
	go vet ./...

test: vet
	go test -v ./...

clean:
	rm -rf ./turbo

.PHONY: build clean vet test docker
