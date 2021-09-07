## Install

#### MacOS

First ensure that `libcrypto` and `libssl` are in your include path. If you use `brew`, this can be done by running

```
brew install openssl@1.1
sudo cp /usr/local/Cellar/openssl@1.1/1.1.1k/lib/libcrypto.* /usr/local/Cellar/openssl@1.1/1.1.1k/lib/libssl.* /usr/local/lib/
```

## Run

To run the test client, from the `rdpclient` directory execute:

```sh
./run.sh <windows host address>:3389 <windows username> <windows password>
```

After it starts, open http://localhost:8080 and click `connect`.
