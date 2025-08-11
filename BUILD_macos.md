# MacOS Environment Setup

For the best experience, follow these steps to set up your MacOS environment
using Homebrew as the primary dependency installer.

The instructions below are provided on a best-effort basis. PRs with corrections
and updates are welcome!

1. **Install [Homebrew](https://brew.sh/)**

1. **Install Go**
    1. Find the required Go version in
      [`build.assets/versions.mk`](/build.assets/versions.mk)
      (`GOLANG_VERSION`).

    1. Install Go (Homebrew only supports MAJOR.MINOR versions):

        ```shell
        # Replace <version> with the MAJOR.MINOR value of GOLANG_VERSION from build.assets/versions.mk (e.g., 1.24)
        brew install go@<version>
        ```

    1. Verify the installed version:

        ```shell
        go version
        ```

1. **Install Rust**
    1. **Install rustup**

        Install rustup with Homebrew:

        ```shell
        brew install rustup

        rustup-init
        # Accept defaults
        ```

    1. **Install and configure Rust toolchain**
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
            rustup default <version>
            ```

            > **Note:** Using `rustup default <version>` sets the toolchain globally
            > for your user. If you only want to override the toolchain for a
            > specific project directory, use `rustup override set <version>` inside
            > that directory instead.

        1. Verify the installed version:

            ```shell
            rustc --version
            ```

1. **Install Node.js**
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
      echo 'export PATH="/opt/homebrew/opt/node@22/bin:$PATH"' >> ~/.zshrc
      source ~/.zshrc
      ```

   1. Enable pnpm using the bundled corepack:

      ```shell
      corepack enable pnpm
      ```

1. **Install additional build dependencies**
   1. Install `wasm-pack`:
      1. Find the required wasm-pack version in
      [`build.assets/versions.mk`](/build.assets/versions.mk)
      (`WASM_PACK_VERSION`).

      1. Install wasm-pack globally:

          ```shell
          # Replace <version> with the value of WASM_PACK_VERSION from build.assets/versions.mk (e.g., 0.12.1)
          npm install --global wasm-pack@<version>
          ```

      1. Verify wasm-pack version:

          ```shell
          wasm-pack --version
          ```

   1. Install `wasm-bindgen-cli`:
      1. Find the required wasm-bindgen version in
        [`build.assets/versions.mk`](/build.assets/versions.mk)
        (`WASM_BINDGEN_VERSION`).

      1. Install wasm-bindgen-cli:

          ```shell
          cargo install wasm-bindgen-cli --force --locked --version <version>
          ```

   1. Install `libfido2` (pulls `openssl 3` as dependency):

      ```shell
      brew install libfido2
      ```

   1. Install `pkg-config`:

      ```shell
      brew install pkg-config
      ```

1. **Install test dependencies (optional)**
   1. Install `helm` and `helm-unittest` plugin:

      ```shell
      brew install helm
      helm plugin install https://github.com/quintush/helm-unittest --version 0.2.11
      ```

   1. Install `protoc`:
      1. Find the required protoc version in
         [`build.assets/versions.mk`](/build.assets/versions.mk)
         (`PROTOC_VERSION`).

      1. Install `protobuf` (Homebrew only supports MAJOR.MINOR versions):

          ```shell
          # Replace <version> with the value of PROTOC_VERSION from build.assets/versions.mk (e.g., 26.1)
          brew install protobuf@<version>
          ```

   1. Increase the maximum number of open files:

      ```shell
      ulimit -n 2560 # 10x default
      ```
