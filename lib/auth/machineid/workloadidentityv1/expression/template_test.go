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

	"github.com/stretchr/testify/require"

	traitv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trait/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
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
			tmpl: `/region/{{workload.kubernetes.labels["com.mycloud/region"]}}/service`,
			attrs: &workloadidentityv1.Attrs{
				Workload: &workloadidentityv1.WorkloadAttrs{
					Kubernetes: &workloadidentityv1.WorkloadAttrsKubernetes{
						Labels: map[string]string{
							"com.mycloud/region": "eu",
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
			tmpl:   "look at my mustache :-{{)",
			attrs:  &workloadidentityv1.Attrs{},
			output: "look at my mustache :-{{)",
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
		"user traits": {
			tmpl: `{{user.traits.skill}}`,
			attrs: &workloadidentityv1.Attrs{
				User: &workloadidentityv1.UserAttrs{
					Traits: []*traitv1.Trait{
						{
							Key:    "skill",
							Values: []string{"coffee-drinker"},
						},
					},
				},
			},
			output: "coffee-drinker",
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

func TestTemplate_MultipleTraitValues(t *testing.T) {
	tmpl, err := expression.NewTemplate(`{{user.traits.skills}}`)
	require.NoError(t, err)

	_, err = tmpl.Render(&workloadidentityv1.Attrs{
		User: &workloadidentityv1.UserAttrs{
			Traits: []*traitv1.Trait{
				{
					Key:    "skills",
					Values: []string{"sword-fighting", "sonnet-writing"},
				},
			},
		},
	})
	require.ErrorContains(t, err, "multiple values")
}

func TestTemplate_MissingSubmessage(t *testing.T) {
	tmpl, err := expression.NewTemplate(`{{workload.kubernetes.pod_name}}`)
	require.NoError(t, err)

	_, err = tmpl.Render(&workloadidentityv1.Attrs{
		Workload: &workloadidentityv1.WorkloadAttrs{},
	})
	require.ErrorContains(t, err, "workload.kubernetes is unset")
}

func TestTemplate_MissingMapValue(t *testing.T) {
	tmpl, err := expression.NewTemplate(`{{workload.kubernetes.labels.foo}}`)
	require.NoError(t, err)

	_, err = tmpl.Render(&workloadidentityv1.Attrs{
		Workload: &workloadidentityv1.WorkloadAttrs{
			Kubernetes: &workloadidentityv1.WorkloadAttrsKubernetes{
				Labels: map[string]string{"bar": "baz"},
			},
		},
	})
	require.ErrorContains(t, err, `no value for key: "foo"`)
}

func TestTemplate_MissingTrait(t *testing.T) {
	tmpl, err := expression.NewTemplate(`{{user.traits.foo}}`)
	require.NoError(t, err)

	_, err = tmpl.Render(&workloadidentityv1.Attrs{
		User: &workloadidentityv1.UserAttrs{
			Traits: []*traitv1.Trait{
				{
					Key:    "bar",
					Values: []string{"baz"},
				},
			},
		},
	})
	require.ErrorContains(t, err, `no value for trait: "foo"`)
}

func TestTemplate_UnsetValue(t *testing.T) {
	tmpl, err := expression.NewTemplate(`{{workload.kubernetes.pod_name}}`)
	require.NoError(t, err)

	_, err = tmpl.Render(&workloadidentityv1.Attrs{
		Workload: &workloadidentityv1.WorkloadAttrs{
			Kubernetes: &workloadidentityv1.WorkloadAttrsKubernetes{
				PodName: "",
			},
		},
	})
	require.ErrorContains(t, err, "workload.kubernetes.pod_name is unset")
}
