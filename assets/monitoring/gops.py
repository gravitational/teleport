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


def collect():
    go = {}
    prev_stack = ""
    for line in sys.stdin:
        if "goroutine" in line:
            if prev_stack != "":
                go[prev_stack] = go.setdefault(prev_stack,0) + 1
                prev_stack = ""
        if ".go" in line:
            parts = line.split()
            prev_stack = parts[0]
    print json.dumps(go)

def diff(a, b):
    with open(a) as fa:
        ja = json.load(fa)
    with open(b) as fb:
        jb = json.load(fb)
    diff = {}
    not_in_a = {}
    not_in_b = {}
    total_diff = 0
    for key, count in ja.iteritems():
        if key in jb:
            diff[key] = jb[key] - count
            total_diff += diff[key]
        else:
            not_in_b[key] = count
            total_diff += count
    for key, count in jb.iteritems():
        if key not in ja:
            not_in_a[key] = count
            total_diff += count

    print "change from %s to %s" % (a, b)
    for key, count in sorted(diff.iteritems()):
        if count != 0:
            print key, ": ", count

    print "not in a"
    for key, count in sorted(not_in_a.iteritems()):
        if count != 0:
            print key, ": ", count

    print "not in b"
    for key, count in sorted(not_in_b.iteritems()):
        if count != 0:
            print key, ": ", count
    print "total diff:", total_diff

parser = argparse.ArgumentParser(
    description="parse and print diffs for go stack traces")

subparsers = parser.add_subparsers(
    title='subcommands',
    description='valid subcommands',
    help='pick one of the commands, collect or diff')

ccollect = subparsers.add_parser(
    'collect',
    help='collect collects the output from stack trace, e.g. gops stack <pid> | python go.py collect > /tmp/a')
ccollect.set_defaults(func='collect')

cdiff = subparsers.add_parser(
    'diff',
    help='diff diffs two collected stats, e.g. python go.py diff /tmp/a /tmp/b')
cdiff.add_argument("first_file")
cdiff.add_argument("second_file")
cdiff.set_defaults(func='diff')
    
args = parser.parse_args()

if args.func == 'collect':
    collect()
else:
    diff(args.first_file, args.second_file)
