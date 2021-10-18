PROJECT_NAME=kubectl-strace
VERSION=0.0.1
build:
	go build -o ${PROJECT_NAME} main.go

run:
	go run main.go


all:
	echo "Compiling for every OS and Platform"
	GOOS=linux GOARCH=amd64 go build -o bin/${PROJECT_NAME}-linux main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/${PROJECT_NAME}-darwin main.go

