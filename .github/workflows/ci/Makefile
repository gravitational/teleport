
# Runs subcommand assign-reviewers
.PHONY: assign
assign: 
	go run cmd/bot.go assign-reviewers 

.PHONY: check
# Runs subcommand check-reviewers
check: 
	go run cmd/bot.go check-reviewers 

.PHONY: test
# Runs all Go tests and shows the coverage
test: 
	go test ./... -cover

.PHONY: lint
lint: 
	golangci-lint run
