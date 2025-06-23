PHONY: test
test:
	go test -v -cover -timeout=120s -parallel=10 ./...

PHONY: testacc
testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: lint
lint:
	golangci-lint run -c ../../.golangci.yml