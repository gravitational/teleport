#!/bin/bash
make full
sudo rm /usr/local/bin/teleport
sudo rm /usr/local/bin/tsh
sudo rm /usr/local/bin/tctl

sudo cp build/teleport /usr/local/bin


sudo cp build/tsh /usr/local/bin


sudo cp build/tctl /usr/local/bin

