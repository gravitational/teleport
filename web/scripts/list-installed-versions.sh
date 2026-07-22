#!/bin/bash

if [ $# -ne 1 ]; then
  echo "Usage: $0 <package-name>" >&2
  exit 1
fi

pnpm why -r "$1" --json | grep "\"$1\"" -A 3 | grep version | tr -s ' ' | cut -d ' ' -f 3 | sort | uniq
