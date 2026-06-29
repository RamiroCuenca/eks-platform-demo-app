.PHONY: build test vet fmt tidy run docker

build:
	go build ./...

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

tidy:
	go mod tidy

run:
	go run ./cmd/server

docker:
	docker build -t eks-platform-demo-app:dev .
