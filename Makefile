PROJECT_NAME=kubectl-strace
VERSION=0.0.2
REPOSITORY=quay.io/mwasher
IMAGE=crictl

build:
	go build -o bin/${PROJECT_NAME} main.go

run:
	go run main.go

docker:
	docker build -t ${REPOSITORY}/${IMAGE}:${VERSION} .
	docker push  ${REPOSITORY}/${IMAGE}:${VERSION}

all:    build
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=amd64 go build -o bin/${PROJECT_NAME}-linux main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/${PROJECT_NAME}-darwin main.go

test:   build
	go test -race ./...

test-e2e: build
	cd test && bash ./basic_test.sh || kubectl get pods -o yaml -A && kubectl get events -A
