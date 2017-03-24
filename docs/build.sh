#!/bin/bash
cd $(dirname $0)

mkdocs build --config-file 1.3.yaml
mkdocs build --config-file 2.0.yaml
