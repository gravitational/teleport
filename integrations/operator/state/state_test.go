package state

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ctest "k8s.io/client-go/testing"
)

// this is a reactor that injects a resource version as the kube backend requires it.
func createSecretAddResourceVersion(t *testing.T, tracker ctest.ObjectTracker) ctest.ReactionFunc {
	return func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(ctest.CreateAction).GetObject()
		secret, ok := obj.(*corev1.Secret)
		assert.True(t, ok)
		secret.ResourceVersion = "123"
		require.NoError(t, tracker.Add(secret))
		return true, secret, nil
	}
}

func TestOperatorID(t *testing.T) {
	const (
		testNamespace   = "test-namespace"
		testRelease     = "test-release"
		stateSecretName = testRelease + "-shared-state"
	)
	t.Setenv("KUBE_NAMESPACE", testNamespace)
	t.Setenv("RELEASE_NAME", testRelease)
	t.Run("no existing state", func(t *testing.T) {
		// Test setup
		kubeClient := fake.NewClientset()
		kubeClient.PrependReactor("create", "secrets", createSecretAddResourceVersion(t, kubeClient.Tracker()))

		state, err := New(t.Context(), kubeClient)
		require.NoError(t, err)

		// Test execution
		id, err := state.OperatorID(t.Context())
		require.NoError(t, err)
		require.NotEmpty(t, id)

		// Test validation
		require.NotEqual(t, uuid.NullUUID{}, id)
		secret, err := kubeClient.CoreV1().Secrets(testNamespace).Get(t.Context(), stateSecretName, v1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, id.String(), string(secret.Data[operatorIDKey]))
	})
	t.Run("existing state", func(t *testing.T) {
		// Test setup
		existingID := uuid.New()
		existingSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				Kind:       "secret",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:            stateSecretName,
				Namespace:       testNamespace,
				ResourceVersion: "123",
			},
			Data: map[string][]byte{
				operatorIDKey: []byte(existingID.String()),
			},
		}
		kubeClient := fake.NewClientset(existingSecret)

		state, err := New(t.Context(), kubeClient)
		require.NoError(t, err)

		// Test execution
		id, err := state.OperatorID(t.Context())
		require.NoError(t, err)
		require.NotEmpty(t, id)

		// Test validation
		require.Equal(t, id, existingID)
	})
	t.Run("existing state with no ID", func(t *testing.T) {
		existingSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				Kind:       "secret",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:            stateSecretName,
				Namespace:       testNamespace,
				ResourceVersion: "123",
			},
			Data: map[string][]byte{
				"other-id": []byte("random-data"),
			},
		}
		kubeClient := fake.NewClientset(existingSecret)

		state, err := New(t.Context(), kubeClient)
		require.NoError(t, err)

		// Test execution
		id, err := state.OperatorID(t.Context())
		require.NoError(t, err)
		require.NotEmpty(t, id)

		// Test validation
		require.NotEqual(t, uuid.NullUUID{}, id)
		secret, err := kubeClient.CoreV1().Secrets(testNamespace).Get(t.Context(), stateSecretName, v1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, id.String(), string(secret.Data[operatorIDKey]))
	})
	t.Run("no existing state, then conflict", func(t *testing.T) {
		// Test setup
		existingID := uuid.New()
		existingSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				Kind:       "secret",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:            stateSecretName,
				Namespace:       testNamespace,
				ResourceVersion: "123",
			},
			Data: map[string][]byte{
				operatorIDKey: []byte(existingID.String()),
			},
		}
		kubeClient := fake.NewClientset(existingSecret)
		var called int
		kubeClient.PrependReactor("get", "secrets", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
			// reactor that fakes a "not found" on the second call (first call is the existence check on state creation)
			called++
			if called == 2 {
				return false, nil, nil
			}
			return true, nil, apierrors.NewNotFound(schema.GroupResource{
				Group:    "",
				Resource: "",
			}, "")
		})

		state, err := New(t.Context(), kubeClient)
		require.NoError(t, err)

		// Test execution
		id, err := state.OperatorID(t.Context())
		require.NoError(t, err)

		// Test validation
		require.Equal(t, 2, called)
		require.Equal(t, id, existingID)
	})
	t.Run("existing state, always conflicts", func(t *testing.T) {
		// TODO implement conflict on update
		existingSecret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				Kind:       "secret",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:            stateSecretName,
				Namespace:       testNamespace,
				ResourceVersion: "123",
			},
			Data: map[string][]byte{},
		}
		kubeClient := fake.NewClientset(existingSecret)
		var called int
		kubeClient.PrependReactor("update", "secrets", func(action ctest.Action) (handled bool, ret runtime.Object, err error) {
			called++
			// reactor that always returns a conflict
			return true, nil, apierrors.NewConflict(schema.GroupResource{
				Group:    "",
				Resource: "",
			}, "", errors.New("conflict"))
		})

		state, err := New(t.Context(), kubeClient)
		require.NoError(t, err)

		// Test execution
		id, err := state.OperatorID(t.Context())
		require.Error(t, err)

		// Test validation
		require.NotZero(t, called)
		require.Equal(t, id, uuid.NullUUID{}.UUID)
	})
}
