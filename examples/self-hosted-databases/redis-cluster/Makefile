self-signed:
	openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout self-signed.key -out self-signed.crt -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP.1:127.0.0.1" -addext "extendedKeyUsage = serverAuth, clientAuth"

init: self-signed
	cp self-signed.crt node.crt
	cp self-signed.key node.key
	cp self-signed.crt node.cas
	tctl auth export --type db-client >> node.cas
	chmod a+r node*
up:
	docker-compose up -d
down:
	docker-compose down

dump:
	@printf "%s\n" \
      'db_service:' \
      '  enabled: "yes"' \
      '  databases:' \
      '  - name: "self-hosted-redis-cluster"' \
      '    protocol: "redis"'\
      '    uri: "rediss://127.0.0.1:17000?mode=cluster"' \
      '    tls:' \
      '      ca_cert_file: $(CURDIR)/self-signed.crt' \
      '    static_labels: ' \
	  '      "env": "teleport-examples" '

.PHONY: clean
clean: down
	rm -rf node*.crt node*.cas node*.key
