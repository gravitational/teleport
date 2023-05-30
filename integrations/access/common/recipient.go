/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

const (
	// RecipientKindSchedule shows a recipient is a schedule.
	RecipientKindSchedule = "schedule"
)

// RawRecipientsMap is a mapping of roles to recipient(s).
type RawRecipientsMap map[string][]string

// UnmarshalTOML will convert the input into map[string][]string
// The input can be one of the following:
// "key" = "value"
// "key" = ["multiple", "values"]
func (r *RawRecipientsMap) UnmarshalTOML(in interface{}) error {
	*r = make(RawRecipientsMap)

	recipientsMap, ok := in.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected type for recipients %T", in)
	}

	for k, v := range recipientsMap {
		switch val := v.(type) {
		case string:
			(*r)[k] = []string{val}
		case []interface{}:
			for _, str := range val {
				str, ok := str.(string)
				if !ok {
					return fmt.Errorf("unexpected type for recipients value %T", v)
				}
				(*r)[k] = append((*r)[k], str)
			}
		default:
			return fmt.Errorf("unexpected type for recipients value %T", v)
		}
	}

	return nil
}

// GetRawRecipientsFor will return the set of raw recipients given a list of roles and suggested reviewers.
// We create a unique list based on:
// - the list of suggestedReviewers
// - for each role, the list of reviewers
// - if the role doesn't exist in the map (or it's empty), we add the list of recipients for the default role ("*") instead
func (r RawRecipientsMap) GetRawRecipientsFor(roles, suggestedReviewers []string) []string {
	recipients := stringset.New()

	for _, role := range roles {
		roleRecipients := r[role]
		if len(roleRecipients) == 0 {
			roleRecipients = r[types.Wildcard]
		}

		recipients.Add(roleRecipients...)
	}

	recipients.Add(suggestedReviewers...)

	return recipients.ToSlice()
}

// GetAllRawRecipients returns unique set of raw recipients
func (r RawRecipientsMap) GetAllRawRecipients() []string {
	recipients := stringset.New()

	for _, r := range r {
		recipients.Add(r...)
	}

	return recipients.ToSlice()
}

// Recipient is a generic representation of a message recipient. Its nature depends on the messaging service used.
// It can be a user, a public/private channel, or something else. A Recipient should contain enough information to
// identify uniquely where to send a message.
type Recipient struct {
	// Name is the original string that was passed to create the recipient. This can be an id, email, channel name
	// URL, ... This is the user input (through suggested reviewers or plugin configuration)
	Name string
	// ID represents the recipient from the messaging service point of view.
	// e.g. if Name is a Slack user email address, ID will be the Slack user id.
	// This information should be sufficient to send a new message to a recipient.
	ID string
	// Kind is the recipient kind inferred from the Recipient Name. This is a messaging service concept, most common
	// values are "User" or "Channel".
	Kind string
	// Data allows MessagingBot to store required data for the recipient
	Data interface{}
}

// RecipientSet is a Set of Recipient. Recipient items are deduplicated based on Recipient.ID
type RecipientSet struct {
	recipients map[string]Recipient
}

// NewRecipientSet returns an initialized RecipientSet
func NewRecipientSet() RecipientSet {
	return RecipientSet{recipients: make(map[string]Recipient)}
}

// Add adds an item to an existing RecipientSet. If an item with the same Recipient.ID already exists it is overridden.
func (s *RecipientSet) Add(recipient Recipient) {
	s.recipients[recipient.ID] = recipient
}

// Contains checks if the RecipientSet contains a Recipient for a given recipientID.
func (s *RecipientSet) Contains(recipientID string) bool {
	_, isPresent := s.recipients[recipientID]
	return isPresent
}

// ToSlice returns a Recipient slice from a RecipientSet. Items are copied but not deep-copied.
func (s *RecipientSet) ToSlice() []Recipient {
	recipientSlice := make([]Recipient, 0, len(s.recipients))
	for _, recipient := range s.recipients {
		recipientSlice = append(recipientSlice, recipient)
	}
	return recipientSlice
}
