up: certs
	docker-compose up

clean:
	docker-compose down
	rm -r data
	rm -r certs

.PHONY: certs
certs:
	./generate.sh

init-db:
	docker exec -ti cockroach-crdb-1
	sh -c "/cockroach/cockroach sql --url='postgres://localhost:26257/defaultdb?sslmode=verify-full&sslrootcert=/certs/ca.crt&sslcert=/certs/root.crt&sslkey=/certs/root.key&sslmode=verify-full' < /scripts/init.sql"

sql:
	COCKROACH-SQL --url='postgres://localhost:26257/teleport_backend?sslmode=verify-full&sslrootcert=certs/ca.crt&sslcert=certs/root.crt&sslkey=certs/root.key&sslmode=verify-full'

user:
	docker exec -ti cockroach-teleport-1 tctl users add --roles=editor,auditor,access --logins=root teleport
