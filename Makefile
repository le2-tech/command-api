-include .env
-include .env.${APP_ENV}

.PHONY: build build0 docker-build
.DEFAULT_GOAL := build
tag := command-api

# include ../_env/Makefile

build0:
	CGO_ENABLED=0 go build
	ls -lh command-api

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"
	ls -lh command-api
	#strip -u -r command-api
	#upx --best --lzma --force-macos command-api
	ls -lh command-api
	file command-api

docker-build:
	docker build --no-cache \
		--build-arg IMAGE_MIRROR=$(IMAGE_MIRROR) \
		--build-arg APT_REPOSITORY=$(APT_REPOSITORY) \
		--build-arg GOPROXY=$(GOPROXY) \
		-t $(tag) .
