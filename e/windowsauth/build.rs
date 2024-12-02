extern crate embed_resource;

use chrono::prelude::*;
use std::env;
use std::ffi::OsString;

use regex::Regex;

fn main() {
    println!("cargo:rerun-if-env-changed=VERSION");
    let version = env::var("VERSION").unwrap_or("v0.0.0".to_string());

    // this transforms version to something FILEVERSION in *.rc files understands, e.g
    // v12.0.0-alpha1 -> 12,0,0
    // v12.1 -> 12,1
    // v12.0.0.0.0.0 -> 12,0,0
    let regex = Regex::new(r"v([0-9]+(\.[0-9]+){0,2}).*").unwrap();
    let version_comma = match regex.captures(&version) {
        None => "0,0,0",
        Some(captures) => captures.get(1).unwrap().as_str(),
    };
    let version_comma = format!("VERSION_COMMA={}", version_comma.replace('.', ","));
    let version = format!("VERSION={}", version);

    let now: DateTime<Utc> = Utc::now();
    let year = format!("YEAR={}", now.year());

    embed_resource::compile(
        "version.rc",
        [
            OsString::from(version),
            OsString::from(version_comma),
            OsString::from(year),
        ],
    ).manifest_optional().unwrap();
}
