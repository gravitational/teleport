/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package incidentio

import "time"

// AlertBody represents an incident.io Alert
type AlertBody struct {
	// Title is the title of the alert.
	Title string `json:"message,omitempty"`
	// Description field of the alert.
	Description string `json:"description,omitempty"`
	// DeduplicationKey is the key to use for deduplication.
	DeduplicationKey string `json:"deduplication_key,omitempty"`
	// Status is the status of the alert.
	Status string `json:"status,omitempty"`
	// Metadata is a map of key-value pairs to use as custom properties of the alert.
	Metadata map[string]string `json:"metadata,omitempty"`
}

type GetScheduleResponse struct {
	Schedule ScheduleResult `json:"schedule"`
}

type ScheduleResult struct {
	Annotations          map[string]string    `json:"annotations"`
	Config               ScheduleConfig       `json:"config"`
	CreatedAt            time.Time            `json:"created_at"`
	CurrentShifts        []CurrentShift       `json:"current_shifts"`
	HolidaysPublicConfig HolidaysPublicConfig `json:"holidays_public_config"`
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Timezone             string               `json:"timezone"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

type ScheduleConfig struct {
	Rotations []Rotation `json:"rotations"`
}

type Rotation struct {
	EffectiveFrom   time.Time         `json:"effective_from"`
	HandoverStartAt time.Time         `json:"handover_start_at"`
	Handovers       []Handover        `json:"handovers"`
	ID              string            `json:"id"`
	Layers          []Layer           `json:"layers"`
	Name            string            `json:"name"`
	Users           []User            `json:"users"`
	WorkingInterval []WorkingInterval `json:"working_interval"`
}

type Handover struct {
	Interval     int    `json:"interval"`
	IntervalType string `json:"interval_type"`
}

type Layer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type User struct {
	Email       string `json:"email"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	SlackUserID string `json:"slack_user_id"`
}

type WorkingInterval struct {
	EndTime   string `json:"end_time"`
	StartTime string `json:"start_time"`
	Weekday   string `json:"weekday"`
}

type CurrentShift struct {
	EndAt       time.Time `json:"end_at"`
	EntryID     string    `json:"entry_id"`
	Fingerprint string    `json:"fingerprint"`
	LayerID     string    `json:"layer_id"`
	RotationID  string    `json:"rotation_id"`
	StartAt     time.Time `json:"start_at"`
	User        *User     `json:"user"`
}

type HolidaysPublicConfig struct {
	CountryCodes []string `json:"country_codes"`
}
