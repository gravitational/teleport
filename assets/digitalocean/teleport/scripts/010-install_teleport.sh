#!/bin/bash
#
# Scripts in this directory are run during the build process.
# each script will be uploaded to /tmp on your build droplet, 
# given execute permissions and run.  The cleanup process will
# remove the scripts from your build system after they have run
# if you use the build_image task.
#


# get the lastest release
LATEST_VERSION=$(curl --silent "https://api.github.com/repos/gravitational/teleport/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
# download latest teleport install archive
curl -O https://get.gravitational.com/teleport-$LATEST_VERSION-linux-amd64-bin.tar.gz
# extract and install
tar -xzf teleport-$LATEST_VERSION-linux-amd64-bin.tar.gz
teleport/install
# remove install dir
rm -rf teleport && rm teleport-$LATEST_VERSION-linux-amd64-bin.tar.gz

# enable and start Teleport service
cat > /usr/lib/systemd/system/teleport.service <<EOM
[Unit]
Description=Teleport 7.1
After=network.target

[Service]
Type=simple
Restart=on-failure
ExecStart=/usr/local/bin/teleport start --pid-file=/run/teleport.pid
ExecReload=/bin/kill -HUP $MAINPID
PIDFile=/run/teleport.pid
LimitNOFILE=8192

[Install]
WantedBy=multi-user.target
EOM


# Add tasks that should be run in first login

chmod +x /opt/teleport/start_teleport.sh
cat >> /root/.bashrc <<EOM
/opt/teleport/start_teleport.sh
EOM