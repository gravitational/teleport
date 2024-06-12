#!/bin/bash

until curl --output /dev/null --silent --head --fail http://grafana:3000; do
    echo 'waiting for grafana to respond'
    sleep 5
done

echo "grafana is up setting up dashboards and data sources"

curl -s -H "Content-Type: application/json" \
    -XPOST http://admin:admin@grafana:3000/api/datasources \
    -d @- <<EOF
{
    "name": "InfluxDB",
    "type": "influxdb",
    "access": "proxy",
    "url": "http://influxdb:8086",
    "database": "telegraf"
}
EOF

curl -X POST -d @/mnt/health-dashboard.json 'http://admin:admin@grafana:3000/api/dashboards/db' --header 'Content-Type: application/json'

echo "all done!"
