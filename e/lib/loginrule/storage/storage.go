package storage

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	loginRuleRangeStart = loginRuleKey("")
	loginRuleRangeEnd   = backend.RangeEnd(loginRuleRangeStart)
)

// GetBackendFunc is a function that returns a [backend.Backend] implementation,
// used so that the storage instance can be created before the backend.
type GetBackendFunc func() backend.Backend

// S implements login rule storage, backed by a [backend.Backend].
type S struct {
	backend backend.Backend
}

// New returns a login rule storage implementation.
func New(backend backend.Backend) *S {
	return &S{
		backend: backend,
	}
}

// CreateLoginRule validates and creates a login rule in the backend if one with
// the same name does not already exist, else it returns an error.
func (s *S) CreateLoginRule(ctx context.Context, rule *loginrulepb.LoginRule) (*loginrulepb.LoginRule, error) {
	item, err := marshalToItem(rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.backend.Create(ctx, *item)
	if trace.IsAlreadyExists(err) {
		return nil, trace.AlreadyExists("login rule %q already exists", rule.GetMetadata().Name)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rule.GetMetadata().SetRevision(lease.Revision)

	return rule, nil
}

// UpsertLoginRule validates and upserts a login rule in the backend.
func (s *S) UpsertLoginRule(ctx context.Context, rule *loginrulepb.LoginRule) (*loginrulepb.LoginRule, error) {
	item, err := marshalToItem(rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := s.backend.Put(ctx, *item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rule.GetMetadata().SetRevision(lease.Revision)

	return rule, nil
}

// GetLoginRule returns a login rule from the backend by name.
func (s *S) GetLoginRule(ctx context.Context, name string) (*loginrulepb.LoginRule, error) {
	item, err := s.backend.Get(ctx, loginRuleKey(name))
	switch {
	case trace.IsNotFound(err):
		return nil, trace.NotFound("login rule %q is not found", name)
	case err != nil:
		return nil, trace.Wrap(err)
	}

	rule, err := unmarshalFromItem(item)
	return rule, trace.Wrap(err)
}

// ListLoginRules is a paginated method to list all login rules. It returns the
// login rules for this page and a page token for the next call.
//
// Use an empty pageToken to start. The returned nextPageToken will be an
// empty string if the call reached the "last" login rule, else it should be
// passed as pageToken for the next call to get the next page of rules.
//
// The requested pageSize will be used as a maximum, but ListLoginRules may
// return fewer items at will. The pagination is only complete once
// nextPageToken is empty.
func (s *S) ListLoginRules(ctx context.Context, requestedPageSize int, pageToken string) (rules []*loginrulepb.LoginRule, nextPageToken string, err error) {
	const maxPageSize = 100
	pageSize := requestedPageSize
	if pageSize <= 0 || pageSize > maxPageSize {
		// A requested page size <= 0 will be treated as "I don't care" from the
		// client so give them the max.
		pageSize = maxPageSize
	}

	startKey := loginRuleRangeStart
	finalRuleFromPreviousCall := pageToken
	if finalRuleFromPreviousCall != "" {
		// If given a page token the range will start from the final rule
		// returned by the previous call.
		//
		// It seems like backend.RangeEnd(finalRuleFromPreviousCall) would work
		// to start "one past" that rule because it just calculates the next
		// possible key, but there is a problem with that. If a rule is named
		// "zz" then the next key is "z{" and that special character will not be
		// allowed by the sanitizer as a start key.
		//
		// So do the next best thing, which is to start at that same key, and
		// skip that item before parsing and returning it. Since that item will
		// probably be skipped (unless it has been deleted since the last call)
		// increment the page size by one.
		startKey = loginRuleKey(finalRuleFromPreviousCall)
		pageSize++
	}

	res, err := s.backend.GetRange(ctx, startKey, loginRuleRangeEnd, pageSize)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	rules = make([]*loginrulepb.LoginRule, 0, len(res.Items))
	for _, item := range res.Items {
		if requestedPageSize > 0 && len(rules) >= requestedPageSize {
			// This check makes sure that requestedPageSize is treated as a max
			// and avoids returning one extra rule if finalRuleFromPreviousCall
			// was not skipped (likely because it no longer exists).
			break
		}
		if ruleNameFromKey(item.Key) == finalRuleFromPreviousCall {
			continue
		}
		rule, err := unmarshalFromItem(&item)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		rules = append(rules, rule)
	}

	// If the backend returned as many items as were asked of it then there are
	// possibly more login rules stored and nextPageToken should be set to the
	// name of the final rule which will be returned. Check len(res.Items)
	// rather than len(rules) which could be off by one.
	if len(res.Items) >= pageSize {
		nextPageToken = rules[len(rules)-1].GetMetadata().Name
	}

	return rules, nextPageToken, nil
}

// DeleteLoginRule deletes the named login rule from the backend, returns
// NotFound error if item does not exist.
func (s *S) DeleteLoginRule(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, loginRuleKey(name))
	if trace.IsNotFound(err) {
		return trace.NotFound("login rule %q is not found", name)
	}
	return trace.Wrap(err)
}

func marshalToItem(rule *loginrulepb.LoginRule) (*backend.Item, error) {
	value, err := utils.FastMarshal(rule)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var expires time.Time
	if rule.GetMetadata().Expires != nil {
		expires = *rule.GetMetadata().Expires
	}

	return &backend.Item{
		Key:     loginRuleKey(rule.GetMetadata().Name),
		Value:   value,
		Expires: expires,
	}, nil
}

func unmarshalFromItem(item *backend.Item) (*loginrulepb.LoginRule, error) {
	var rule loginrulepb.LoginRule
	if err := utils.FastUnmarshal(item.Value, &rule); err != nil {
		return nil, trace.Wrap(err, "error unmarshalling login rule from storage")
	}

	// Sanity check that nothing untoward happened with storage.
	if !rule.HasMetadata() {
		return nil, trace.BadParameter("unable to unmarshal login rule metadata from storage")
	}
	rule.GetMetadata().Revision = item.Revision
	expires := item.Expires
	if !expires.IsZero() {
		rule.GetMetadata().Expires = &expires
	}

	return &rule, nil
}

func ruleNameFromKey(key backend.Key) string {
	return key.TrimPrefix(loginRuleKey("")).String()
}

func loginRuleKey(name string) backend.Key {
	return backend.NewKey("login_rules", name)
}
