/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react'
import { storiesOf } from '@storybook/react'
import ConfigMapEditor from './ConfigMapEditor'

storiesOf('Gravity/K8s', module)
  .add('ConfigMapDialog', () => {
    const props = {
      onSave(){
        throw Error('some server error')
      },

      onClose(){
      },

      configMap: {
        namespace: 'kube-system',
        data,
        name: "grafana-cfg"
      }
    }

    return (
      <ConfigMapEditor {...props} />
    );
  });

const data = [
  {
    "name": "high_cpu.tick.long.name.on.purpose",
    "content": "var period = 5m\nvar every = 1m\nvar warnRate = 75\nvar warnReset = 50\nvar critRate = 90\nvar critReset = 75\n\nvar usage_rate = stream\n    |from()\n        .measurement('cpu/usage_rate')\n        .groupBy('nodename')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar cpu_total = stream\n    |from()\n        .measurement('cpu/node_capacity')\n        .groupBy('nodename')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar percent_used = usage_rate\n    |join(cpu_total)\n        .as('usage_rate', 'total')\n        .tolerance(30s)\n        .streamName('percent_used')\n    |eval(lambda: (float(\"usage_rate.value\") * 100.0) / float(\"total.value\"))\n        .as('percent_usage')\n    |mean('percent_usage')\n        .as('avg_percent_used')\n\nvar trigger = percent_used\n    |alert()\n        .message('{{ .Level}} / Node {{ index .Tags \"nodename\" }} has high cpu usage: {{ index .Fields \"avg_percent_used\" }}%')\n        .warn(lambda: \"avg_percent_used\" > warnRate)\n        .warnReset(lambda: \"avg_percent_used\" < warnReset)\n        .crit(lambda: \"avg_percent_used\" > critRate)\n        .critReset(lambda: \"avg_percent_used\" < critReset)\n        .stateChangesOnly()\n        .details('''\n<b>{{ .Message }}</b>\n<p>Level: {{ .Level }}</p>\n<p>Nodename: {{ index .Tags \"nodename\" }}</p>\n<p>Usage: {{ index .Fields \"avg_percent_used\"  | printf \"%0.2f\" }}%</p>\n''')\n        .email()\n        .log('/var/lib/kapacitor/logs/high_cpu.log')\n        .mode(0644)\n"
  },
  {
    "name": "filesystem.tick",
    "content": "var period = 1m\nvar every = 1m\nvar warnRate = 80\nvar warnReset = 70\nvar critRate = 90\nvar critReset = 80\n\nvar usage = stream\n    |from()\n        .measurement('filesystem/usage')\n        .groupBy('nodename', 'resource_id')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar total = stream\n    |from()\n        .measurement('filesystem/limit')\n        .groupBy('nodename', 'resource_id')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar percent_used = usage\n    |join(total)\n        .as('usage', 'total')\n        .tolerance(30s)\n        .streamName('percent_used')\n    |eval(lambda: (float(\"usage.value\") * 100.0) / float(\"total.value\"))\n        .as('percent_used')\n\nvar trigger = percent_used\n    |alert()\n        .message('{{ .Level}} / Node {{ index .Tags \"nodename\" }} has low free space on {{ index .Tags \"resource_id\" }}')\n        .warn(lambda: \"percent_used\" > warnRate)\n        .warnReset(lambda: \"percent_used\" < warnReset)\n        .crit(lambda: \"percent_used\" > critRate)\n        .critReset(lambda: \"percent_used\" < critReset)\n        .stateChangesOnly()\n        .details('''\n<b>{{ .Message }}</b>\n<p>Level: {{ .Level }}</p>\n<p>Nodename: {{ index .Tags \"nodename\" }}</p>\n<p>Resource: {{ index .Tags \"resource_id\" }}</p>\n<p>Usage: {{ index .Fields \"percent_used\"  | printf \"%0.2f\" }}%</p>\n''')\n        .email()\n        .log('/var/lib/kapacitor/logs/filesystem.log')\n        .mode(0644)\n\nvar warnInodes = 90\nvar warnInodesReset = 80\nvar critInodes = 95\nvar critInodesReset = 90\n\nvar inodes_free = stream\n    |from()\n        .measurement('filesystem/inodes_free')\n        .groupBy('nodename', 'resource_id')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar inodes_total = stream\n    |from()\n        .measurement('filesystem/inodes')\n        .groupBy('nodename', 'resource_id')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n\nvar percent_used_inodes = inodes_free\n    |join(inodes_total)\n        .as('free', 'total')\n        .tolerance(30s)\n    |eval(lambda: (100.0 - (float(\"free.value\") * 100.0) / float(\"total.value\")))\n        .as('percent_used_inodes')\n\nvar trigger_inodes = percent_used_inodes\n    |alert()\n        .message('{{ .Level}} / Node {{ index .Tags \"nodename\" }} has low free inodes on {{ index .Tags \"resource_id\" }}')\n        .warn(lambda: \"percent_used_inodes\" > warnInodes)\n        .warnReset(lambda: \"percent_used_inodes\" < warnInodesReset)\n        .crit(lambda: \"percent_used_inodes\" > critInodes)\n        .critReset(lambda: \"percent_used_inodes\" < critInodesReset)\n        .stateChangesOnly()\n        .details('''\n<b>{{ .Message }}</b>\n<p>Level: {{ .Level }}</p>\n<p>Nodename: {{ index .Tags \"nodename\" }}</p>\n<p>Resource: {{ index .Tags \"resource_id\" }}</p>\n<p>Usage: {{ index .Fields \"percent_used_inodes\"  | printf \"%0.2f\" }}%</p>\n''')\n        .email()\n        .log('/var/lib/kapacitor/logs/inodes.log')\n        .mode(0644)\n"
  },
  {
    "name": "etcd_latency_batch.tick",
    "content": "var period = 5m\nvar every = 1m\nvar data_etcd_latency = batch\n    |query('''SELECT (DERIVATIVE(count,1m) * 0.95) AS count, DERIVATIVE(\"0.512\",1m) AS v512, DERIVATIVE(\"1.024\",1m) AS v1024 FROM \"k8s\".\"default\".\"etcd_rafthttp_message_sent_latency_seconds\" WHERE \"msgType\" = 'MsgHeartbeat' AND \"sendingType\" = 'message' ''')\n        .period(period)\n        .every(every)\n        .groupBy('remoteID')\n\nvar count = data_etcd_latency\n    |mean('count')\nvar v512 = data_etcd_latency\n    |mean('v512')\nvar v1024 = data_etcd_latency\n    |mean('v1024')\n\nvar trigger_etcd_latency = count\n    |join(v512,v1024)\n        .as('count', 'v512', 'v1024')\n        .tolerance(10s)\n\ntrigger_etcd_latency\n    |alert()\n        .message('{{ .Level }} / etcd: High latency between leader and follower {{ index .Tags \"followerName\" }}')\n        .warn(lambda: \"count.mean\" > \"v512.mean\")\n        .crit(lambda: \"count.mean\" > \"v1024.mean\")\n        .details('''\n<b>{{ .Message }}</b>\n<p>Level: {{ .Level }}</p>\n<p>etcd instance: {{ index .Tags \"followerName\" }}</p>\n<p>Latency greater than: {{ if eq .Level \"WARNING\" }}512{{ else }}1024{{ end }}ms</p>\n''')\n        .email()\n        .log('/var/lib/kapacitor/logs/etcd_latency.log')\n        .mode(0644)\n"
  },
  {
    "name": "influxdb_health_batch.tick",
    "content": "var period = 5m\nvar every = 1m\nvar data_influxdb_health = batch\n    |query('''SELECT * FROM \"k8s\".\"default\".\"cpu/usage\"''')\n        .period(period)\n        .every(every)\n\nvar deadman_influxdb_health = data_influxdb_health\n    |deadman(0.0, 5m)\n        .message('InfluxDB is down or no connection between Kapacitor and InfluxDB')\n        .email()\n        .log('/var/lib/kapacitor/logs/influxdb_health.log')\n        .mode(0644)\n"
  },
  {
    "name": "uptime.tick",
    "content": "var period = 1m\nvar every = 1m\nvar warn = 300 // seconds\nvar warnReset = 600 // seconds\n\nvar node_down = stream\n    |from()\n        .measurement('uptime')\n        .groupBy('*')\n        .where(lambda: \"type\" == 'node')\n    |deadman(0.0, 5m)\n        .message('Node {{ index .Tags \"nodename\" }} is down')\n        .stateChangesOnly()\n        .email()\n        .log('/var/lib/kapacitor/logs/node_down.log')\n        .mode(0644)\n\nvar uptime = stream\n    |from()\n        .measurement('uptime')\n        .groupBy('nodename')\n        .where(lambda: \"type\" == 'node')\n    |window()\n        .period(period)\n        .every(every)\n    |eval(lambda: ceil(float(\"value\") / 1000.0))\n        .as('uptime')\n\nvar trigger = uptime\n    |alert()\n        .message('{{ .Level }} / Node {{ index .Tags \"nodename\" }} was rebooted')\n        .warn(lambda: \"uptime\" < warn)\n        .warnReset(lambda: \"uptime\" > warnReset)\n        .stateChangesOnly()\n        .details('''\n<b>{{ .Message }}</b>\n<p>Level: {{ .Level }}</p>\n<p>Nodename: {{ index .Tags \"nodename\" }}</p>\n<p>Uptime: {{ index .Fields \"uptime\" }} sec</p>\n''')\n        .email()\n        .log('/var/lib/kapacitor/logs/uptime.log')\n        .mode(0644)\n"
  },
]