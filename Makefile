.PHONY: hooks test lint e2e verify

hooks:
	git config core.hooksPath scripts

verify:
	./scripts/verify.sh

test:
	go test -v -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

e2e:
	npm run test:e2e
