To test Linux desktop connection with local cluster:

1. Run `run_xrdp.sh`, that will create container with xrdp and xfce4 installed and listening on port 33890
2. Add static desktop as usual but add `teleport.dev/os: Linux` label:
    ```yaml
    windows_desktop_service:
      static_hosts:
        - name: linux
          addr: localhost:33890
          ad: false
          labels:
            teleport.dev/os: Linux
    ```
3. Add `host.docker.internal` as public address of your cluster so PAM module can download `jwks.json` automatically:
   ```yaml
    proxy_service:
      public_addr: ["your.regular.public.addr:3080", "host.docker.internal:3080"]
    ```
4. Connect as you normally would with username `newuser`, everything should work, clipboard, directory sharing etc