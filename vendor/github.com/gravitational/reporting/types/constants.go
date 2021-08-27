/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

const (
	// ResourceVersion is the current event resource version
	ResourceVersion = "v2"
	// KindEvent is the event resource kind
	KindEvent = "event"
	// EventTypeServer is the server-related event type
	EventTypeServer = "server"
	// EventTypeUser is the user-related event type
	EventTypeUser = "user"
	// EventActionLogin is the event login action
	EventActionLogin = "login"
	// KindHeartbeat is the heartbeat resource kind
	KindHeartbeat = "heartbeat"
	// NotificationUsage is the usage limit notification type
	NotificationUsage = "usage"
	// NotificationTerms is the terms of service violation notification type
	NotificationTerms = "terms"
	// SeverityInfo is info notification severity
	SeverityInfo = "info"
	// SeverityWarning is warning notification severity
	SeverityWarning = "warning"
	// SeverityError is error notification severity
	SeverityError = "error"
)
