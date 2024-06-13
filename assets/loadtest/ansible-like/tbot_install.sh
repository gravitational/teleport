#!/bin/sh
cd $( dirname -- ${0} )

sudo loginctl enable-linger ${USER}

curl https://goteleport.com/static/install.sh | sudo bash -s 15.4.3 enterprise || exit $?

mkdir -p ~/.config/systemd/user/ &&
cp tbot.service ~/.config/systemd/user/ && \
systemctl --user daemon-reload && \
systemctl --user enable tbot && \
systemctl --user restart tbot && \
journalctl --user-unit tbot --follow
