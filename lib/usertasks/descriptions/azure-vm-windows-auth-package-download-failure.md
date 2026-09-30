# Installer download failure

The VM could not download the Teleport Windows Authentication Package installer
from the Teleport CDN.

This usually means the VM does not have outbound HTTPS connectivity to the Teleport
CDN (https://cdn.teleport.dev), or a firewall, NSG, or proxy is blocking the
request. If your network requires a proxy for outbound traffic, confirm the
discovery config's `http_proxy_settings` are set correctly.
