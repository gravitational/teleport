pub use prost;
pub use prost_types;

pub mod teleport {
	pub mod desktop {
		pub mod v1 {
			include!(concat!(env!("OUT_DIR"), "/teleport.desktop.v1.rs"));
		}
	}
	pub mod mfa {
		pub mod v1 {
			include!(concat!(env!("OUT_DIR"), "/teleport.mfa.v1.rs"));
		}
	}
}

pub mod types {
	include!(concat!(env!("OUT_DIR"), "/types.rs"));
}

pub mod webauthn {
	include!(concat!(env!("OUT_DIR"), "/webauthn.rs"));
}
