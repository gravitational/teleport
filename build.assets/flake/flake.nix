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

    # Rust and GCC dependencies
    libbpfPkgs.url = "github:nixos/nixpkgs/79b3d4bcae8c7007c9fd51c279a8a67acfa73a2a"; # libbpf 1.0.1
  };

  outputs = { self,
              flake-utils,
              nixpkgs,

              helmPkgs,

              libbpfPkgs,
     }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let
          # These versions are not available from nixpkgs
          golangciLintVersion = "v1.53.2";
          rustVersion = "1.68.0";
          gogoVersion = "v1.3.2";
          helmUnittestVersion = "v1.0.16";
          nodeProtocTsVersion = "5.0.1";
          grpcToolsVersion = "1.12.4";

          # Package aliases to make reusing these packages easier.
          # The individual package names here have been determined by using
          # https://lazamar.co.uk/nix-versions/
          libbpf = libbpfPkgs.legacyPackages.${system}.libbpf;

          # pkgs is an alias for the nixpkgs at the system level. This will be used
          # for general utilities.
          pkgs = nixpkgs.legacyPackages.${system};

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

          # Wrap helm with the unittest plugin.
          helm = (pkgs.wrapHelm helmPkgs.legacyPackages.${system}.kubernetes-helm {plugins = [helm-unittest];});

          # Install golangci-lint
          golangci-lint = pkgs.stdenv.mkDerivation {
            name = "golangci-lint";
            buildInputs = [
              pkgs.cacert
              pkgs.curl
            ];
            dontUnpack = true;
            buildPhase = ''
              curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $out/bin ${golangciLintVersion}
            '';
          };

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
              pkgs.go
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
              pkgs.nodejs-16_x
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

          conditional = if pkgs.stdenv.isLinux then libbpf else pkgs.hello;
        in
        {
          packages = {
            conditional = conditional;
            golangci-lint = golangci-lint;
            grpc-tools = grpc-tools;
            helm = helm;
            protoc-gen-gogo = protoc-gen-gogo;
            rust = rust;
          };
      });
}
