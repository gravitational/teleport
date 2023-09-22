# MacOS environment setup

The instructions below are provided as in a best-effort basis.
PRs with corrections and updates are welcome!

* `Go` version from
  [go.mod](https://github.com/gravitational/teleport/blob/master/go.mod#L3)
  
  * Follow [official instructions](https://go.dev/doc/install) to install `Go`
    * **On an M1 Mac, download ARM64 installer from https://go.dev/dl/**
    * Download the installer for `<version from go.mod>`  
    * After installing, don't forget to `export PATH="/usr/local/go/bin:$PATH"` in `~/.zprofile`
    * If you need other go versions, see https://go.dev/doc/manage-install
      * You will need to add `export PATH="$HOME/go/bin:$PATH"` to the `~/.zprofile`

  * Or install required version of `Go` with homebrew:

  ```shell
  # if we are not on the latest, you might need to install like this:
  # brew install go@<version from go.mod>, i.e. 1.16
  #
  # check which version will be installed by running:
  # brew info go
  
  brew install go
  ````

* `Rust` and `Cargo` version from
  [build.assets/Makefile](https://github.com/gravitational/teleport/blob/master/build.assets/Makefile#L21)
  (search for RUST_VERSION):

  * Follow [official instructions](https://www.rust-lang.org/tools/install) to install `rustup`
  *  Or install with homebrew:
  
  ```shell
  brew install rustup
  ```
  
  * Initialize Rustup
  
  ```shell
  rustup-init
  #
  # accept defaults
  #
  # Once command finishes successfully, you might need to add
  # 
  # export PATH="$HOME/.cargo/bin:$PATH"
  # 
  # into ~/.zprofile and run:
  # 
  # . ~/.zprofile
  # 
  # or open a new shell
  ```
  
  * Install the required version
  
  ```shell
  rustup toolchain install <version from build.assets/Makefile>
  cd <teleport.git>
  rustup override set <version from build.assets/Makefile>
  rustc --version                                                                                                                                                                  ─╯
  # rustc <version from build.assets/Makefile> (db9d1b20b 2022-01-20)
  ```

* To install `libfido2` (pulls `openssl 1.1.1` as dependency)
 
  ```shell
  brew install libfido2
  ```

##### Local Tests Dependencies
 
To run a full test suite locally, you will need

* `helm` and `helm-unittest` plugin
 
  ```shell
  brew install helm
  helm plugin install https://github.com/quintush/helm-unittest
  ```
  
* `bats-core` version from [build.assets/Dockerfile](https://github.com/gravitational/teleport/blob/master/build.assets/Dockerfile#L183) (search for `bats-core`)

  ```shell
  curl -L https://github.com/bats-core/bats-core/archive/v1.2.1.tar.gz -o ~/Downloads/bats.tar.gz
  cd ~/Downloads
  tar xzvf bats.tar.gz
  sudo mkdir /usr/local/libexec
  sudo chown $USER /usr/local/libexec
  cd bats-core-1.2.1
  sudo ./install.sh /usr/local
  cd ../
  rm -rf bats-core-1.2.1 bats.tar.gz
  ```

* `protoc` binary, typically found in `protobuf` package 

  ```shell
  brew install protobuf
  ```

* `gnu-sed` since Makefile tasks run inside linux containers typically

  ```shell
  brew install gnu-sed
  echo 'export PATH="/opt/homebrew/opt/gnu-sed/libexec/gnubin:$PATH"' >> ~/.zprofile
  ```
  
* increased `ulimit -n`
  
  ```shell
  ulimit -n 2560 # 10x default
  ```
