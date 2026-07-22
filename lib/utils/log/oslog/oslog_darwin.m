// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

#include "oslog_darwin.h"

#import <os/log.h>
#import <Foundation/Foundation.h>

void* TELCreateLog(const char *subsystem, const char *category) {
  os_log_t log = os_log_create(subsystem, category);
  // __bridge_retained casts an Obj-C pointer to a Core Foundation pointer and transfers ownership to the callsite.
  // The pointer is cast to void* so that it can be used as unsafe.Pointer in Go code.
  // https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFDesignConcepts/Articles/tollFreeBridgedTypes.html#//apple_ref/doc/uid/TP40010677-SW2
  // https://www.informit.com/articles/article.aspx?p=1745876&seqNum=2
  return (__bridge_retained void*)log;
}

void TELLog(void *log, uint type, const char *message) {
  // __bridge transfers a pointer between Obj-C and Core Foundation with no transfer of ownership.
  // Since the pointer comes from a callsite that owns log and that is going to continue using it,
  // there's no need to transfer ownership.
  os_log_with_type((__bridge os_log_t)log, type, "%{public}s", message);
}
