#!/bin/bash
cd auth && tctl -c teleport.yaml "$@"
