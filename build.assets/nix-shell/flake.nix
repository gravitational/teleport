# Copyright 2023 Gravitational, Inc.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#
# This file contains the dependencies for the Teleport nix shell, which contains
# all of the utilities for building and linting Teleport.
#

{
  description = "Teleport shell dependencies";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # general packages

    # Linting dependencies
    helmPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # helm 3.11.1
    golangci-lintPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # golangci-lint 1.51.2
    shellcheckPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # shellcheck 0.9.0
    yamllintPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # yamllint 1.28.0
    gciPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # gci 0.9.1
    addlicensePkgs.url = "github:nixos/nixpkgs/f597e7e9fcf37d8ed14a12835ede0a7d362314bd"; # addlicense 1.0.0

    # Rust and GCC dependencies
    gccPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # gcc 12.2.0
    libiconvPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # libiconv 1.16
    libbpfPkgs.url = "github:nixos/nixpkgs/79b3d4bcae8c7007c9fd51c279a8a67acfa73a2a"; # libbpf 1.0.1
    libfido2Pkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # libfido2 1.12.0

    # UI dependencies
    pythonPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # python 3.11.2
    nodePkgs.url = "github:nixos/nixpkgs/a3d5f09dfd7134153136d3153820a0642898cc9d"; # node 16.18.1
    yarnPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # yarn 1.22.19

    # GRPC dependencies
    protobufPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # protobuf 3.20.3
    protoc-gen-goPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # protoc-gen-go 1.28.1
    bufPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # buf 1.15.1

    # Go dependencies
    patchelfPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # patchelf 0.15.0
    goPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # go 1.20.2
  };

  outputs = { self,
              flake-utils,
              nixpkgs,

              helmPkgs,
              golangci-lintPkgs,
              shellcheckPkgs,
              yamllintPkgs,
              gciPkgs,
              addlicensePkgs,

              gccPkgs,
              libiconvPkgs,
              libbpfPkgs,
              libfido2Pkgs,

              pythonPkgs,
              nodePkgs,
              yarnPkgs,

              protobufPkgs,
              protoc-gen-goPkgs,
              bufPkgs,

              patchelfPkgs,
              goPkgs
     }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let
          # These versions are not available from nixpkgs
          rustVersion = "1.68.0";
          gogoVersion = "v1.3.2";
          helmUnittestVersion = "v1.0.16";
          nodeProtocTsVersion = "5.0.1";
          grpcToolsVersion = "1.12.4";

          # Package aliases to make reusing these packages easier.
          # The individual package names here have been determined by using
          # https://lazamar.co.uk/nix-versions/

          # Wrap helm with the unittest plugin.
          helm = (pkgs.wrapHelm helmPkgs.legacyPackages.${system}.kubernetes-helm {plugins = [helm-unittest];});
          golangci-lint = golangci-lintPkgs.legacyPackages.${system}.golangci-lint;
          shellcheck = shellcheckPkgs.legacyPackages.${system}.shellcheck;
          yamllint = yamllintPkgs.legacyPackages.${system}.yamllint;
          gci = gciPkgs.legacyPackages.${system}.gci;
          addlicense = addlicensePkgs.legacyPackages.${system}.addlicense;

          gcc = gccPkgs.legacyPackages.${system}.gcc-unwrapped;
          libiconv = libiconvPkgs.legacyPackages.${system}.libiconvReal;
          libbpf = libbpfPkgs.legacyPackages.${system}.libbpf;
          libfido2 = libfido2Pkgs.legacyPackages.${system}.libfido2;

          python = pythonPkgs.legacyPackages.${system}.python311;
          node = nodePkgs.legacyPackages.${system}.nodejs-16_x;
          yarn = yarnPkgs.legacyPackages.${system}.yarn;

          protobuf = protobufPkgs.legacyPackages.${system}.protobuf3_20;
          protoc-gen-go = protoc-gen-goPkgs.legacyPackages.${system}.protoc-gen-go;
          buf = bufPkgs.legacyPackages.${system}.buf;

          patchelf = patchelfPkgs.legacyPackages.${system}.patchelf;
          go = goPkgs.legacyPackages.${system}.go_1_20;

          # pkgs is an alias for the nixpkgs at the system level. This will be used
          # for general utilities.
          pkgs = nixpkgs.legacyPackages.${system};

          # Compile protoc-gen-gogo for golang protobuf compilation.
          protoc-gen-gogo = pkgs.stdenv.mkDerivation {
            name = "protoc-gen-gogo";
            src = pkgs.fetchFromGitHub {
              owner = "gogo";
              repo = "protobuf";
              rev = gogoVersion;
              sha256 = "sha256-CoUqgLFnLNCS9OxKFS7XwjE17SlH6iL1Kgv+0uEK2zU=";
            };
            buildInputs = [
              pkgs.cacert
              go
            ];
            buildPhase = ''
              export GOBIN="$out/bin"
              export GOCACHE="$(mktemp -d)"
              make install
              cp -R protobuf "$out/protobuf"
            '';
          };

          # Compile grpc-tools for nodejs protobuf compilation.
          grpc-tools = pkgs.stdenv.mkDerivation {
            name = "grpc-tools";
            dontUnpack = true;
            buildInputs = [
              node
            ];
            buildPhase = ''
              export HOME="$(mktemp -d)"
              export TEMPDIR="$(mktemp -d)"
              npm install --prefix "$TEMPDIR" grpc_tools_node_protoc_ts@${nodeProtocTsVersion} grpc-tools@${grpcToolsVersion}
              mv "$TEMPDIR" "$out"
              mkdir "$out/bin"
              cd "$out/bin"
              ln -s ../node_modules/.bin/* "$out/bin/"
            '';
          };

          # Rust and cargo binaries.
          rust = pkgs.stdenv.mkDerivation {
            name = "rust";
            dontUnpack = true;
            buildInputs = [
              pkgs.cacert
              pkgs.curl
            ];
            buildPhase = ''
              export RUSTUP_HOME="$out"
              export CARGO_HOME="$out"
              curl --proto '=https' --tlsv1.2 -fsSL https://sh.rustup.rs | sh -s -- -y --no-modify-path --default-toolchain "${rustVersion}"
            '';
          };

          # The helm unittest plugin.
          helm-unittest = pkgs.buildGoModule rec {
            name = "helm-unittest";
          
            src = pkgs.fetchFromGitHub {
              owner = "vbehar";
              repo = "helm3-unittest";
              rev = helmUnittestVersion;
              sha256 = "sha256-2UfQimIlA+hb1CpQrWfMh5iBEvgdnrkCGYaTJC3Bzpo=";
            };

            vendorSha256 = null;
          
            postInstall = ''
              install -Dm644 plugin.yaml $out/helm-unittest/plugin.yaml
              mkdir "$out/helm-unittest/bin"
              mv $out/bin/helm3-unittest $out/helm-unittest/bin/unittest
            '';
          
            doCheck = false;
          };

          # These inputs are given to the devShell
          baseInputs = [
              helm
              golangci-lint
              shellcheck
              yamllint
              gci
              addlicense

              gcc
              libiconv
              libfido2
              rust

              python
              node
              yarn

              protobuf
              protoc-gen-go
              protoc-gen-gogo
              grpc-tools
              buf

              go
          ];
          conditionalInputs = if pkgs.stdenv.isLinux then
          [
            libbpf
          ] else [];
        in {
          # All of the Teleport shell dependencies for this system.
          teleportShellDeps = baseInputs ++ conditionalInputs;
        }
      );
}
