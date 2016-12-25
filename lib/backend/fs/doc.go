/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

fs package implements backend.Backend interface using a regular
filesystem-based directory. The filesystem needs to be POSIX
compliant and support 'date modified' attribute on files.
*/

//
// filesystem backend is supposed to be used for single-host,
// multip-process teleport deployments (it allows for multi-process locking).
//
// Limitations:
// 	- key names cannot start with '.' (dot)
//

package fs
