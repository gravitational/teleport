# MacOS Environment Setup

To set up your MacOS environment, follow these steps using Homebrew as the main
package manager. Aim to install versions specified in
[`build.assets/versions.mk`](/build.assets/versions.mk); for others, use the
latest Homebrew version.

The instructions below are provided on a best-effort basis. PRs with corrections
and updates are welcome!

1. Install [Homebrew](https://brew.sh/)

1. Install Go

      ```shell
      brew install go
      ```

1. Install Rust
    1. Install rustup

        Install rustup with Homebrew:

        ```shell
        brew install rustup

        rustup-init
        # Accept defaults
        ```

    1. Install and configure Rust toolchain
        1. Find the required Rust version in
            [`build.assets/versions.mk`](/build.assets/versions.mk)
            (`RUST_VERSION`).

        1. Install the Rust toolchain:

            ```shell
            # Replace <version> with the value of RUST_VERSION from build.assets/versions.mk (e.g., 1.81.0)
            rustup toolchain install <version>
            ```

        1. Set the default Rust toolchain globally (applies to all projects):

            ```shell
            # Replace <version> with the value of RUST_VERSION from build.assets/versions.mk (e.g., 1.81.0)
            rustup default <version>
            ```

            > **Note:** Using `rustup default <version>` sets the toolchain
            > globally for your user. If you only want to override the toolchain
            > for a specific project directory, use `rustup override set
            > <version>` inside that directory instead.

        1. Verify the installed version:

            ```shell
            rustc --version
            ```

1. Install Node.js
    1. Find the required Node version in
      [`build.assets/versions.mk`](/build.assets/versions.mk) (`NODE_VERSION`).

    1. Install Node.js (Homebrew only supports MAJOR version):

        ```shell
        # Replace <version> with the MAJOR value of NODE_VERSION from build.assets/versions.mk (e.g., 22)
        brew install node@<version>
        ```

    1. Install to PATH and apply the changes to your shell:

        ```shell
        # Replace <version> with the MAJOR value of NODE_VERSION from build.assets/versions.mk (e.g., 22)
        echo 'export PATH="/opt/homebrew/opt/node@<version>/bin:$PATH"' >> ~/.zshrc

        source ~/.zshrc
        ```

    1. Verify the installed version:

        ```shell
        node --version
        ```

1. Install `libfido2`:

    ```shell
    brew install libfido2
    ```

1. Install `pkg-config`:

    ```shell
    brew install pkg-config
    ```

1. Install `helm` and the `helm-unittest` plugin:

    ```shell
    brew install helm

    helm plugin install https://github.com/quintush/helm-unittest --version 0.2.11
    ```

1. Install `bats`:
    1. Find the required `bats-core` version from
        [`build.assets/Dockerfile`](/build.assets/Dockerfile) (search for
        `bats-core`).
    1. Set the version variable and install `bats-core`:

        ```shell
        # Replace <version> with the required bats-core version (e.g., 1.12.0)
        BATS_VERSION=1.12.0

        curl -L https://github.com/bats-core/bats-core/archive/v${BATS_VERSION}.tar.gz -o ~/Downloads/bats.tar.gz
        cd ~/Downloads
        tar xzvf bats.tar.gz
        sudo mkdir -p /usr/local/libexec
        sudo chown $USER /usr/local/libexec
        cd bats-core-${BATS_VERSION}
        sudo ./install.sh /usr/local
        cd ../
        rm -rf bats-core-${BATS_VERSION} bats.tar.gz
        ```

    1. Verify `bats` installation:

          ```shell
          bats --version
          ```

1. Increase the maximum number of open files:

    ```shell
    ulimit -n 2560 # 10x default
    ```

1. Test the environment by building development artifacts and running tests:

    ```shell
    make all test
    ```

Congrats! Your MacOS environment is now ready for development 🎉

If you encounter any issues, please refer to the [official
documentation](https://goteleport.com/docs/) or [open an
issue](https://github.com/gravitational/teleport/issues) in the repository for
assistance.
