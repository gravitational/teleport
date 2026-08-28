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

use bitflags::bitflags;

bitflags! {
    /// DesiredAccess can be interpreted as either
    /// 2.2.13.1.1 File_Pipe_Printer_Access_Mask [MS-SMB2] (https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/77b36d0f-6016-458a-a7a0-0f4a72ae1534)
    /// or
    /// 2.2.13.1.2 Directory_Access_Mask [MS-SMB2] (https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/0a5934b1-80f1-4da0-b1bf-5e021c309b71)
    ///
    /// This implements the combination of the two. For flags where the names and/or functions are distinct between the two,
    /// the names are appended with an "_OR_", and the File_Pipe_Printer_Access_Mask functionality is described on the top line comment,
    /// and the Directory_Access_Mask functionality is described on the bottom (2nd) line comment.
    #[derive(Debug, Clone)]
    pub struct DesiredAccess: u32 {
        /// This value indicates the right to read data from the file or named pipe.
        /// This value indicates the right to enumerate the contents of the directory.
        const FILE_READ_DATA_OR_FILE_LIST_DIRECTORY = 0x00000001;
        /// This value indicates the right to write data into the file or named pipe beyond the end of the file.
        /// This value indicates the right to create a file under the directory.
        const FILE_WRITE_DATA_OR_FILE_ADD_FILE = 0x00000002;
        /// This value indicates the right to append data into the file or named pipe.
        /// This value indicates the right to add a sub-directory under the directory.
        const FILE_APPEND_DATA_OR_FILE_ADD_SUBDIRECTORY = 0x00000004;
        /// This value indicates the right to read the extended attributes of the file or named pipe.
        const FILE_READ_EA = 0x00000008;
        /// This value indicates the right to write or change the extended attributes to the file or named pipe.
        const FILE_WRITE_EA = 0x00000010;
        /// This value indicates the right to traverse this directory if the server enforces traversal checking.
        const FILE_TRAVERSE = 0x00000020;
        /// This value indicates the right to delete entries within a directory.
        const FILE_DELETE_CHILD = 0x00000040;
        /// This value indicates the right to execute the file/directory.
        const FILE_EXECUTE = 0x00000020;
        /// This value indicates the right to read the attributes of the file/directory.
        const FILE_READ_ATTRIBUTES = 0x00000080;
        /// This value indicates the right to change the attributes of the file/directory.
        const FILE_WRITE_ATTRIBUTES = 0x00000100;
        /// This value indicates the right to delete the file/directory.
        const DELETE = 0x00010000;
        /// This value indicates the right to read the security descriptor for the file/directory or named pipe.
        const READ_CONTROL = 0x00020000;
        /// This value indicates the right to change the discretionary access control list (DACL) in the security descriptor for the file/directory or named pipe. For the DACL data pub structure, see ACL in [MS-DTYP].
        const WRITE_DAC = 0x00040000;
        /// This value indicates the right to change the owner in the security descriptor for the file/directory or named pipe.
        const WRITE_OWNER = 0x00080000;
        /// SMB2 clients set this flag to any value. SMB2 servers SHOULD ignore this flag.
        const SYNCHRONIZE = 0x00100000;
        /// This value indicates the right to read or change the system access control list (SACL) in the security descriptor for the file/directory or named pipe. For the SACL data pub structure, see ACL in [MS-DTYP].
        const ACCESS_SYSTEM_SECURITY = 0x01000000;
        /// This value indicates that the client is requesting an open to the file with the highest level of access the client has on this file. If no access is granted for the client on this file, the server MUST fail the open with STATUS_ACCESS_DENIED.
        const MAXIMUM_ALLOWED = 0x02000000;
        /// This value indicates a request for all the access flags that are previously listed except MAXIMUM_ALLOWED and ACCESS_SYSTEM_SECURITY.
        const GENERIC_ALL = 0x10000000;
        /// This value indicates a request for the following combination of access flags listed above: FILE_READ_ATTRIBUTES| FILE_EXECUTE| SYNCHRONIZE| READ_CONTROL.
        const GENERIC_EXECUTE = 0x20000000;
        /// This value indicates a request for the following combination of access flags listed above: FILE_WRITE_DATA| FILE_APPEND_DATA| FILE_WRITE_ATTRIBUTES| FILE_WRITE_EA| SYNCHRONIZE| READ_CONTROL.
        const GENERIC_WRITE = 0x40000000;
        /// This value indicates a request for the following combination of access flags listed above: FILE_READ_DATA| FILE_READ_ATTRIBUTES| FILE_READ_EA| SYNCHRONIZE| READ_CONTROL.
        const GENERIC_READ = 0x80000000;
    }
}

