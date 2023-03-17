.PHONY: test

test: pkg/img/cosign_fixtures_test.go
	go test ./...

pkg/img/cosign_fixtures_test.go: hack/cosign-fixtures.go
	go run hack/cosign-fixtures.go | gofmt > pkg/img/cosign_fixtures_test.go
