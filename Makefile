PROJECT_NAME=kubectl-strace
VERSION=0.0.1
REPOSITORY=quay.io/mwasher
IMAGE=crictl

build:
	go build -o ${PROJECT_NAME} main.go

run:
	go run main.go

docker:
	docker build -t ${REPOSITORY}/${IMAGE}:${VERSION} .
	docker push  ${REPOSITORY}/${IMAGE}:${VERSION}

all:
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=amd64 go build -o bin/${PROJECT_NAME}-linux main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/${PROJECT_NAME}-darwin main.go

