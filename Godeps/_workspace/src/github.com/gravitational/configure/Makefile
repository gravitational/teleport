.PHONY: test test-package remove-temp-files sloccount

test: remove-temp-files
	go test -v -test.parallel=0 ./... -cover

test-package: remove-temp-files
	go test -v -test.parallel=0 ./$(p)

remove-temp-files:
	find . -name flymake_* -delete

sloccount:
	find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l
