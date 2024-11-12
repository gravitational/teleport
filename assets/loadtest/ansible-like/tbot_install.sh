#!/bin/sh
cd "$( dirname -- "${0}" )" || exit 1

sudo loginctl enable-linger

mkdir -p ~/.config/systemd/user/ &&
cp tbot.service ~/.config/systemd/user/ && \
systemctl --user daemon-reload && \
systemctl --user enable tbot && \
systemctl --user restart tbot && \
journalctl --user-unit tbot --follow
