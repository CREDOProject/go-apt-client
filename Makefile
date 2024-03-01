testDocker:
	@docker run --rm  -v .:/code goubu make test

build-devcontainer:
	docker build . -t goubu

test:
	go test ./...
