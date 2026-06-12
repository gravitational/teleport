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

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/predicate"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
)

func TestInferencePolicyMatchingContext(t *testing.T) {
	t.Parallel()

	user, err := types.NewUser("alice")
	require.NoError(t, err)

	server, err := types.NewServer("server-1", types.KindNode, types.ServerSpecV2{Hostname: "host-1"})
	require.NoError(t, err)

	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{Name: "kube-1"},
		types.KubernetesClusterSpecV3{
			Kubeconfig: []byte("dummy-kubeconfig"),
		},
	)
	require.NoError(t, err)

	database, err := types.NewDatabaseV3(
		types.Metadata{Name: "db-1"},
		types.DatabaseSpecV3{Protocol: types.DatabaseProtocolPostgreSQL, URI: "postgres://dummy"},
	)
	require.NoError(t, err)

	kubeSessionEnd := &events.SessionEnd{
		KubernetesPodMetadata: events.KubernetesPodMetadata{KubernetesPodName: "pod-1"},
	}

	dbSessionEnd := &events.DatabaseSessionEnd{
		DatabaseMetadata: events.DatabaseMetadata{DatabaseProtocol: types.DatabaseProtocolPostgreSQL},
	}

	cases := []struct {
		name       string
		user       types.User
		resource   types.Resource
		session    events.AuditEvent
		expression string
		expected   any
		notFound   bool
	}{
		{
			name:       "known user field",
			user:       user,
			expression: "user.metadata.name",
			expected:   "alice",
		},
		{
			name:       "unknown user field",
			user:       user,
			expression: "user.spec.unknown",
			notFound:   true,
		},
		{
			name:       "known server field",
			resource:   server,
			expression: "resource.spec.hostname",
			expected:   "host-1",
		},
		{
			name:       "unknown server field",
			resource:   server,
			expression: "resource.spec.unknown",
			notFound:   true,
		},
		{
			name:       "known database field",
			resource:   database,
			expression: "resource.spec.protocol",
			expected:   types.DatabaseProtocolPostgreSQL,
		},
		{
			name:       "unknown database field",
			resource:   database,
			expression: "resource.spec.unknown",
			notFound:   true,
		},
		{
			name:       "known Kubernetes cluster field",
			resource:   kubeCluster,
			expression: "resource.spec.kubeconfig",
			expected:   []byte("dummy-kubeconfig"),
		},
		{
			name:       "unknown Kubernetes cluster field",
			resource:   kubeCluster,
			expression: "resource.spec.unknown",
			notFound:   true,
		},
		{
			name:       "known shell session field",
			session:    kubeSessionEnd,
			expression: "session.kubernetes_pod_name",
			expected:   "pod-1",
		},
		{
			name:       "unknown shell session field",
			session:    kubeSessionEnd,
			expression: "session.unknown",
			notFound:   true,
		},
		{
			name:       "known database session field",
			session:    dbSessionEnd,
			expression: "session.db_protocol",
			expected:   types.DatabaseProtocolPostgreSQL,
		},
		{
			name:       "unknown database session field",
			session:    dbSessionEnd,
			expression: "session.unknown",
			notFound:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &InferencePolicyMatchingContext{
				User:     tc.user,
				Resource: tc.resource,
				Session:  tc.session,
			}
			parser, err := NewWhereParser(ctx)
			require.NoError(t, err)

			val, err := parser.Parse(tc.expression)
			if tc.notFound {
				require.Error(t, err)
				assert.True(t, trace.IsNotFound(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, val)
			}
		})
	}
}

