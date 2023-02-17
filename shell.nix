{ pkgs ? import <nixpkgs> { }
, buildGoModule ? pkgs.buildGoModule
, fetchFromGitHub ? pkgs.fetchFromGitHub
}:
let
  protoc-gen-go-grpc = buildGoModule rec {
    pname = "protoc-gen-go-grpc";
    version = "1.51.0";
    src = fetchFromGitHub {
      owner = "grpc";
      repo = "grpc-go";
      rev = "v${version}";
      hash = "sha256-7IPF8vtW10IpZdV0bMfygpRhMrtMn5hgD3IS7I8bzKw=";
    };
    sourceRoot = "source/cmd/protoc-gen-go-grpc";
    doCheck = false;
    vendorHash = "sha256-yxOfgTA5IIczehpWMM1kreMqJYKgRT5HEGbJ3SeQ/Lg=";
  };

  protoc-gen-gogofast = buildGoModule rec {
    pname = "protoc-gen-gogofast";
    version = "1.3.2";
    src = fetchFromGitHub {
      owner = "gogo";
      repo = "protobuf";
      rev = "v${version}";
      hash = "sha256-CoUqgLFnLNCS9OxKFS7XwjE17SlH6iL1Kgv+0uEK2zU=";
    };
    doCheck = false;
    subPackages = [ pname ];
    vendorHash = "sha256-nOL2Ulo9VlOHAqJgZuHl7fGjz/WFAaWPdemplbQWcak=";
  };

  tdr = buildGoModule {
    pname = "tdr";
    version = "0.0.0";
    # https://unix.stackexchange.com/questions/557977
    src = builtins.fetchGit {
      url = "git@github.com:gravitational/tdr.git";
      rev = "5a17e035aec3f014553290fbd295dcdfeef8179c";
      ref = "main";
    };
    vendorSha256 = "sha256-KKUE+ZbdZ6x9NIjpWYpS3tG/GTZlI2oNoI6W/FntcuY=";
  };

  teleport-hot-reload = pkgs.writeShellScriptBin "teleport-hot-reload" ''
    ${pkgs.reflex}/bin/reflex -r '\.go$' -s -- bash -c \
      "go build -tags 'webassets_embed' -o build/teleport  -ldflags '-w -s' ./tool/teleport && cd sandbox && ./run.sh"
  '';

in
pkgs.mkShell {
  buildInputs = with pkgs; [
    buf
    drone-cli
    grpc-tools
    kubernetes-helm
    protobuf
    protoc-gen-go
    protoc-gen-go-grpc
    protoc-gen-gogofast
    teleport
    tdr
    zip

    go
    nodejs
    python3
    rustc
    rust-cbindgen
    cargo
    yarn

    teleport-hot-reload
  ];
  shellHook = ''
    export PROTOBUF_LOCATION=${pkgs.protobuf}
    export PROTOC=${pkgs.protobuf}/bin/protoc
    export PROTOC_INCLUDE=${pkgs.protobuf}/include

    # npm install grpc_tools_node_protoc_ts grpc_tools_node_protoc_plugin
    export PATH="$PWD/node_modules/grpc_tools_node_protoc_ts/bin:$PATH"
  '';
}
