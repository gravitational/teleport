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
    rust-overlay.url = "github:oxalica/rust-overlay";


    # Linting dependencies
    helmPkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # helm 3.11.1

    # libbpf dependencies.
    libbpfPkgs.url = "github:nixos/nixpkgs/79b3d4bcae8c7007c9fd51c279a8a67acfa73a2a"; # libbpf 1.0.1

    # bats dependencies.
    batsPkgs.url = "github:nixos/nixpkgs/5c1ffb7a9fc96f2d64ed3523c2bdd379bdb7b471"; # bats 1.2.1
  };

  outputs = { self,
              flake-utils,
              nixpkgs,
              rust-overlay,

              helmPkgs,
              libbpfPkgs,
              batsPkgs,
     }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let
          # These versions are not available from nixpkgs
          gogoVersion = "v1.3.2";
          helmUnittestVersion = "v1.0.16";
          nodeProtocTsVersion = "v5.0.1";
          grpcToolsVersion = "1.12.4";
          libpcscliteVersion = "1.9.9-teleport";
          rustVersion = "1.68.0";
          yarnVersion = "1.22.19";

          overlays = [ (import rust-overlay) ];

          # Package aliases to make reusing these packages easier.
          # The individual package names here have been determined by using
          # https://lazamar.co.uk/nix-versions/
          libbpf = libbpfPkgs.legacyPackages.${system}.libbpf;
          bats = batsPkgs.legacyPackages.${system}.bats;

          # pkgs is an alias for the nixpkgs at the system level. This will be used
          # for general utilities.
          pkgs = import nixpkgs {
            inherit system overlays;
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

          # Wrap helm with the unittest plugin.
          helm = (pkgs.wrapHelm helmPkgs.legacyPackages.${system}.kubernetes-helm {plugins = [helm-unittest];});

          libpcscliteAdditionalNativeBuildInputs = if pkgs.stdenv.isDarwin then
            [pkgs.darwin.IOKit] else [];
          libpcscliteAdditionalBuildInputs = if pkgs.stdenv.isLinux then
            [pkgs.libusb1] else [];

          # Compile libpcsclite.
          libpcsclite = pkgs.stdenv.mkDerivation {
            name = "libpcsclite";
            src = pkgs.fetchFromGitHub {
              owner = "gravitational";
              repo = "PCSC";
              rev = libpcscliteVersion;
              sha256 = "sha256-Eig30fj7YlDHe6A/ceJ+KLhzT/ctxb9d4nFnsxk+WsA=";
            };
            nativeBuildInputs = [
              pkgs.autoreconfHook
            ] ++ libpcscliteAdditionalNativeBuildInputs;
            buildInputs = [
              pkgs.autoconf-archive
              pkgs.flex
              pkgs.gcc
              pkgs.pkg-config
            ] ++ libpcscliteAdditionalBuildInputs;
            configurePhase = ''
              ./bootstrap
              ./configure --enable-static --with-pic --disable-libsystemd --with-systemdsystemunitdir=$out --exec-prefix=$out --prefix=$out
            '';
            makeFlags = [
              "CFLAGS=\"-std=gnu99\""
            ];
          };

          # Compile protoc-gen-gogo for golang protobuf compilation.
          protoc-gen-gogo = pkgs.buildGoModule {
            name = "protoc-gen-gogo";
            version = gogoVersion;

            src = pkgs.fetchFromGitHub {
              owner = "gogo";
              repo = "protobuf";
              rev = gogoVersion;
              sha256 = "sha256-CoUqgLFnLNCS9OxKFS7XwjE17SlH6iL1Kgv+0uEK2zU=";
            };

            vendorSha256 = "sha256-nOL2Ulo9VlOHAqJgZuHl7fGjz/WFAaWPdemplbQWcak=";

            buildPhase = ''
              export GOBIN="$out/bin"
              export GOCACHE="$(mktemp -d)"
              make install
              cp -R protobuf "$out/protobuf"
            '';
          };

          node-protoc-ts = pkgs.buildNpmPackage {
            name = "grpc_tools_node_protoc_ts";
            version = nodeProtocTsVersion;

            src = pkgs.fetchFromGitHub {
              owner = "agreatfool";
              repo = "grpc_tools_node_protoc_ts";
              rev = nodeProtocTsVersion;
              sha256 = "sha256-kDrflQVENjOY7ei3+D3Znx4eUDPoja8UGG2Phv1eptA=";
            };

            npmDepsHash = "sha256-fxOyItDkkv5OAmtScD9ykq26Meh6qyZSDmWegeh+GRY=";
          };

          grpc-tools = pkgs.stdenv.mkDerivation rec {
            pname = "grpc-tools";
            version = grpcToolsVersion;
          
            src = pkgs.fetchFromGitHub {
              owner = "grpc";
              repo = "grpc-node";
              rev = "grpc-tools@${grpcToolsVersion}";
              fetchSubmodules = true;
              sha256 = "sha256-708lBIGW5+vvSTrZHl/kc+ck7JKNXElrghIGDrMSyx8=";
            };
          
            sourceRoot = "source/packages/grpc-tools";
          
            nativeBuildInputs = [ pkgs.cmake ];
          
            installPhase = ''
              install -Dm755 -t $out/bin grpc_node_plugin

              cp grpc_node_plugin grpc_tools_node_protoc_plugin
              install -Dm755 -t $out/bin grpc_tools_node_protoc_plugin
              
              install -Dm755 -t $out/bin deps/protobuf/protoc
            '';
          };

          rust = pkgs.rust-bin.stable.${rustVersion}.default;

          # Yarn binary.
          yarn = pkgs.stdenv.mkDerivation {
            name = "yarn";
            src = fetchTarball {
              url = "https://yarnpkg.com/downloads/${yarnVersion}/yarn-v${yarnVersion}.tar.gz";
              sha256 = "sha256:0jl77rl2sidsj3ym637w7g35wnv190l96n050aqlm4pyc6wi8v6p";
            };
            buildInputs = [
              pkgs.nodejs-16_x
            ];
            buildPhase = ''
              mkdir "$out"
              cp -R * "$out"
            '';
          };

          conditional = if pkgs.stdenv.isLinux then pkgs.stdenv.mkDerivation {
            name = "conditional";
            dontUnpack = true;
            dontBuild = true;
            propagatedBuildInputs = [
              bats
              libbpf
            ];
          } else pkgs.hello;
        in
        {
          packages = {
            conditional = conditional;
            node-protoc-ts = node-protoc-ts;
            grpc-tools = grpc-tools;
            helm = helm;
            libpcsclite = libpcsclite;
            protoc-gen-gogo = protoc-gen-gogo;
            rust = rust;
            yarn = yarn;
          };
      });
}
