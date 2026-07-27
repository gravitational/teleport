use crate::error::{CodecError, InvalidValue};

macro_rules! byte_enum {
      ($name:ident { $($variant:ident = $value:literal),* $(,)? }, $invalid:ident) => {
          #[derive(Debug, Clone, Copy, PartialEq, Eq)]
          #[repr(u8)]
          pub enum $name { $($variant = $value),* }

          impl TryFrom<u8> for $name {
              type Error = CodecError;

              fn try_from(b: u8) -> Result<Self, CodecError> {
                  match b {
                      $( $value => Ok(Self::$variant), )*
                      n => Err(CodecError::Invalid(InvalidValue::$invalid(i32::from(n)))),
                  }
              }
          }

          impl From<$name> for u8 {
              fn from(e: $name) -> Self {
                  match e { $( $name::$variant => $value, )* }
              }
          }
      };
  }

byte_enum!(MouseButtonKind { Left = 0, Middle = 1, Right = 2 },  MouseButton);
byte_enum!(ButtonState     { Up = 0, Down = 1 },                 ButtonState);
byte_enum!(ScrollAxis      { Vertical = 0, Horizontal = 1 },     ScrollAxis);
byte_enum!(Severity        { Info = 0, Warning = 1, Error = 2 }, Severity);
byte_enum!(MfaKind         { U2f = b'u', WebAuthn = b'n' },      MfaKind);

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u32)]
pub enum FileType {
    File = 0,
    Directory = 1,
}

impl TryFrom<u32> for FileType {
    type Error = CodecError;

    fn try_from(v: u32) -> Result<Self, CodecError> {
        match v {
            0 => Ok(Self::File),
            1 => Ok(Self::Directory),
            n => Err(CodecError::Invalid(InvalidValue::FileType(n))),
        }
    }
}

impl From<FileType> for u32 {
    fn from(ft: FileType) -> Self {
        match ft {
            FileType::File => 0,
            FileType::Directory => 1,
        }
    }
}
