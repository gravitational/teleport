extern crate pam;
extern crate reqwest;
mod validate;

use crate::validate::validate;
use pam::constants::{PamFlag, PamResultCode, PAM_PROMPT_ECHO_OFF};
use pam::conv::Conv;
use pam::module::{PamHandle, PamHooks};
use pam::pam_try;
use std::collections::HashMap;
use std::ffi::CStr;

struct PamHttp;
pam::pam_hooks!(PamHttp);

impl PamHooks for PamHttp {
    fn acct_mgmt(_pamh: &mut PamHandle, _args: Vec<&CStr>, _flags: PamFlag) -> PamResultCode {
        println!("account management");
        PamResultCode::PAM_SUCCESS
    }

    // This function performs the task of authenticating the user.
    fn sm_authenticate(pamh: &mut PamHandle, args: Vec<&CStr>, _flags: PamFlag) -> PamResultCode {
        println!("Let's validate JWT2");

        let args: Vec<_> = args.iter().map(|s| s.to_string_lossy()).collect();
        println!("Let's validate JWT3");
        let args: HashMap<&str, &str> = args
            .iter()
            .map(|s| {
                let mut parts = s.splitn(2, '=');
                (parts.next().unwrap(), parts.next().unwrap_or(""))
            })
            .collect();

        println!("q");

        let user = match (pamh.get_user(None)) {
            Ok(t) => {
                println!("got user {t}");
                t
            }
            Err(e) => {
                println!("got error {e:?}");
                return e;
            }
        };

        println!("user :{user}");

        let conv = match pamh.get_item::<Conv>() {
            Ok(Some(conv)) => conv,
            Ok(None) => {
                println!("no conv!");
                unreachable!("No conv available");
            }
            Err(err) => {
                println!("Couldn't get pam_conv");
                return err;
            }
        };

        println!("after conv");

        let password = pam_try!(conv.send(PAM_PROMPT_ECHO_OFF, "JWT Token:"));
        println!("1");
        let password = match password {
            Some(password) => {
                println!("2");
                pam_try!(password.to_str(), PamResultCode::PAM_AUTH_ERR)
            }
            None => {
                println!("Couldn't get password");
                return PamResultCode::PAM_AUTH_ERR;
            }
        };
        println!("Got a password {password:?}");

        match validate(
            &user,
            password,
            args.get("url"),
            args.get("file"),
            args.get("insecure_skip_verify") == Some(&"true"),
        ) {
            Ok(_) => PamResultCode::PAM_SUCCESS,
            Err(e) => {
                println!("Couldn't validate JWT: {e}");
                PamResultCode::PAM_AUTH_ERR
            }
        }
    }

    fn sm_setcred(_pamh: &mut PamHandle, _args: Vec<&CStr>, _flags: PamFlag) -> PamResultCode {
        println!("set credentials");
        PamResultCode::PAM_SUCCESS
    }
}
