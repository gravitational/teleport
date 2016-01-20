clean:
	find . -name flymake_* -delete

test: clean
	go vet ./... # note that I added vetting step here to to see if there are anything outstanding
	go test -v ./... -cover

cover: clean
	go test -v . -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

sloccount:
	find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l
