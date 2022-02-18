{ pkgs ? import <nixpkgs> { }
, pkgsUnstable ? import (builtins.fetchTarball "https://github.com/NixOS/nixpkgs/archive/48d63e924a2666baf37f4f14a18f19347fbd54a2.tar.gz") { }
, buildGoModule ? pkgs.buildGoModule
}:
let tdr = buildGoModule rec {
  pname = "tdr";
  version = "0.0.0-${src.rev}";
  # https://unix.stackexchange.com/questions/557977
  src = builtins.fetchGit {
    url = "git@github.com:gravitational/tdr.git";
    rev = "5a17e035aec3f014553290fbd295dcdfeef8179c";
  };
  vendorSha256 = "07cbhyisv510jyqp4srcc5256xzclcfnalfn3h9a34x9j8qa70ph";
};
in
pkgs.mkShell {
  buildInputs = with pkgs; [
    drone-cli
    pkgsUnstable.teleport # drone app is not listed when using tsh v7
    tdr
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
