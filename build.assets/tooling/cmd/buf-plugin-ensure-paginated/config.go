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
package main

import (
	"buf.build/go/bufplugin/option"
)

// config holds the configuration for the plugin
type config struct {
	prefixes      []string
	sizeNames     []string
	tokenNames    []string
	nextNames     []string
	checkRepeated bool
}

const (
	prefixesKey      = "prefixes"
	sizeNamesKey     = "size_names"
	tokenNamesKey    = "token_names"
	nextNamesKey     = "next_names"
	checkRepeatedKey = "check_repeated"
)

var (
	prefixesDefault   = []string{"List", "Search"}
	sizeNamesDefault  = []string{"page_size"}
	tokenNamesDefault = []string{"page_token"}
	nextNamesDefault  = []string{"next_page_token"}
)

// parseOptions takes buf options and returns configuration for this plugin.
func parseOptions(opts option.Options) (*config, error) {
	prefixes, err := option.GetStringSliceValue(opts, prefixesKey)
	if err != nil {
		return nil, err
	}

	sizeNames, err := option.GetStringSliceValue(opts, sizeNamesKey)
	if err != nil {
		return nil, err
	}
	tokenNames, err := option.GetStringSliceValue(opts, tokenNamesKey)
	if err != nil {
		return nil, err
	}
	nextNames, err := option.GetStringSliceValue(opts, nextNamesKey)
	if err != nil {
		return nil, err
	}

	checkRepeated, err := option.GetBoolValue(opts, checkRepeatedKey)
	if err != nil {
		return nil, err
	}

	if len(prefixes) == 0 {
		prefixes = prefixesDefault
	}
	if len(sizeNames) == 0 {
		sizeNames = sizeNamesDefault
	}
	if len(tokenNames) == 0 {
		tokenNames = tokenNamesDefault
	}
	if len(nextNames) == 0 {
		nextNames = nextNamesDefault
	}

	return &config{
		prefixes:      prefixes,
		sizeNames:     sizeNames,
		tokenNames:    tokenNames,
		nextNames:     nextNames,
		checkRepeated: checkRepeated,
	}, nil

}
