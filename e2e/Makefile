.PHONY: test clean stop build
test:
	docker compose up --abort-on-container-exit --exit-code-from e2e

clean:
	docker volume rm  e2e_teleport-config e2e_teleport-data

stop:
	docker compose down

build:
	docker compose build
	docker compose create