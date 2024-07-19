init:
	tctl auth sign --format=redis --host=localhost --out=single --ttl=2190h
	chmod a+r single*
up:
	docker run -d --name redis -v $(PWD):/opt/redis -p 6379:6379 redis:7.0 redis-server /opt/redis/single.conf

down:
	docker rm -f redis

dump:
	@printf "%s\n" \
      'db_service:' \
      '  enabled: "yes"' \
      '  databases:' \
      '  - name: "self-hosted-redis"' \
      '    protocol: "redis"'\
      '    uri: "localhost:6379"' \
      '    static_labels: ' \
	  '      "env": "teleport-examples" '

.PHONY: clean
clean: down
	rm -rf single*.crt single*.cas single*.key
