#!/bin/bash
rsync -av -e "ssh -p 61722" . ekontsevoy@host.kontsevoy.com:gravitational.io/demo/ 
