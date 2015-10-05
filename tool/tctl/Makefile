test: clean
	go test -v ./command -cover

test-grep: clean
	go test -v ./command  -check.f=$(e)

cover: clean
	go test -v ./command -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

clean:
	find . -name flymake_* -delete

sloccount:
	 find . -name "*.go" -print0 | xargs -0 wc -l

install: clean
	go install github.com/gravitational/teleport/tctl
