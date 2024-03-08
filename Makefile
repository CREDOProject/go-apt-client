testDocker:
	@docker run --rm  -v .:/code goubu make test

build-devcontainer:
	docker build . -t goubu

test:
	go test -cover ./...

t="coverage.txt"
coverage:
	go test -coverprofile=$t ./... && go tool cover -html=$t && unlink $t

