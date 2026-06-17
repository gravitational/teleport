package summarizerv1

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestService_Classifier_CRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	user := createTestUserWithClassifier(t, srv, "classifier-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	classifier := newTestClassifier("test-classifier-1")

	// Create.
	created, err := sclt.CreateClassifier(ctx, summarizerv1pb.CreateClassifierRequest_builder{
		Classifier: classifier,
	}.Build())
	require.NoError(t, err)
	assert.NotEmpty(t, created.GetClassifier().GetMetadata().GetRevision())
	assertResourceEquals(t, classifier, created.GetClassifier())

	// Creating a duplicate fails.
	_, err = sclt.CreateClassifier(ctx, summarizerv1pb.CreateClassifierRequest_builder{
		Classifier: classifier,
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAlreadyExists(err))

	// Get.
	fetched, err := sclt.GetClassifier(ctx, summarizerv1pb.GetClassifierRequest_builder{
		Name: "test-classifier-1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, created.GetClassifier(), fetched.GetClassifier())

	// Getting a nonexistent classifier fails.
	_, err = sclt.GetClassifier(ctx, summarizerv1pb.GetClassifierRequest_builder{
		Name: "no-such-classifier",
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err))

	// Update.
	updated := created.GetClassifier()
	updated.GetSpec().SetCriteria("sessions that delete production data")
	updateResp, err := sclt.UpdateClassifier(ctx, summarizerv1pb.UpdateClassifierRequest_builder{
		Classifier: updated,
	}.Build())
	require.NoError(t, err)
	assert.Equal(
		t,
		"sessions that delete production data",
		updateResp.GetClassifier().GetSpec().GetCriteria(),
	)

	// The update is persisted; re-fetching returns the full updated resource.
	fetchedUpdate, err := sclt.GetClassifier(ctx, summarizerv1pb.GetClassifierRequest_builder{
		Name: "test-classifier-1",
	}.Build())
	require.NoError(t, err)
	assertResourceEquals(t, updateResp.GetClassifier(), fetchedUpdate.GetClassifier())

	// Updating a nonexistent classifier fails. The update is conditional on the
	// resource revision, and a classifier that does not exist has no matching
	// revision, so the backend reports a compare failure.
	_, err = sclt.UpdateClassifier(ctx, summarizerv1pb.UpdateClassifierRequest_builder{
		Classifier: newTestClassifier("no-such-classifier"),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsCompareFailed(err), "expected CompareFailed, got %v", err)

	// Upsert over an existing resource.
	upserted := updateResp.GetClassifier()
	upserted.GetSpec().SetCriteria("sessions that modify production data")
	upsertResp, err := sclt.UpsertClassifier(ctx, summarizerv1pb.UpsertClassifierRequest_builder{
		Classifier: upserted,
	}.Build())
	require.NoError(t, err)
	assert.Equal(
		t,
		"sessions that modify production data",
		upsertResp.GetClassifier().GetSpec().GetCriteria(),
	)

	// Upsert a new resource.
	_, err = sclt.UpsertClassifier(ctx, summarizerv1pb.UpsertClassifierRequest_builder{
		Classifier: newTestClassifier("test-classifier-2"),
	}.Build())
	require.NoError(t, err)

	// List.
	listResp, err := sclt.ListClassifiers(ctx, summarizerv1pb.ListClassifiersRequest_builder{}.Build())
	require.NoError(t, err)
	assert.Len(t, listResp.GetClassifiers(), 2)
	assert.Empty(t, listResp.GetNextPageToken())

	// List with pagination. The two single-item pages together must cover both
	// distinct classifiers exactly once.
	var paginated []string
	listResp, err = sclt.ListClassifiers(ctx, summarizerv1pb.ListClassifiersRequest_builder{
		PageSize: 1,
	}.Build())
	require.NoError(t, err)
	require.Len(t, listResp.GetClassifiers(), 1)
	require.NotEmpty(t, listResp.GetNextPageToken())
	paginated = append(paginated, listResp.GetClassifiers()[0].GetMetadata().GetName())

	listResp, err = sclt.ListClassifiers(ctx, summarizerv1pb.ListClassifiersRequest_builder{
		PageSize:  1,
		PageToken: listResp.GetNextPageToken(),
	}.Build())
	require.NoError(t, err)
	require.Len(t, listResp.GetClassifiers(), 1)
	assert.Empty(t, listResp.GetNextPageToken())
	paginated = append(paginated, listResp.GetClassifiers()[0].GetMetadata().GetName())

	assert.ElementsMatch(t, []string{"test-classifier-1", "test-classifier-2"}, paginated)

	// Delete.
	_, err = sclt.DeleteClassifier(ctx, summarizerv1pb.DeleteClassifierRequest_builder{
		Name: "test-classifier-1",
	}.Build())
	require.NoError(t, err)

	_, err = sclt.GetClassifier(ctx, summarizerv1pb.GetClassifierRequest_builder{
		Name: "test-classifier-1",
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err))

	// Deleting a nonexistent classifier fails.
	_, err = sclt.DeleteClassifier(ctx, summarizerv1pb.DeleteClassifierRequest_builder{
		Name: "test-classifier-1",
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsNotFound(err))
}

func TestService_Classifier_AccessDenied(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	srv := newTestTLSServer(t)
	// createTestUser grants access to the other summarizer resources, but not
	// to classifiers.
	user := createTestUser(t, srv, "no-classifier-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	_, err = sclt.CreateClassifier(ctx, summarizerv1pb.CreateClassifierRequest_builder{
		Classifier: newTestClassifier("test-classifier"),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err))

	_, err = sclt.GetClassifier(ctx, summarizerv1pb.GetClassifierRequest_builder{
		Name: "test-classifier",
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err))

	_, err = sclt.UpdateClassifier(ctx, summarizerv1pb.UpdateClassifierRequest_builder{
		Classifier: newTestClassifier("test-classifier"),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err))

	_, err = sclt.UpsertClassifier(ctx, summarizerv1pb.UpsertClassifierRequest_builder{
		Classifier: newTestClassifier("test-classifier"),
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err))

	_, err = sclt.DeleteClassifier(ctx, summarizerv1pb.DeleteClassifierRequest_builder{
		Name: "test-classifier",
	}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err))

	_, err = sclt.ListClassifiers(ctx, summarizerv1pb.ListClassifiersRequest_builder{}.Build())
	require.Error(t, err)
	assert.True(t, trace.IsAccessDenied(err))
}

func TestService_Classifier_AuditEvents(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	emitter := &eventstest.MockRecorderEmitter{}
	srv := newTestTLSServer(t, withEmitter(emitter))
	user := createTestUserWithClassifier(t, srv, "classifier-user")

	clt, err := srv.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)
	sclt := clt.SummarizerServiceClient()

	lastEvent := func(t *testing.T, eventType string) apievents.AuditEvent {
		t.Helper()
		var last apievents.AuditEvent
		for _, evt := range emitter.Events() {
			if evt.GetType() == eventType {
				last = evt
			}
		}
		require.NotNil(t, last, "expected at least one %s event", eventType)
		return last
	}

	classifier := newTestClassifier("audit-classifier")

	// Successful create.
	created, err := sclt.CreateClassifier(ctx, summarizerv1pb.CreateClassifierRequest_builder{
		Classifier: classifier,
	}.Build())
	require.NoError(t, err)

	createEvt, ok := lastEvent(t, events.ClassifierCreateEvent).(*apievents.ClassifierCreate)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierCreateCode, createEvt.Code)
	assert.Equal(t, "audit-classifier", createEvt.ResourceMetadata.Name)
	assert.Equal(t, user.GetName(), createEvt.User)
	assert.True(t, createEvt.Status.Success)
	assert.NotNil(t, createEvt.Payload)

	// Failed create (duplicate).
	_, err = sclt.CreateClassifier(ctx, summarizerv1pb.CreateClassifierRequest_builder{
		Classifier: classifier,
	}.Build())
	require.Error(t, err)

	createEvt, ok = lastEvent(t, events.ClassifierCreateEvent).(*apievents.ClassifierCreate)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierCreateFailureCode, createEvt.Code)
	assert.Equal(t, "audit-classifier", createEvt.ResourceMetadata.Name)
	assert.False(t, createEvt.Status.Success)
	assert.NotEmpty(t, createEvt.Status.Error)
	assert.Nil(t, createEvt.Payload, "failure events must not carry a resource payload")

	// Successful update.
	updated := created.GetClassifier()
	updated.GetSpec().SetCriteria("updated criteria")
	_, err = sclt.UpdateClassifier(ctx, summarizerv1pb.UpdateClassifierRequest_builder{
		Classifier: updated,
	}.Build())
	require.NoError(t, err)

	updateEvt, ok := lastEvent(t, events.ClassifierUpdateEvent).(*apievents.ClassifierUpdate)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierUpdateCode, updateEvt.Code)
	assert.Equal(t, "audit-classifier", updateEvt.ResourceMetadata.Name)
	assert.True(t, updateEvt.Status.Success)

	// Failed update (nonexistent resource).
	_, err = sclt.UpdateClassifier(ctx, summarizerv1pb.UpdateClassifierRequest_builder{
		Classifier: newTestClassifier("no-such-classifier"),
	}.Build())
	require.Error(t, err)

	updateEvt, ok = lastEvent(t, events.ClassifierUpdateEvent).(*apievents.ClassifierUpdate)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierUpdateFailureCode, updateEvt.Code)
	assert.Equal(t, "no-such-classifier", updateEvt.ResourceMetadata.Name)
	assert.False(t, updateEvt.Status.Success)
	assert.NotEmpty(t, updateEvt.Status.Error)
	assert.Nil(t, updateEvt.Payload, "failure events must not carry a resource payload")

	// Upserting a new resource emits a create event.
	_, err = sclt.UpsertClassifier(ctx, summarizerv1pb.UpsertClassifierRequest_builder{
		Classifier: newTestClassifier("upsert-classifier"),
	}.Build())
	require.NoError(t, err)

	createEvt, ok = lastEvent(t, events.ClassifierCreateEvent).(*apievents.ClassifierCreate)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierCreateCode, createEvt.Code)
	assert.Equal(t, "upsert-classifier", createEvt.ResourceMetadata.Name)
	assert.True(t, createEvt.Status.Success)

	// Upserting over an existing resource emits an update event.
	_, err = sclt.UpsertClassifier(ctx, summarizerv1pb.UpsertClassifierRequest_builder{
		Classifier: newTestClassifier("upsert-classifier"),
	}.Build())
	require.NoError(t, err)

	updateEvt, ok = lastEvent(t, events.ClassifierUpdateEvent).(*apievents.ClassifierUpdate)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierUpdateCode, updateEvt.Code)
	assert.Equal(t, "upsert-classifier", updateEvt.ResourceMetadata.Name)
	assert.True(t, updateEvt.Status.Success)

	// Successful delete.
	_, err = sclt.DeleteClassifier(ctx, summarizerv1pb.DeleteClassifierRequest_builder{
		Name: "audit-classifier",
	}.Build())
	require.NoError(t, err)

	deleteEvt, ok := lastEvent(t, events.ClassifierDeleteEvent).(*apievents.ClassifierDelete)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierDeleteCode, deleteEvt.Code)
	assert.Equal(t, "audit-classifier", deleteEvt.ResourceMetadata.Name)
	assert.True(t, deleteEvt.Status.Success)

	// Failed delete (nonexistent resource).
	_, err = sclt.DeleteClassifier(ctx, summarizerv1pb.DeleteClassifierRequest_builder{
		Name: "audit-classifier",
	}.Build())
	require.Error(t, err)

	deleteEvt, ok = lastEvent(t, events.ClassifierDeleteEvent).(*apievents.ClassifierDelete)
	require.True(t, ok)
	assert.Equal(t, events.ClassifierDeleteFailureCode, deleteEvt.Code)
	assert.Equal(t, "audit-classifier", deleteEvt.ResourceMetadata.Name)
	assert.False(t, deleteEvt.Status.Success)
	assert.NotEmpty(t, deleteEvt.Status.Error)
}

// newTestClassifier creates a test Classifier.
func newTestClassifier(name string) *summarizerv1pb.Classifier {
	return summarizer.NewClassifier(name, summarizerv1pb.ClassifierSpec_builder{
		Kinds:    []string{string(types.SSHSessionKind)},
		Criteria: "sessions that touch production data",
	}.Build())
}

// createTestUserWithClassifier creates a user that has full access to classifier resources.
func createTestUserWithClassifier(
	t *testing.T, srv *authtest.TLSServer, name string,
) types.User {
	user, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		name,
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindClassifier},
				Verbs:     []string{types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete, types.VerbList},
			},
		},
	)
	require.NoError(t, err)
	return user
}
