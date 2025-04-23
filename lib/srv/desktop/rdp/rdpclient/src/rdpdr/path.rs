// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

use ironrdp_pdu::{pdu_other_err, PduResult};
use std::ffi::CString;

/// WindowsPath is a String that we assume to be in the form
/// of a traditional DOS path:
///
/// https://docs.microsoft.com/en-us/dotnet/standard/io/file-path-formats
///
/// Because RDP device redirection is limited in the paths it uses, we can
/// further assume that it is in one of the following forms:
///
/// r"\Program Files\Custom Utilities\StringFinder.exe": An absolute path from the root of the current drive.
///
/// r"2018\January.xlsx": A relative path to a file in a subdirectory of the current directory.
#[derive(Debug, Clone)]
pub struct WindowsPath {
    pub path: String,
}

impl WindowsPath {
    pub fn len(&self) -> u32 {
        self.path.len() as u32
    }
}

impl From<String> for WindowsPath {
    fn from(path: String) -> WindowsPath {
        Self { path }
    }
}

/// UnixPath is a String that we assume to be in the form of a
/// Unix Path, qualified by the qualifications laid out in RFD 0067
///
/// https://github.com/gravitational/teleport/blob/master/rfd/0067-desktop-access-file-system-sharing.md
#[derive(Debug, Clone)]
pub struct UnixPath {
    pub path: String,
}

impl UnixPath {
    /// This function will create a CString from a UnixPath.
    ///
    /// # Errors
    ///
    /// This function will return an error if the UnixPath contains
    /// any characters that can't be handled by CString::new().
    pub fn to_cstring(&self) -> PduResult<CString> {
        CString::new(self.path.clone()).map_err(|e| {
            pdu_other_err!(
                "",
                source:PathError(format!("Error converting UnixPath to CString: {}", e))
            )
        })
    }

    pub fn len(&self) -> u32 {
        self.path.len() as u32
    }

    pub fn last(&self) -> Option<&str> {
        self.path.split('/').last()
    }
}

impl From<&WindowsPath> for UnixPath {
    fn from(p: &WindowsPath) -> UnixPath {
        Self {
            path: to_unix_path(&p.path),
        }
    }
}

impl From<&str> for UnixPath {
    fn from(p: &str) -> UnixPath {
        Self {
            path: to_unix_path(p),
        }
    }
}

impl From<String> for UnixPath {
    fn from(p: String) -> UnixPath {
        Self {
            path: to_unix_path(&p),
        }
    }
}

impl From<&String> for UnixPath {
    fn from(p: &String) -> UnixPath {
        Self {
            path: to_unix_path(p),
        }
    }
}

/// Converts a String from the type of path that's sent to us by RDP
/// into a unix-style path, as specified in Teleport RFD 0067:
///
/// https://github.com/gravitational/teleport/blob/master/rfd/0067-desktop-access-file-system-sharing.md
fn to_unix_path(rdp_path: &str) -> String {
    // Convert r"\" to "/"
    let mut cleaned = rdp_path.replace('\\', "/");

    // If the string started with r"\", just remove it
    if cleaned.starts_with('/') {
        crop_first_n_letters(&mut cleaned, 1);
    }

    cleaned
}

/// Crops the first n letters off of a String (in-place).
fn crop_first_n_letters(s: &mut String, n: usize) {
    match s.char_indices().nth(n) {
        Some((pos, _)) => {
            s.drain(..pos);
        }
        None => {
            s.clear();
        }
    }
}

#[allow(dead_code)]
#[derive(Debug)]
pub struct PathError(pub String);

impl std::fmt::Display for PathError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:#?}", self)
    }
}

impl std::error::Error for PathError {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_to_unix_path() {
        assert_eq!(to_unix_path(r"\"), "");
        assert_eq!(to_unix_path(r"\desktop.ini"), "desktop.ini");
        assert_eq!(
            to_unix_path(r"\test_directory\desktop.ini"),
            "test_directory/desktop.ini"
        );
    }
}