bitflags! {
    /// 2.6 File Attributes [MS-FSCC]
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/ca28ec38-f155-4768-81d6-4bfeb8586fc9
    #[derive(Debug, Clone)]
    pub struct FileAttributes: u32 {
        const FILE_ATTRIBUTE_READONLY = 0x00000001;
        const FILE_ATTRIBUTE_HIDDEN = 0x00000002;
        const FILE_ATTRIBUTE_SYSTEM = 0x00000004;
        const FILE_ATTRIBUTE_DIRECTORY = 0x00000010;
        const FILE_ATTRIBUTE_ARCHIVE = 0x00000020;
        const FILE_ATTRIBUTE_NORMAL = 0x00000080;
        const FILE_ATTRIBUTE_TEMPORARY = 0x00000100;
        const FILE_ATTRIBUTE_SPARSE_FILE = 0x00000200;
        const FILE_ATTRIBUTE_REPARSE_POINT = 0x00000400;
        const FILE_ATTRIBUTE_COMPRESSED = 0x00000800;
        const FILE_ATTRIBUTE_OFFLINE = 0x00001000;
        const FILE_ATTRIBUTE_NOT_CONTENT_INDEXED = 0x00002000;
        const FILE_ATTRIBUTE_ENCRYPTED = 0x00004000;
        const FILE_ATTRIBUTE_INTEGRITY_STREAM = 0x00008000;
        const FILE_ATTRIBUTE_NO_SCRUB_DATA = 0x00020000;
        const FILE_ATTRIBUTE_RECALL_ON_OPEN = 0x00040000;
        const FILE_ATTRIBUTE_PINNED = 0x00080000;
        const FILE_ATTRIBUTE_UNPINNED = 0x00100000;
        const FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS = 0x00400000;
    }
}

bitflags! {
    /// Specifies the sharing mode for the open. If ShareAccess values of FILE_SHARE_READ, FILE_SHARE_WRITE and FILE_SHARE_DELETE are set for a printer file or a named pipe, the server SHOULD<35> ignore these values. The field MUST be pub constructed using a combination of zero or more of the following bit values.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/e8fb45c1-a03d-44ca-b7ae-47385cfd7997
    #[derive(Debug, Clone)]
    pub struct SharedAccess: u32 {
        const FILE_SHARE_READ = 0x00000001;
        const FILE_SHARE_WRITE = 0x00000002;
        const FILE_SHARE_DELETE = 0x00000004;
    }
}

bitflags! {
    /// Defines the action the server MUST take if the file that is specified in the name field already exists. For opening named pipes, this field can be set to any value by the client and MUST be ignored by the server. For other files, this field MUST contain one of the following values.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/e8fb45c1-a03d-44ca-b7ae-47385cfd7997
    /// See https://github.com/FreeRDP/FreeRDP/blob/511444a65e7aa2f537c5e531fa68157a50c1bd4d/channels/drive/client/drive_file.c#L207
    /// for information about how these should be interpreted.
    #[derive(PartialEq, Eq, Debug, Clone)]
    pub struct CreateDisposition: u32 {
        const FILE_SUPERSEDE = 0x00000000;
        const FILE_OPEN = 0x00000001;
        const FILE_CREATE = 0x00000002;
        const FILE_OPEN_IF = 0x00000003;
        const FILE_OVERWRITE = 0x00000004;
        const FILE_OVERWRITE_IF = 0x00000005;
    }
}