func TestInferencePolicyMatchingContext_MixedTypeBooleanExpressions(t *testing.T) {
	t.Parallel()

	server, err := types.NewServer("server-1", types.KindNode, types.ServerSpecV2{Hostname: "host-1"})
	require.NoError(t, err)

	kubeSessionEnd := &events.SessionEnd{
		KubernetesPodMetadata: events.KubernetesPodMetadata{KubernetesPodName: "pod-1"},
	}

	cases := []struct {
		name      string
		predicate string
	}{
		{
			name:      "mixing server and database predicates",
			predicate: `resource.spec.protocol == "postgres" || resource.spec.hostname == "host-1"`,
		},
		{
			name:      "mixing Kubernetes and database session predicates",
			predicate: `session.db_protocol == "postgres" || session.kubernetes_pod_name == "pod-1"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &InferencePolicyMatchingContext{
				Resource: server,
				Session:  kubeSessionEnd,
			}
			parser, err := NewWhereParser(ctx)
			require.NoError(t, err)

			parseResult, err := parser.Parse(tc.predicate)
			require.NoError(t, err)
			pred, ok := parseResult.(predicate.BoolPredicate)
			require.True(t, ok, "expected BoolPredicate, got %T", parseResult)
			assert.True(t, pred())
		})
	}
}

func TestValidateInferencePolicy(t *testing.T) {
	t.Parallel()

	allKinds := []string{"ssh", "k8s", "db"}

	cases := []struct {
		name         string
		kinds        []string
		filter       string
		errorMessage string
	}{
		{name: "valid empty filter", kinds: allKinds, filter: ""},
		{name: "valid user filter", kinds: allKinds, filter: `contains(user.spec.roles, "admin")`},
		{name: "valid server filter", kinds: allKinds, filter: `equals(resource.spec.hostname, "node1")`},
		{name: "valid db filter", kinds: allKinds, filter: `equals(resource.spec.protocol, "postgres")`},
		{name: "valid kube filter", kinds: allKinds, filter: `resource.metadata.labels["env"] == "prod"`},
		{name: "valid shell session filter", kinds: allKinds, filter: `contains(session.participants, "joe")`},
		{name: "valid db session filter", kinds: allKinds, filter: `session.db_protocol == "postgres"`},

		{
			name:         "invalid kinds",
			kinds:        nil,
			errorMessage: "spec.kinds are required",
		},
		{
			name:         "invalid filter syntax",
			kinds:        allKinds,
			filter:       "equals(resource.metadata.name, ",
			errorMessage: "spec.filter has to be a valid predicate",
		},
		{
			name:         "invalid user filter field",
			kinds:        allKinds,
			filter:       `user.metadata.foo == "bar"`,
			errorMessage: "field name foo is not found",
		},
		{
			name:         "invalid resource filter field",
			kinds:        allKinds,
			filter:       `resource.spec.foo == "bar"`,
			errorMessage: "field name spec.foo is not found",
		},
		{
			name:         "invalid session filter field",
			kinds:        allKinds,
			filter:       `session.foo == "bar"`,
			errorMessage: "field name foo is not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := apisummarizer.NewInferencePolicy("my-policy", summarizerv1.InferencePolicySpec_builder{
				Kinds:  tc.kinds,
				Filter: tc.filter,
				Model:  "my-model",
			}.Build())
			err := ValidateInferencePolicy(p)
			if tc.errorMessage == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.errorMessage)
			}
		})
	}
}

func newTestClassifier(name string, kinds []string, filter string) *summarizerv1.Classifier {
	return apisummarizer.NewClassifier(name, summarizerv1.ClassifierSpec_builder{
		Kinds:    kinds,
		Filter:   filter,
		Criteria: "sessions that touch production data",
	}.Build())
}

func TestMatchingClassifiers(t *testing.T) {
	t.Parallel()

	user, err := types.NewUser("alice")
	require.NoError(t, err)

	server, err := types.NewServer("server-1", types.KindNode, types.ServerSpecV2{Hostname: "host-1"})
	require.NoError(t, err)
	server.SetStaticLabels(map[string]string{"env": "prod"})

	matchingCtx := &InferencePolicyMatchingContext{
		User:     user,
		Resource: server,
	}

	all := []*summarizerv1.Classifier{
		newTestClassifier("ssh-any", []string{"ssh"}, ""),
		newTestClassifier("ssh-prod", []string{"ssh"}, `equals(resource.metadata.labels["env"], "prod")`),
		newTestClassifier("ssh-dev", []string{"ssh"}, `equals(resource.metadata.labels["env"], "dev")`),
		newTestClassifier("ssh-alice", []string{"ssh"}, `equals(user.metadata.name, "alice")`),
		newTestClassifier("db-any", []string{"db"}, ""),
	}
	classifiers := func(yield func(*summarizerv1.Classifier, error) bool) {
		for _, c := range all {
			if !yield(c, nil) {
				return
			}
		}
	}

	matched, err := MatchingClassifiers(classifiers, types.SSHSessionKind, matchingCtx)
	require.NoError(t, err)
	names := make([]string, 0, len(matched))
	for _, c := range matched {
		names = append(names, c.GetMetadata().GetName())
	}
	assert.Equal(t, []string{"ssh-any", "ssh-prod", "ssh-alice"}, names)

	matched, err = MatchingClassifiers(classifiers, types.DatabaseSessionKind, matchingCtx)
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "db-any", matched[0].GetMetadata().GetName())

	matched, err = MatchingClassifiers(classifiers, types.KubernetesSessionKind, matchingCtx)
	require.NoError(t, err)
	assert.Empty(t, matched)

	iterErr := trace.BadParameter("backend failure")
	failing := func(yield func(*summarizerv1.Classifier, error) bool) {
		yield(nil, iterErr)
	}
	_, err = MatchingClassifiers(failing, types.SSHSessionKind, matchingCtx)
	require.ErrorIs(t, err, iterErr)

	invalid := []*summarizerv1.Classifier{
		newTestClassifier("bad-filter", []string{"ssh"}, "$%^@$"),
	}
	invalidSeq := func(yield func(*summarizerv1.Classifier, error) bool) {
		for _, c := range invalid {
			if !yield(c, nil) {
				return
			}
		}
	}
	_, err = MatchingClassifiers(invalidSeq, types.SSHSessionKind, matchingCtx)
	require.Error(t, err)
}

func TestValidateClassifier(t *testing.T) {
	t.Parallel()

	c := newTestClassifier(
		"my-classifier",
		[]string{"ssh", "k8s", "db"},
		`equals(resource.metadata.labels["env"], "prod") || equals(user.metadata.name, "admin")`,
	)
	require.NoError(t, ValidateClassifier(c))

	// Empty filter should also be valid.
	c.GetSpec().SetFilter("")
	require.NoError(t, ValidateClassifier(c))

	// Filter syntax errors are rejected.
	c.GetSpec().SetFilter("$%^@$")
	err := ValidateClassifier(c)
	require.Error(t, err)
	assert.ErrorContains(t, err, "spec.filter has to be a valid predicate")

	// Errors from the api-level validation propagate.
	c.GetSpec().SetFilter("")
	c.GetSpec().SetCriteria("")
	assert.ErrorContains(t, ValidateClassifier(c), "spec.criteria is required")
}
