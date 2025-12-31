## Login load-test agent

This directory contains a modified Teleport client that performs passwordless
logins using a saved passkey.

> [!CAUTION]
> This client is completely unsafe to use in any production environment.

### How to use

#### Generate a passkey

In this directory:

1. Create a `test` user, with passwordless login, and register your own Yubikey
2. `tsh logout`
3. Set your target cluster: `export TARGET_CLUSTER=teleport.example.com:443`
3. Log in as the test user: `tsh login "--proxy=$TARGET_CLUSTER" --auth=passwordless --user=test`
4. Generate a passkey:
   ```shell
   go run -tags=libfido2 github.com/gravitational/teleport/e/scripts/loadtest-login --proxy-addr=teleport.example.com:443 --device-name="foobar" generate ./device.json
   
   # 2025/12/08 14:56:01 INFO ALPN connection upgrade required trace.component=client web_proxy_addr=teleport.example.com:443 upgrade_required=false 
   # 2025/12/08 14:56:01 INFO no host login given, using default trace.component=client default_host_login=shaka 
   # Tap any security key 
   # Detected security key tap 
   # 2025/12/08 14:56:05 Registered test device "fake-mfa-device" for user "test" 
   # 2025/12/08 14:56:05 INFO no host login given, using default trace.component=client default_host_login=shaka 
   # 2025/12/08 14:56:06 INFO Loading SSH key trace.component=keyagent user=test cluster=teleport.example.com 
   # Successfully wrote device file "./device.json".
   ```
5. Log out of the test account `tsh logout`
6. Test that you can log in using the passkey:
   ```shell
   go run -tags=libfido2 github.com/gravitational/teleport/e/scripts/loadtest-login --proxy-addr=teleport.example.com:443 once ./device.json
   
   # 2025/12/08 14:59:12 INFO no host login given, using default trace.component=client default_host_login=shaka
   # 2025/12/08 14:59:13 INFO Loading SSH key trace.component=keyagent user=test cluster=teleport.example.com
   # Successfully logged in using device file "./device.json".
   ```
   
#### Do the load-test

There's a minimal bash script that copies the load-test binary on every node and
creates the required services. This is minimal and slow, you're free to build
your own automation if you have stronger requirements.

You are responsible for creating the load-test VMs beforehand, and registering
them into a Teleport cluster so you can `tsh` into them as `root`. Each VM must
have its own public IP and will perform 20 logins per minute.
All load-test VMs must have `lt-agent` in their name.

In this directory:

1. Build the `loadtest-login` CLI for the right CPU architecture. This requires:
   - a build instance with the right CPU architecture, or being a guru of cross-compilation
   - libfido2-dev
   - gcc
   - setting the `libfido2` build tag
2. If you built the login CLI remotely, copy the binary back to `./loadtest-login`
3. `export TARGET_CLUSTER="teleport.example.com:443"`
4. Configure the load-test agents with `./load.bash`
5. Run the `tsh` commands from the `load.bash` output to start the load-test.
