PROJECT_NAME=kubectl-strace
REPOSITORY=quay.io/mwasher

IMAGE=crictl

SUPPORTED_OS= darwin linux
SUPPORTED_ARCH=amd64 arm64

VERSION=0.0.1

build:
	GIT_COMMIT=$$(git rev-list -1 HEAD)
	go build -o bin/${PROJECT_NAME} -ldflags "-X cmd.Version.Tag=${VERSION} -X cmd.Version.Commit=$${GIT_COMMIT}" main.go 

run:
	go run main.go

docker:
	docker build -t ${REPOSITORY}/${IMAGE}:${VERSION} .
	docker push  ${REPOSITORY}/${IMAGE}:${VERSION}

all:   
	echo "Compiling for every OS and Platform"
	GIT_COMMIT=$$(git rev-list -1 HEAD)
	for RELEASE_OS in ${SUPPORTED_OS} ; do \
		for ARCH in ${SUPPORTED_ARCH} ; do \
		GOOS=$$RELEASE_OS GOARCH=$$ARCH go build -o bin/${PROJECT_NAME}-$$RELEASE_OS-$$ARCH -ldflags \
		"-X cmd.Version.Tag=${VERSION} -X cmd.Version.Commit=$${GIT_COMMIT}" main.go ; \
		done \
	done

test:   build
	go test -race ./...

test-e2e: build
	cd test && bash ./test.sh

release: all
	for RELEASE_OS in ${SUPPORTED_OS} ; do \
		for ARCH in ${SUPPORTED_ARCH}; do \
			echo "Building release for $$RELEASE_OS/$$ARCH" ; \
			RELEASE_NAME=${PROJECT_NAME}-$$RELEASE_OS-$$ARCH ; \
			mkdir $$RELEASE_NAME ; \
			cp LICENSE $$RELEASE_NAME/ ; \
			cp bin/$$RELEASE_NAME $$RELEASE_NAME/${PROJECT_NAME} ; \
			tar -czf $$RELEASE_NAME-${VERSION}.tar -C $$RELEASE_NAME LICENSE ${PROJECT_NAME} ;\
			rm -r $$RELEASE_NAME ;\
		done \
	done
