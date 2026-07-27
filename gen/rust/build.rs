use std::process::Command;

use prost::Message;
use prost_types::FileDescriptorSet;

fn main() {
    let repo_root = "../..";
    let out_dir = std::env::var("OUT_DIR").unwrap();
    let fds_path = format!("{out_dir}/proto.binpb");

    let paths = [
        "api/proto/teleport/desktop",
        "api/proto/teleport/mfa/v1/challenge.proto",
        "api/proto/teleport/legacy/types/metadata.proto",
        "api/proto/teleport/legacy/types/mfa_device.proto",
        "api/proto/teleport/legacy/types/webauthn/webauthn.proto",
    ];

    println!("cargo:rerun-if-changed=build.rs");
    println!("cargo:rerun-if-changed={repo_root}/buf.yaml");
    println!("cargo:rerun-if-changed={repo_root}/buf.lock");
    for p in &paths {
        println!("cargo:rerun-if-changed={repo_root}/{p}");
    }

    let mut cmd = Command::new("buf");
    cmd.arg("build")
        .arg("--as-file-descriptor-set")
        .arg("-o")
        .arg(&fds_path);
    for p in &paths {
        cmd.arg("--path").arg(p);
    }
    cmd.current_dir(repo_root);

    let status = cmd.status().expect("failed to spawn buf — is it on $PATH?");
    assert!(status.success(), "buf build failed");

    let bytes = std::fs::read(&fds_path).unwrap();
    let fds = FileDescriptorSet::decode(bytes.as_slice()).unwrap();

    prost_build::Config::new()
        .boxed(".teleport.desktop.v1.Envelope.payload.mfa")
        .bytes(["."])
        .compile_fds(fds)
        .unwrap();
}