bitflags! {
    /// Specifies the options to be applied when creating or opening the file. Combinations of the bit positions listed below are valid, unless otherwise noted. This field MUST be pub constructed using the following values.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/e8fb45c1-a03d-44ca-b7ae-47385cfd7997
    #[derive(Debug, Clone)]
    pub struct CreateOptions: u32 {
        const FILE_DIRECTORY_FILE = 0x00000001;
        const FILE_WRITE_THROUGH = 0x00000002;
        const FILE_SEQUENTIAL_ONLY = 0x00000004;
        const FILE_NO_INTERMEDIATE_BUFFERING = 0x00000008;
        const FILE_SYNCHRONOUS_IO_ALERT = 0x00000010;
        const FILE_SYNCHRONOUS_IO_NONALERT = 0x00000020;
        const FILE_NON_DIRECTORY_FILE = 0x00000040;
        const FILE_COMPLETE_IF_OPLOCKED = 0x00000100;
        const FILE_NO_EA_KNOWLEDGE = 0x00000200;
        const FILE_RANDOM_ACCESS = 0x00000800;
        const FILE_DELETE_ON_CLOSE = 0x00001000;
        const FILE_OPEN_BY_FILE_ID = 0x00002000;
        const FILE_OPEN_FOR_BACKUP_INTENT = 0x00004000;
        const FILE_NO_COMPRESSION = 0x00008000;
        const FILE_OPEN_REMOTE_INSTANCE = 0x00000400;
        const FILE_OPEN_REQUIRING_OPLOCK = 0x00010000;
        const FILE_DISALLOW_EXCLUSIVE = 0x00020000;
        const FILE_RESERVE_OPFILTER = 0x00100000;
        const FILE_OPEN_REPARSE_POINT = 0x00200000;
        const FILE_OPEN_NO_RECALL = 0x00400000;
        const FILE_OPEN_FOR_FREE_SPACE_QUERY = 0x00800000;
    }
}

bitflags! {
    /// An unsigned 8-bit integer. This field indicates the success of the Device Create Request (section 2.2.1.4.1).
    /// The value of the Information field depends on the value of CreateDisposition field in the Device Create Request
    /// (section 2.2.1.4.1). If the IoStatus field is set to 0x00000000, this field MAY be skipped, in which case the
    /// server MUST assume that the Information field is set to 0x00.
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpefs/99e5fca5-b37a-41e4-bc69-8d7da7860f76
    #[derive(Debug)]
    pub struct Information: u8 {
        /// A new file was created.
        const FILE_SUPERSEDED = 0x00000000;
        /// An existing file was opened.
        const FILE_OPENED = 0x00000001;
        /// An existing file was overwritten.
        const FILE_OVERWRITTEN = 0x00000003;
    }
}

bitflags! {
    /// Specifies the types of changes to monitor. It is valid to choose multiple trigger conditions.
    /// In this case, if any condition is met, the client is notified of the change and the CHANGE_NOTIFY operation is completed.
    /// See CompletionFilter at: https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-smb2/598f395a-e7a2-4cc8-afb3-ccb30dd2df7c
    #[derive(Debug)]
    pub struct CompletionFilter: u32 {
        /// The client is notified if a file-name changes.
        const FILE_NOTIFY_CHANGE_FILE_NAME = 0x00000001;
        /// The client is notified if a directory name changes.
        const FILE_NOTIFY_CHANGE_DIR_NAME = 0x00000002;
        /// The client is notified if a file's attributes change. Possible file attribute values are specified in [MS-FSCC] section 2.6.
        const FILE_NOTIFY_CHANGE_ATTRIBUTES = 0x00000004;
        /// The client is notified if a file's size changes.
        const FILE_NOTIFY_CHANGE_SIZE = 0x00000008;
        /// The client is notified if the last write time of a file changes.
        const FILE_NOTIFY_CHANGE_LAST_WRITE = 0x00000010;
        /// The client is notified if the last access time of a file changes.
        const FILE_NOTIFY_CHANGE_LAST_ACCESS = 0x00000020;
        /// The client is notified if the creation time of a file changes.
        const FILE_NOTIFY_CHANGE_CREATION = 0x00000040;
        /// The client is notified if a file's extended attributes (EAs) change.
        const FILE_NOTIFY_CHANGE_EA = 0x00000080;
        /// The client is notified of a file's access control list (ACL) settings change.
        const FILE_NOTIFY_CHANGE_SECURITY = 0x00000100;
        /// The client is notified if a named stream is added to a file.
        const FILE_NOTIFY_CHANGE_STREAM_NAME = 0x00000200;
        /// The client is notified if the size of a named stream is changed.
        const FILE_NOTIFY_CHANGE_STREAM_SIZE = 0x00000400;
        /// The client is notified if a named stream is modified.
        const FILE_NOTIFY_CHANGE_STREAM_WRITE = 0x00000800;
    }
}

bitflags! {
    /// Only defines the subset we require from
    /// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/ebc7e6e5-4650-4e54-b17c-cf60f6fbeeaa
    #[derive(Debug)]
    pub struct FileSystemAttributes: u32 {
        const FILE_CASE_SENSITIVE_SEARCH = 0x00000001;
        const FILE_CASE_PRESERVED_NAMES = 0x00000002;
        const FILE_UNICODE_ON_DISK = 0x00000004;
    }
}
