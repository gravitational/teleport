/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package expression_test

import (
	"testing"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
	"github.com/stretchr/testify/require"
)

func TestTemplate_Success(t *testing.T) {
	testCases := map[string]struct {
		tmpl   string
		attrs  *workloadidentityv1.Attrs
		output string
	}{
		"empty string": {
			tmpl:   "",
			attrs:  &workloadidentityv1.Attrs{},
			output: "",
		},
		"simple interpolation": {
			tmpl: "{{user.name}}",
			attrs: &workloadidentityv1.Attrs{
				User: &workloadidentityv1.UserAttrs{
					Name: "Larry",
				},
			},
			output: "Larry",
		},
		"multiple interpolations": {
			tmpl: "{{user.name}} {{user.bot_name}}",
			attrs: &workloadidentityv1.Attrs{
				User: &workloadidentityv1.UserAttrs{
					Name:    "Larry",
					BotName: "LarryBot",
				},
			},
			output: "Larry LarryBot",
		},
		"map access": {
			tmpl: `/region/{{workload.podman.pod.labels["com.mycloud/region"]}}/service`,
			attrs: &workloadidentityv1.Attrs{
				Workload: &workloadidentityv1.WorkloadAttrs{
					Podman: &workloadidentityv1.WorkloadAttrsPodman{
						Pod: &workloadidentityv1.WorkloadAttrsPodmanPod{
							Labels: map[string]string{
								"com.mycloud/region": "eu",
							},
						},
					},
				},
			},
			output: "/region/eu/service",
		},
		"function calling": {
			tmpl:   `{{strings.upper("hello")}}`,
			attrs:  &workloadidentityv1.Attrs{},
			output: "HELLO",
		},
		"incomplete curly braces": {
			tmpl:   "look at my moustache :-{{)",
			attrs:  &workloadidentityv1.Attrs{},
			output: "look at my moustache :-{{)",
		},
		"empty expression": {
			tmpl:   "{{ }}",
			attrs:  &workloadidentityv1.Attrs{},
			output: "",
		},
		"boolean expression": {
			tmpl:   `{{ "a" == "a" }}`,
			attrs:  &workloadidentityv1.Attrs{},
			output: "true",
		},
		"integer expression": {
			tmpl:   `{{ 1234 }}`,
			attrs:  &workloadidentityv1.Attrs{},
			output: "1234",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			template, err := expression.NewTemplate(tc.tmpl)
			require.NoError(t, err)

			output, err := template.Render(tc.attrs)
			require.NoError(t, err)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestTemplate_ParseError(t *testing.T) {
	testCases := map[string]struct {
		tmpl string
		err  string
	}{
		"non-existent variable": {
			tmpl: `{{foo.bar}}`,
			err:  `unknown identifier: "foo.bar"`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := expression.NewTemplate(tc.tmpl)
			require.ErrorContains(t, err, tc.err)
		})
	}
}
