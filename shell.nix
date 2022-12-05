{ pkgs ? import <nixpkgs> { }
, buildGoModule ? pkgs.buildGoModule
}:
let
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
    drone-cli
    teleport
    tdr
    zip

    go
    rustc
    rust-cbindgen
    cargo

    teleport-hot-reload
  ];
  shellHook = ''
    export PROTOBUF_LOCATION=${pkgs.protobuf}
    export PROTOC=${pkgs.protobuf}/bin/protoc
    export PROTOC_INCLUDE=${pkgs.protobuf}/include
  '';
}
