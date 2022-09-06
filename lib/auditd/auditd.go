//go:build !linux

/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package auditd implements Linux Audit client that allows sending events
// and checking system configuration.
package auditd

// SendEvent is a stub function that is called on macOS. Auditd is implemented in Linux kernel and doesn't
// work on system different from Linux.
func SendEvent(_ EventType, _ ResultType, _ Message) error {
	return nil
}

// IsLoginUIDSet returns always false on non Linux systems.
func IsLoginUIDSet() bool {
	return false
}
