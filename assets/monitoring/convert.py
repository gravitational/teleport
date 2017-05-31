'''
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
'''

import fileinput
import collections
import sys
import json
import argparse

parser = argparse.ArgumentParser(
    description="convert exported Grafana dashboard to format that can be imported by Grafana dashboard, :/")
parser.add_argument("input_file")
parser.add_argument("output_file")

args = parser.parse_args()

with open(args.input_file) as f:
    out = f.read()
    out = out.replace("${DS_INFLUXDB}", "InfluxDB")
    input = json.loads(out)

output = {
    'dashboard': input
}
with open(args.output_file, "w+") as f:
    json.dump(output, f)
    
