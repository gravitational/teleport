{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  buildInputs = with pkgs; [
    drone-cli
    zip

    go_1_17
    rustc
    rust-cbindgen
    cargo
  ];
  shellHook = ''
    export PROTOBUF_LOCATION=${pkgs.protobuf}
    export PROTOC=${pkgs.protobuf}/bin/protoc
    export PROTOC_INCLUDE=${pkgs.protobuf}/include
  '';
}
