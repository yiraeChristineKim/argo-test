IMG ?= cluster-switch-controller:latest

.PHONY: build run docker-build docker-push fmt vet tidy

build: fmt vet
	go build -o bin/manager main.go

run: fmt vet
	go run main.go

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

docker-build:
	docker build -t $(IMG) .

docker-push:
	docker push $(IMG)
