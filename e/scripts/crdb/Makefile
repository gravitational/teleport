# up starts crdb in the background.
.PHONY: up
up:
	docker compose up --wait

# down stops crdb.
.PHONY: down
down:
	docker compose down

# test runs crdb backend tests ensuring crdb is running.
.PHONY: test
test: up test-go down

# test-go runs crdb backend tests.
.PHONY: test-go
test-go:
	export TELEPORT_BACKEND_TEST_TTL_DELETE_TIMEOUT=5m; \
	export TELEPORT_CRDB_TEST_PARAMS_JSON='{ \
		"conn_string":"postgres://root:teleport@localhost:26257/teleport?sslmode=disable", \
		"ttl_job_cron":"* * * * *" \
	}'; \
	go test ../../lib/backend/crdb -race -count 1
