#!/bin/bash
cd auth && tctl -c teleport.yaml $1 $2 $3
