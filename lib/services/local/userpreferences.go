/*
 *
 * Copyright 2023 Gravitational, Inc.
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
 *
 */

package local

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	userpreferencesv1 "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/lib/backend"
)

// UserPreferencesService is responsible for managing a user's preferences.
type UserPreferencesService struct {
	backend.Backend
}

func DefaultUserPreferences() *userpreferencesv1.UserPreferences {
	return &userpreferencesv1.UserPreferences{
		Assist: &userpreferencesv1.AssistUserPreferences{
			PreferredLogins: []string{},
			ViewMode:        userpreferencesv1.AssistViewMode_ASSIST_VIEW_MODE_DOCKED,
		},
		Theme: userpreferencesv1.Theme_THEME_LIGHT,
		Onboard: &userpreferencesv1.OnboardUserPreferences{
			PreferredResources: []userpreferencesv1.Resource{},
		},
	}
}

// NewUserPreferencesService returns a new instance of the UserPreferencesService.
func NewUserPreferencesService(backend backend.Backend) *UserPreferencesService {
	return &UserPreferencesService{
		Backend: backend,
	}
}

// GetUserPreferences returns the user preferences for the given user.
func (u *UserPreferencesService) GetUserPreferences(ctx context.Context, req *userpreferencesv1.GetUserPreferencesRequest) (*userpreferencesv1.GetUserPreferencesResponse, error) {
	preferences, err := u.getUserPreferences(ctx, req.Username)
	if err != nil {
		if trace.IsNotFound(err) {
			return &userpreferencesv1.GetUserPreferencesResponse{Preferences: DefaultUserPreferences()}, nil
		}

		return nil, trace.Wrap(err)
	}

	return &userpreferencesv1.GetUserPreferencesResponse{Preferences: preferences}, nil
}

// UpsertUserPreferences creates or updates user preferences for a given username.
func (u *UserPreferencesService) UpsertUserPreferences(ctx context.Context, req *userpreferencesv1.UpsertUserPreferencesRequest) error {
	if req.Username == "" {
		return trace.BadParameter("missing username")
	}
	if err := validatePreferences(req.Preferences); err != nil {
		return trace.Wrap(err)
	}

	preferences, err := u.getUserPreferences(ctx, req.Username)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		preferences = DefaultUserPreferences()
	}

	if err := overwriteValues(preferences, req.Preferences); err != nil {
		return trace.Wrap(err)
	}

	item, err := createBackendItem(req.Username, preferences)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = u.Put(ctx, item)

	return trace.Wrap(err)
}

// getUserPreferences returns the user preferences for the given username.
func (u *UserPreferencesService) getUserPreferences(ctx context.Context, username string) (*userpreferencesv1.UserPreferences, error) {
	existing, err := u.Get(ctx, backendKey(username))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var p userpreferencesv1.UserPreferences
	if err := json.Unmarshal(existing.Value, &p); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := mergeIfUnset(&p, DefaultUserPreferences()); err != nil {
		return nil, trace.Wrap(err)
	}

	return &p, nil
}

// backendKey returns the backend key for the user preferences for the given username.
func backendKey(username string) []byte {
	return backend.Key(userPreferencesPrefix, username)
}

// validatePreferences validates the given preferences.
func validatePreferences(preferences *userpreferencesv1.UserPreferences) error {
	if preferences == nil {
		return trace.BadParameter("missing preferences")
	}

	return nil
}

// createBackendItem creates a backend.Item for the given username and user preferences.
func createBackendItem(username string, preferences *userpreferencesv1.UserPreferences) (backend.Item, error) {
	settingsKey := backend.Key(userPreferencesPrefix, username)

	payload, err := json.Marshal(preferences)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	item := backend.Item{
		Key:   settingsKey,
		Value: payload,
	}

	return item, nil
}

// overwriteValues overwrites the values in dst with the values in src.
func overwriteValues(dst, src protoreflect.ProtoMessage) error {
	d := dst.ProtoReflect()
	s := src.ProtoReflect()

	dName := d.Descriptor().FullName().Name()
	sName := s.Descriptor().FullName().Name()
	// If the names don't match, then the types don't match, so we can't overwrite.
	if dName != sName {
		return trace.BadParameter("dst and src must be the same type")
	}

	overwriteValuesRecursive(d, s)

	return nil
}

// overwriteValuesRecursive recursively overwrites the values in dst with the values in src.
// It's a helper function for overwriteValues.
func overwriteValuesRecursive(dst, src protoreflect.Message) {
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Message() != nil:
			overwriteValuesRecursive(dst.Mutable(fd).Message(), src.Get(fd).Message())
		default:
			dst.Set(fd, src.Get(fd))
		}

		return true
	})
}

// mergeIfUnset merges the values in src with the values in dst if the values in dst are unset.
func mergeIfUnset(dst, src proto.Message) error {
	if reflect.TypeOf(src) != reflect.TypeOf(dst) {
		return trace.BadParameter("src and dst must be the same type")
	}

	mergeIfUnsetHelper(dst, src)

	return nil
}

// mergeIfUnsetHelper recursively merges the values in src with the values in dst if the values in dst are unset.
func mergeIfUnsetHelper(dst, src any) {
	srcV := reflect.ValueOf(src).Elem()
	dstV := reflect.ValueOf(dst).Elem()

	for i := 0; i < dstV.NumField(); i++ {
		dstF := dstV.Field(i)
		srcF := srcV.Field(i)

		switch dstF.Kind() {
		case reflect.Ptr:
			if dstF.IsNil() && dstF.CanSet() {
				dstF.Set(srcF)
			} else if dstF.Type().Elem().Kind() == reflect.Struct {
				// Recursively call mergeIfUnset for nested messages.
				mergeIfUnsetHelper(dstF.Interface(), srcF.Interface())
			}
		case reflect.Slice, reflect.Map, reflect.Array:
			if dstF.CanSet() && dstF.Len() == 0 {
				// Copy the slice/map/array from src to dst.
				dstF.Set(srcF)
			}
		case reflect.Struct:
			if dstF.CanInterface() {
				// Recursively call mergeIfUnset for nested messages.
				mergeIfUnsetHelper(dstF.Addr().Interface(), srcF.Addr().Interface())
			}
		default:
			if dstF.CanSet() && dstF.IsZero() {
				dstF.Set(srcF)
			}
		}
	}
}
