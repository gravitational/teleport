package awsic

import (
	"context"
	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/services"
)

type icAccountAssertion func(assert.TestingT, *identitycenterv1.Account) bool

func withAccountName(name string) icAccountAssertion {
	return func(t assert.TestingT, acct *identitycenterv1.Account) bool {
		return assert.Equal(t, name, acct.GetSpec().Name)
	}
}

func withAccountLabel(key, value string) icAccountAssertion {
	return func(t assert.TestingT, acct *identitycenterv1.Account) bool {
		labels := acct.GetMetadata().GetLabels()
		actual, ok := labels[key]
		if !assert.True(t, ok, "expected label %q to be present on account %q", key, acct.GetMetadata().GetName()) {
			return false
		}
		return assert.Equal(t, value, actual, "label %q on account %q", key, acct.GetMetadata().GetName())
	}
}

func assertICAccount(ctx context.Context, t assert.TestingT, icAccountSvc services.IdentityCenterAccountGetter, id string, assertions ...icAccountAssertion) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	acct, err := icAccountSvc.GetIdentityCenterAccount(ctx, id)
	if !assert.NoError(t, err, "Failed loading IC Account [%s]", id) {
		return false
	}
	for _, assertion := range assertions {
		if !assertion(t, acct) {
			return false
		}
	}
	return true
}

func requireICAccount(ctx context.Context, t require.TestingT, icAccountSvc services.IdentityCenterAccountGetter, id string, assertions ...icAccountAssertion) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertICAccount(ctx, t, icAccountSvc, id, assertions...) {
		return
	}
	t.FailNow()
}

func assertNoICAccount(ctx context.Context, t assert.TestingT, icAccountSvc services.IdentityCenterAccountGetter, id string) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	_, err := icAccountSvc.GetIdentityCenterAccount(ctx, id)
	return assert.ErrorIs(t, err, os.ErrNotExist)
}

func requireNoICAccount(ctx context.Context, t require.TestingT, icAccountSvc services.IdentityCenterAccountGetter, id string) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if assertNoICAccount(ctx, t, icAccountSvc, id) {
		return
	}
	t.FailNow()
}
