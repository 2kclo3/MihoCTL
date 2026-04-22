PACKAGE_TARGET ?= linux
PACKAGE_ARCH ?= amd64

.PHONY: build package package-linux-amd64

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mihoctl .

package:
	GOOS=$(PACKAGE_TARGET) GOARCH=$(PACKAGE_ARCH) ./scripts/package_release.sh

package-linux-amd64:
	GOOS=linux GOARCH=amd64 ./scripts/package_release.sh
