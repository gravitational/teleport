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
# This file contains the dev shell definition for Teleport.
#

{
  description = "Teleport dev shell";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/8ad5e8132c5dcf977e308e7bf5517cc6cc0bf7d8"; # general packages
    flake-utils.url = "github:numtide/flake-utils";
    nix-shell.url = "path:./build.assets/nix-shell";
  };

  outputs = { self,
              nixpkgs,
              flake-utils,
              nix-shell,
     }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          teleportShellDeps = nix-shell.teleportShellDeps.${system};
        in {
          devShells.default = pkgs.mkShell {
            buildInputs = teleportShellDeps;
            shellHook = ''
              export GOMODCACHE="/tmp/nix-shell/gomodcache-$(id -u)"
              export GOCACHE="/tmp/nix-shell/gocache-$(id -u)"
              export GOLANGCI_LINT_CACHE="/tmp/nix-shell/golangci-lint-cache-$(id -u)"
              mkdir -p "$GOMODCACHE"
              mkdir -p "$GOCACHE"
              mkdir -p "$GOLANGCI_LINT_CACHE"
            '';
          };
        }
      );
}
