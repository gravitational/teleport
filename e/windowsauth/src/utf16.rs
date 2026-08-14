use windows::core::{PCWSTR, PWSTR};
use windows::Win32::Security::Authentication::Identity::LSA_UNICODE_STRING;

#[derive(Clone, Debug)]
pub(super) struct UTF16(Vec<u16>);
impl UTF16 {
    pub(crate) fn from(s: &str) -> UTF16 {
        let mut vec: Vec<u16> = s.encode_utf16().collect();
        vec.push(0);
        UTF16(vec)
    }

    pub(crate) fn pcwstr(&self) -> PCWSTR {
        PCWSTR::from_raw(self.0.as_ptr())
    }

    pub(crate) fn pwstr(&mut self) -> PWSTR {
        PWSTR::from_raw(self.0.as_mut_ptr())
    }

    pub(crate) fn lsa_unicode_string(&mut self) -> LSA_UNICODE_STRING {
        LSA_UNICODE_STRING {
            Length: ((self.0.len() - 1) * 2) as u16,
            MaximumLength: (self.0.len() * 2) as u16,
            Buffer: self.pwstr(),
        }
    }
}
