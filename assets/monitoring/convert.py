'''
Teleport
Copyright (C) 2023  Gravitational, Inc.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
    
