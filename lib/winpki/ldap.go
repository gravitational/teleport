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

package winpki

import (
	"cmp"
	"context"
	"crypto/tls"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/dns"
	libset "github.com/gravitational/teleport/lib/utils/set"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

const (
	// ldapDialTimeout is the timeout for dialing the LDAP server
	// when making an initial connection
	ldapDialTimeout = 30 * time.Second

	// ldapRequestTimeout is the timeout for making LDAP requests.
	// It is larger than the dial timeout because LDAP queries in large
	// Active Directory environments may take longer to complete.
	ldapRequestTimeout = 45 * time.Second

	// maxReferralsCount is overall maximum number of referrals that
	// we will attempt to follow when performing a recursive search.
	maxReferralsCount = 10

	// maxSearchDepth is the maximum number of referrals for a given referral
	// chain that we will attempt to follow when performing a recursive search.
	maxSearchDepth = 5

	// maxSearchHosts is the maximum number of hosts we will attempt
	// to contact for each referral when performing a recursive search.
	maxSearchHosts = 5
)

// LocateServer contains parameters for locating LDAP servers
// from the AD Domain
type LocateServer struct {
	// Automatically locate the LDAP server using DNS SRV records.
	// https://ldap.com/dns-srv-records-for-ldap/
	Enabled bool
	// Use LDAP site to locate servers from a specific logical site.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-adts/b645c125-a7da-4097-84a1-2fa7cea07714#gt_8abdc986-5679-42d9-ad76-b11eb5a0daba
	Site string
}

// LDAPConfig contains parameters for connecting to an LDAP server.
type LDAPConfig struct {
	// Addr is the LDAP server address in the form host:port.
	// Standard port is 636 for LDAPS.
	Addr string
	// LocateServer contains parameters for locating LDAP servers from the AD domain.
	LocateServer LocateServer
	// Domain is an Active Directory domain name, like "example.com".
	Domain string
	// Username is an LDAP username, like "EXAMPLE\Administrator", where
	// "EXAMPLE" is the NetBIOS version of Domain.
	Username string
	// SID is the SID for the user specified by Username.
	SID string
	// Logger is the logger for the service.
	Logger *slog.Logger
}

// LDAPClient is an LDAP client designed for Active Directory environments.
// It uses mutual TLS for authentication, and has the ability to discover
// LDAP servers from DNS if an explicit address is not provided.
//
// LDAPClient does not implement any form of credential refresh or certificate
// rotation. It is no longer useful after its certificate expires.
// For this reason, callers are encouraged to create clients on-demand rather
// than keeping them open for long periods of time.
type LDAPClient struct {
	cfg         *LDAPConfig
	conn        *ldap.Conn
	credentials *tls.Config
}

// DialLDAP creates a new LDAP client using the provided TLS config for client credentials.
func DialLDAP(ctx context.Context, cfg *LDAPConfig, credentials *tls.Config) (*LDAPClient, error) {
	conn, err := cfg.createConnection(ctx, credentials)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to LDAP server")
	}

	return &LDAPClient{
		cfg:         cfg,
		conn:        conn,
		credentials: credentials,
	}, nil
}

func (l *LDAPClient) Close() error {
	return l.conn.Close()
}

// DomainDN returns the distinguished name for an Active Directory Domain.
func DomainDN(domain string) string {
	var sb strings.Builder
	parts := strings.SplitSeq(domain, ".")
	for p := range parts {
		if sb.Len() > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("DC=")
		sb.WriteString(p)
	}
	return sb.String()
}

const (
	// AttrObjectSid is the Security Identifier of an LDAP object
	AttrObjectSid = "objectSid"
	// AttrObjectClass is the object class of an LDAP object
	AttrObjectClass = "objectClass"

	// AttrSAMAccountType is the SAM Account type for an LDAP object.
	AttrSAMAccountType = "sAMAccountType"
	// AccountTypeUser is the SAM account type for user accounts.
	// See https://learn.microsoft.com/en-us/windows/win32/adschema/a-samaccounttype
	// (SAM_USER_OBJECT)
	AccountTypeUser = "805306368"

	// AttrSAMAccountName is the SAM Account name of an LDAP object.
	AttrSAMAccountName = "sAMAccountName"

	// AttrUserPrincipalName is the User Principal Name of an LDAP object.
	AttrUserPrincipalName = "userPrincipalName"
)

// searchPageSize is desired page size for LDAP search. In Active Directory the default search size limit is 1000 entries,
// so in most cases the 1000 search page size will result in the optimal amount of requests made to
// LDAP server.
const searchPageSize = 1000

// convertLDAPError attempts to convert LDAP error codes to their
// equivalent trace errors.
func convertLDAPError(err error) error {
	if err == nil {
		return nil
	}

	var ldapErr *ldap.Error
	if errors.As(err, &ldapErr) {
		switch ldapErr.ResultCode {
		case ldap.ErrorNetwork:
			return trace.ConnectionProblem(err, "network error")
		case ldap.LDAPResultOperationsError:
			if strings.Contains(err.Error(), "successful bind must be completed") {
				return trace.NewAggregate(trace.AccessDenied(
					"the LDAP server did not accept Teleport's client certificate, "+
						"has the Teleport CA been imported correctly?"), err)
			}
		case ldap.LDAPResultEntryAlreadyExists:
			return trace.AlreadyExists("LDAP object already exists: %v", err)
		case ldap.LDAPResultConstraintViolation:
			return trace.BadParameter("object constraint violation: %v", err)
		case ldap.LDAPResultInsufficientAccessRights:
			return trace.AccessDenied("insufficient permissions: %v", err)
		}
	}

	return err
}

// GetActiveDirectorySIDAndDN makes an LDAP query to retrieve the security identifier (SID)
// for the specified Active Directory user. It also returns their distinguished name.
// The provided username can be a plain username "bob", or a full UPN like
// "alice@example.com". In cases where the specified username's domain does not match the target
// desktop's domain, this function will attempt to extract and chase referrals from search responses.
func (l *LDAPClient) GetActiveDirectorySIDAndDN(ctx context.Context, username string) (string, string, error) {
	fullUsername := username
	username, domain, _ := strings.Cut(username, "@")
	domain = cmp.Or(domain, l.cfg.Domain)

	// By default, search for the user without following referrals
	searchFn := l.ReadWithFilter
	externalDomain := !strings.EqualFold(domain, l.cfg.Domain)
	if externalDomain {
		// The user does not belong to the Teleport configured domain. We may need
		// to chase referrals to locate their SID.
		searchFn = func(dn, filter string, attrs []string) ([]*ldap.Entry, error) {
			return l.RecursiveReadWithFilter(ctx, dn, filter, attrs)
		}
	}

	queries := []struct {
		name  string
		query func() ([]*ldap.Entry, error)
	}{
		{
			name: "UPN and configured baseDN",
			query: func() ([]*ldap.Entry, error) {
				// User principal name and configured baseDN
				filter := fmt.Sprintf("(%s=%s)", AttrUserPrincipalName, ldap.EscapeFilter(username+"@"+domain))
				return searchFn(DomainDN(l.cfg.Domain), withSAMAccountFilter(filter), []string{AttrObjectSid})
			},
		},
		{
			name: "UPN and derived baseDN",
			query: func() ([]*ldap.Entry, error) {
				// User principal name and baseDN derived from username
				filter := fmt.Sprintf("(%s=%s)", AttrUserPrincipalName, ldap.EscapeFilter(username+"@"+domain))
				return searchFn(DomainDN(domain), withSAMAccountFilter(filter), []string{AttrObjectSid})
			},
		},
		{
			name: "sAMAccountName",
			query: func() ([]*ldap.Entry, error) {
				// sAMAccountName and baseDN derived from username
				// Limited to 20 characters by AD schema
				// https://learn.microsoft.com/en-us/windows/win32/adschema/a-samaccountname
				if len(username) > 20 {
					l.cfg.Logger.WarnContext(ctx, "username used for querying sAMAccountName is longer than 20 characters, results might be invalid", "username", username)
					username = username[:20]
				}
				filter := fmt.Sprintf("(%s=%s)", AttrSAMAccountName, ldap.EscapeFilter(username))
				return searchFn(DomainDN(domain), withSAMAccountFilter(filter), []string{AttrObjectSid})
			},
		},
	}

	var entries []*ldap.Entry
	for _, query := range queries {
		var err error
		entries, err = query.query()
		if err != nil {
			l.cfg.Logger.DebugContext(ctx, "query failed", "query", query.name, "error", err)
			continue
		}

		if len(entries) > 0 {
			l.cfg.Logger.DebugContext(ctx, "query succeeded", "query", query.name, "results", len(entries))
			break
		}
		l.cfg.Logger.DebugContext(ctx, "query succeeded but found no results", "query", query.name)
	}

	if len(entries) == 0 {
		l.cfg.Logger.DebugContext(ctx, "all SID queries exhausted with no results found")
		return "", "", trace.NotFound("could not find Windows account %q", fullUsername)
	}
	if len(entries) > 1 {
		l.cfg.Logger.WarnContext(ctx, "found multiple entries for user, taking the first", "username", fullUsername)
	}
	activeDirectorySID, err := ADSIDStringFromLDAPEntry(entries[0])
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	l.cfg.Logger.DebugContext(ctx, "Found objectSid Windows user", "username", fullUsername)
	distinguishedName := entries[0].DN
	return activeDirectorySID, distinguishedName, nil
}

func withSAMAccountFilter(filter string) string {
	return CombineLDAPFilters([]string{
		fmt.Sprintf("(%s=%s)", AttrSAMAccountType, AccountTypeUser),
		filter,
	})
}

// extractReferrals gathers referrals from the ldapErr.
// If the LDAP server can't provide the information requested, but it has knowledge of another host or domain
// that could fulfill the request, then it will return error like:
// LDAP Result Code 10 "Referral": 0000202B: RefErr: DSID-0310084A
// You then have to parse content of the ber-encoded error to extract the address for the referral
func extractReferrals(ldapErr *ldap.Error) []string {
	if ldapErr == nil {
		return nil
	}

	if ldapErr.ResultCode != ldap.LDAPResultReferral {
		return nil
	}
	searchResultIndex := slices.IndexFunc(ldapErr.Packet.Children, func(packet *ber.Packet) bool {
		return packet.Description == "Search Result Done"
	})
	if searchResultIndex < 0 {
		return nil
	}
	searchResult := ldapErr.Packet.Children[searchResultIndex]

	referralsIndex := slices.IndexFunc(searchResult.Children, func(packet *ber.Packet) bool {
		return packet.Description == "Referral"
	})
	if referralsIndex < 0 {
		return nil
	}
	referrals := searchResult.Children[referralsIndex].Children

	out := make([]string, 0, len(referrals))
	for _, referral := range referrals {
		referralValue, ok := referral.Value.(string)
		// we only support LDAPS connections
		if ok && strings.HasPrefix(referralValue, "ldaps://") {
			out = append(out, referralValue)
		}
	}

	return out
}

// searchResult attempts to emulate the behavior of
// a tagged union. The 'search' function returns a
// list of ldap entries XOR a list of referral strings
// in the success path.
type searchResult interface {
	isSearchResult()
}

type searchResultEntry struct{ entries []*ldap.Entry }
type searchResultReferral struct{ referrals []string }

func (searchResultEntry) isSearchResult()    {}
func (searchResultReferral) isSearchResult() {}

func (l *LDAPClient) search(ctx context.Context, client ldap.Client, searchRequest *ldap.SearchRequest) (searchResult, error) {
	l.cfg.Logger.DebugContext(ctx, "Executing paged query", "filter", searchRequest.Filter, "baseDN", searchRequest.BaseDN)
	res, err := client.SearchWithPaging(searchRequest, searchPageSize)

	switch {
	case err != nil:
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultReferral {
			referrals := extractReferrals(ldapErr)
			if len(referrals) == 0 {
				return nil, trace.Errorf("Failed to extract referrals from ldap error: %v", err)
			}
			l.cfg.Logger.DebugContext(ctx, "Got referrals from paged query error", "referrals", referrals)
			return searchResultReferral{
				referrals: referrals,
			}, nil
		}
		return nil, trace.Wrap(err)
	case len(res.Entries) > 0:
		l.cfg.Logger.DebugContext(ctx, "Got results from paged query", "count", len(res.Entries))
		return searchResultEntry{
			entries: res.Entries,
		}, nil
	case len(res.Referrals) > 0:
		l.cfg.Logger.DebugContext(ctx, "Got referrals from paged query", "referrals", res.Referrals)
		return searchResultReferral{
			referrals: res.Referrals,
		}, nil
	default:
		return nil, trace.NotFound("no results found for LDAP query")
	}
}

type ldapReferral struct {
	// URL scheme (ldaps:// or ldap://)
	scheme string
	// The raw referral received from LDAP server.
	raw string
	// (optional) host is the add:port (port optional) hostname
	host string
	// (optional) base distinguished name specified by the referral.
	baseDN string
	// (optional) scope specified by the referral.
	scope string
	// (optional) comma separated list of attributes specified by the referral.
	attributes string
	// (optional) filter specified by the referral.
	filter string
	// (optional) comma separated list of extensions specified by the referral.
	extensions string
}

// Referral grammar:
// https://datatracker.ietf.org/doc/html/rfc4516#section-2
//
// parses an LDAP referral URL
func parseLDAPReferral(raw string) (ldapReferral, error) {
	const ldapsPrefix = "ldaps://"
	const ldapPrefix = "ldap://"
	var ref string
	var scheme string
	switch {
	case strings.HasPrefix(raw, ldapsPrefix):
		ref = strings.TrimPrefix(raw, ldapsPrefix)
		scheme = ldapsPrefix
	case strings.HasPrefix(raw, ldapPrefix):
		ref = strings.TrimPrefix(raw, ldapPrefix)
		scheme = ldapPrefix
	default:
		return ldapReferral{}, trace.BadParameter("LDAP referral does not have ldaps scheme")
	}

	if len(ref) == 0 {
		// I guess it's technically a valid URL, but useless.
		return ldapReferral{scheme: scheme, raw: raw}, trace.BadParameter("empty referral")
	}

	isValidHostPort := func(host string) error {
		const subDelims = "!$&'()*+,;=:"
		for _, r := range host {
			switch {
			case r >= 'A' && r <= 'Z':
			case r >= 'a' && r <= 'z':
			case r >= '0' && r <= '9':
			case r == '-' || r == '.' || r == '_' || r == '~':
			case strings.ContainsRune(subDelims, r):
			case r == '%':
			default:
				return trace.BadParameter("malformed LDAP URL - host portion contains invalid character %c", r)
			}
		}
		return nil
	}

	hostPort, remainder, found := strings.Cut(ref, "/")
	if err := isValidHostPort(hostPort); err != nil {
		return ldapReferral{}, err
	}

	if !found || remainder == "" {
		return ldapReferral{scheme: scheme, raw: raw, host: hostPort}, nil
	}

	// invariant: remainder is non-empty
	// Since we 'found' the forward slash above, the remainder should contain
	// at least the DN component. Optionally, up to 4 parameters (each prefixed
	// by a '?') may follow the DN. So we'll split 'remainder' on "?"
	// for a maximum of 5 substrings.
	parts := strings.SplitN(remainder, "?", 5)
	// therefore len(parts) > 0 holds
	dn, params := parts[0], parts[1:]
	var err error
	if dn, err = url.QueryUnescape(dn); err != nil {
		return ldapReferral{}, trace.BadParameter("LDAP URL contains malformed DN component")
	}

	referral := ldapReferral{
		scheme: scheme,
		raw:    raw,
		host:   hostPort,
		baseDN: dn,
	}

	assign := []*string{
		&referral.attributes,
		&referral.scope,
		&referral.filter,
		&referral.extensions,
	}

	for idx, param := range params {
		// Params may be percent encoded
		*assign[idx], err = url.QueryUnescape(param)
		if err != nil {
			return ldapReferral{}, trace.Wrap(err)
		}
	}
	return referral, nil
}

// A referral may be a hostname, or a *domain referral* which needs to be
// resolved to a set of hosts. This function attempts an SRV lookup on the url.
// Returns either a slice of the resolved hosts, or upon failure, a slice
// containing only parsed hostname of the raw referral.
func (r *ldapReferral) resolve(ctx context.Context, rslv resolver) []string {
	// The host portion of an LDAP URL is actually optional.
	// Avoid looking up or returning empty hosts
	if r.host == "" {
		return []string{}
	}

	_, port, _ := net.SplitHostPort(r.host)
	if port != "" {
		// Skip the SRV lookup. If the referral URL has
		// an exact port then this isn't a domain referral
		return []string{r.host}
	}

	_, records, err := rslv.LookupSRV(ctx, "ldap", "tcp", r.host)
	if err == nil && len(records) > 0 {
		return libslices.Map(records, func(srv *net.SRV) string {
			return srv.Target
		})
	}
	// No records found or SRV lookup failed. Fall
	// back to trying the host as-is.
	return []string{r.host}
}

type searcher interface {
	search(ctx context.Context, searchRequest *ldap.SearchRequest) (searchResult, error)
	io.Closer
}

type ldapSearcher struct {
	*LDAPClient
}

func (l *ldapSearcher) search(ctx context.Context, searchRequest *ldap.SearchRequest) (searchResult, error) {
	return l.LDAPClient.search(ctx, l.conn, searchRequest)
}

type resolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
}

// Clones 'originalRequest' and overrides the attributes, filter, and scope
// parameters if the 'referral' contains a non-empty replacement.
func newRequestFromReferral(originalRequest *ldap.SearchRequest, referral ldapReferral) *ldap.SearchRequest {
	getScope := func(scope string) int {
		switch scope {
		case "base":
			return ldap.ScopeBaseObject
		case "one":
			return ldap.ScopeSingleLevel
		case "sub":
			return ldap.ScopeWholeSubtree
		default:
			return originalRequest.Scope
		}
	}

	attributes := originalRequest.Attributes
	if referral.attributes != "" {
		attributes = strings.Split(referral.attributes, ",")
	}

	return ldap.NewSearchRequest(
		cmp.Or(referral.baseDN, originalRequest.BaseDN),
		getScope(referral.scope),
		originalRequest.DerefAliases,
		originalRequest.SizeLimit,
		originalRequest.TimeLimit,
		originalRequest.TypesOnly,
		cmp.Or(referral.filter, originalRequest.Filter),
		attributes,
		originalRequest.Controls,
	)
}

// recursiveSearch maintains context for an LDAP
// query that chases referrals.
type recursiveSearch struct {
	// Tracks raw referral strings that the search has encountered.
	referrals libset.Set[string]
	// Limits how far down a given referral chain the search will go.
	maxDepth uint
	// Limits the how many referrals will be attempted overall.
	maxReferrals uint
	// Maximum number of hosts to try per referral.
	maxHosts uint
	// constructor for new searcher (wrapped LDAP client) and associated close function
	// to be called when the searcher is no longer needed.
	newSearcher func(context.Context, string) (searcher, error)
	// A resolver to use for SRV lookups when resolving domain referrals.
	resolver resolver
	logger   *slog.Logger
}

func (r *recursiveSearch) start(ctx context.Context, client searcher, request *ldap.SearchRequest) ([]*ldap.Entry, error) {
	return r.run(ctx, client, request, 0)
}

func (r *recursiveSearch) run(ctx context.Context, client searcher, request *ldap.SearchRequest, depth uint) ([]*ldap.Entry, error) {
	res, err := client.search(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if entryResult, ok := res.(searchResultEntry); ok {
		return entryResult.entries, nil
	}

	referralResult, ok := res.(searchResultReferral)
	if !ok {
		return nil, trace.Errorf("unexpected search result type")
	}

	if depth >= r.maxDepth {
		return nil, trace.LimitExceeded("cannot chase LDAP referral chains beyond the maximum allowed length (%d)", r.maxDepth)
	}

	// Parse LDAP referrals, filter out those that have already been encountered,
	// and add new ones to the referrals set.
	parsedReferrals := libslices.FilterMapUnique(referralResult.referrals, func(ref string) (ldapReferral, bool) {
		defer r.referrals.Add(ref)
		referral, err := parseLDAPReferral(ref)
		if err != nil {
			r.logger.WarnContext(ctx, "Could not parse referral as URL", "referral", ref, "error", err)
			return ldapReferral{}, false
		}

		if !strings.Contains(referral.scheme, "ldaps") {
			r.logger.InfoContext(ctx, "Ignoring referral URL with unexpected scheme", "referral", ref, "scheme", referral.scheme)
			return ldapReferral{}, false
		}

		// Avoid following the same referral twice or exceeding the maximum referral limit
		alreadySeen := r.referrals.Contains(ref)
		maxReferralsReached := len(r.referrals) >= int(r.maxReferrals)
		return referral, !alreadySeen && !maxReferralsReached
	})

referralLoop:
	for _, ref := range parsedReferrals {
		// The referral *may* resolve to multiple hosts
		hosts := ref.resolve(ctx, r.resolver)
		r.logger.InfoContext(ctx, "Chasing referral", "referral", ref.raw, "hosts", hosts)
		for _, host := range hosts[:min(uint(len(hosts)), r.maxHosts)] {
			entries, err := func() ([]*ldap.Entry, error) {
				newClient, err := r.newSearcher(ctx, host)
				if err != nil {
					r.logger.ErrorContext(ctx, "Failed to dial LDAPS server while chasing referral", "error", err, "hostname", host)
					return nil, err
				}
				defer newClient.Close()

				entries, err := r.run(ctx, newClient, newRequestFromReferral(request, ref), depth+1)
				if err != nil && !trace.IsNotFound(err) {
					r.logger.WarnContext(ctx, "Failed to execute LDAPS query while chasing referral", "error", err, "hostname", host)
					return nil, err
				}
				return entries, nil
			}()

			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, err
				}
				continue
			}

			if len(entries) > 0 {
				r.logger.InfoContext(ctx, "Terminating referral chase after successfully finding entries", "referral", ref.raw, "host", host)
				return entries, nil
			}

			// We successfully contacted the referred domain, but it simply didn't
			// have the data we're looking for. We don't need to continue contacting
			// other hosts belonging to the same referral/domain.
			continue referralLoop
		}
	}
	// Referral chasing complete, but no relevant entries found.
	return nil, nil
}

// RecursiveReadWithFilter follows referrals and executes the query/read recursively by
// following referrals to other domains where the search request should be repeated.
func (l *LDAPClient) RecursiveReadWithFilter(ctx context.Context, dn string, filter string, attrs []string) ([]*ldap.Entry, error) {
	search := recursiveSearch{
		referrals:    libset.New[string](),
		maxDepth:     maxSearchDepth,
		maxHosts:     maxSearchHosts,
		maxReferrals: maxReferralsCount,
		newSearcher: func(ctx context.Context, host string) (searcher, error) {
			serverName, _, err := net.SplitHostPort(host)
			if err != nil {
				serverName = host
			}

			// Clone the existing credentials, so that we can change the 'ServerName' to
			// match the hostname of the new LDAP server that we're about to dial.
			referralCreds := l.credentials.Clone()
			referralCreds.ServerName = serverName
			client, err := DialLDAP(ctx, &LDAPConfig{
				Addr:   host,
				Logger: l.cfg.Logger,
			}, referralCreds)
			return &ldapSearcher{LDAPClient: client}, err
		},
		resolver: net.DefaultResolver,
		logger:   l.cfg.Logger,
	}
	return search.start(ctx, &ldapSearcher{LDAPClient: l}, ldap.NewSearchRequest(
		dn,
		ldap.ScopeWholeSubtree,
		ldap.DerefAlways,
		0,     // no SizeLimit
		0,     // no TimeLimit
		false, // TypesOnly == false, we want attribute values
		filter,
		attrs,
		nil, // no Controls)
	))
}

// ReadWithFilter searches the specified DN (and its children) using the specified LDAP filter.
// See https://ldap.com/ldap-filters/ for more information on LDAP filter syntax.
func (l *LDAPClient) ReadWithFilter(dn string, filter string, attrs []string) ([]*ldap.Entry, error) {
	req := ldap.NewSearchRequest(
		dn,
		ldap.ScopeWholeSubtree,
		ldap.DerefAlways,
		0,     // no SizeLimit
		0,     // no TimeLimit
		false, // TypesOnly == false, we want attribute values
		filter,
		attrs,
		nil, // no Controls
	)

	res, err := l.conn.SearchWithPaging(req, searchPageSize)
	if err != nil {
		return nil, trace.Wrap(convertLDAPError(err), "fetching LDAP object %q with filter %q", dn, filter)
	}

	return res.Entries, nil
}

// Read fetches an LDAP entry at path and its children, if any. Only
// entries with the given class are returned and only with the specified
// attributes.
//
// You can browse LDAP on the Windows host to find the objectClass for a
// specific entry using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (l *LDAPClient) Read(dn string, class string, attrs []string) ([]*ldap.Entry, error) {
	return l.ReadWithFilter(dn, fmt.Sprintf("(%s=%s)", AttrObjectClass, class), attrs)
}

// Create creates an LDAP entry at the given path, with the given class and
// attributes. Note that AD will create a bunch of attributes for each object
// class automatically and you don't need to specify all of them.
//
// You can browse LDAP on the Windows host to find the objectClass and
// attributes for similar entries using ADSIEdit.msc.
// You can find the list of all AD classes at
// https://docs.microsoft.com/en-us/windows/win32/adschema/classes-all
func (l *LDAPClient) Create(dn string, class string, attrs map[string][]string) error {
	req := ldap.NewAddRequest(dn, nil)
	for k, v := range attrs {
		req.Attribute(k, v)
	}
	req.Attribute("objectClass", []string{class})

	if err := l.conn.Add(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "error creating LDAP object %q", dn)
	}
	return nil
}

// CreateContainer creates an LDAP container entry if it doesn't already exist.
func (l *LDAPClient) CreateContainer(ctx context.Context, dn string) error {
	const classContainer = "container"
	err := l.Create(dn, classContainer, nil /* attrs */)
	// Ignore the error if container already exists.
	if trace.IsAlreadyExists(err) {
		return nil
	}

	return trace.Wrap(err)
}

// Update updates an LDAP entry at the given path, replacing the provided
// attributes. For each attribute in replaceAttrs, the value is completely
// replaced, not merged. If you want to modify the value of an existing
// attribute, you should read the existing value first, modify it and provide
// the final combined value in replaceAttrs.
//
// You can browse LDAP on the Windows host to find attributes of existing
// entries using ADSIEdit.msc.
func (l *LDAPClient) Update(ctx context.Context, dn string, replaceAttrs map[string][]string) error {
	req := ldap.NewModifyRequest(dn, nil)
	for k, v := range replaceAttrs {
		req.Replace(k, v)
	}

	if err := l.conn.Modify(req); err != nil {
		return trace.Wrap(convertLDAPError(err), "updating %q", dn)
	}
	return nil
}

// CombineLDAPFilters joins the slice of filters
func CombineLDAPFilters(filters []string) string {
	return "(&" + strings.Join(filters, "") + ")"
}

func crlContainerDN(domain string, caType types.CertAuthType) (string, error) {
	ckn, err := crlKeyName(caType)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf(
		"CN=%s,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,%s",
		ckn, DomainDN(domain),
	), nil
}

// CRLCN computes the common name for a Teleport CRL in Windows environments.
// The issuer SKID is optional, but should generally be set for compatibility
// with clusters having more than one issuer (like those using HSMs).
func CRLCN(issuerCN string, issuerSKID []byte) string {
	name := issuerCN
	if len(issuerSKID) > 0 {
		id := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(issuerSKID)
		name = id + "_" + name
	}
	// The limit on the CN attribute should be 64 characters, but in practice
	// we observe that certutil.exe truncates the CN as soon as it exceeds 51 characters.
	return name[:min(len(name), 51)]
}

// CRLDN computes the distinguished name for a Teleport CRL in Windows environments.
// The issuer SKID is optional, but should generally be set for compatibility
// with clusters having more than one issuer (like those using HSMs).
func CRLDN(issuerCN string, issuerSKID []byte, activeDirectoryDomain string, caType types.CertAuthType) (string, error) {
	containerDN, err := crlContainerDN(activeDirectoryDomain, caType)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("CN=%s,%s", CRLCN(issuerCN, issuerSKID), containerDN), nil
}

// CRLDistributionPoint computes the CRL distribution point for certs issued.
func CRLDistributionPoint(activeDirectoryDomain string, caType types.CertAuthType, issuer *tlsca.CertAuthority, includeSKID bool) (string, error) {
	var issuerSKID []byte
	if includeSKID {
		issuerSKID = issuer.Cert.SubjectKeyId
	}
	crlDN, err := CRLDN(issuer.Cert.Subject.CommonName, issuerSKID, activeDirectoryDomain, caType)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf(
		"ldap:///%s?certificateRevocationList?base?objectClass=cRLDistributionPoint", crlDN,
	), nil
}

// crlKeyName returns the appropriate LDAP key given the CA type.
//
// WindowsCA must use "Teleport" to keep backwards compatibility.
func crlKeyName(caType types.CertAuthType) (string, error) {
	switch caType {
	case types.UserCA:
		// TODO(codingllama): DELETE IN 20.
		//  Once the fallback is removed this shouldn't be needed anymore.
		fallthrough
	case types.WindowsCA:
		return "Teleport", nil
	case types.DatabaseCA, types.DatabaseClientCA:
		return "TeleportDB", nil
	default:
		return "", trace.BadParameter("cannot create CRL name for unexpected CA type: %q", caType)
	}
}

// createConnection dials an LDAP server using the provided TLS config.
// The server is either obtained directly from the configuration or
// discovered via DNS.
func (c *LDAPConfig) createConnection(ctx context.Context, ldapTLSConfig *tls.Config) (*ldap.Conn, error) {
	servers := []string{c.Addr}
	dialer := net.Dialer{Timeout: ldapDialTimeout}

	if c.LocateServer.Enabled {
		// In development environments, the system's default resolver is unlikely to be
		// able to resolve the Active Directory SRV records needed for server location,
		// so we allow overriding the resolver. If the TELEPORT_LDAP_RESOLVER paramater
		// is not set, the default resolver will be used.
		resolver := dns.NewResolver(ctx, os.Getenv("TELEPORT_LDAP_RESOLVER"), c.Logger)

		var err error
		if servers, err = dns.LocateServerBySRV(ctx, c.Domain, c.LocateServer.Site, resolver, "ldap", "636"); err != nil {
			return nil, trace.Wrap(err, "locating LDAP server")
		}
	}

	if len(servers) == 0 {
		return nil, trace.NotFound("no LDAP servers found for domain %q", c.Domain)
	}

	var lastErr error
	for _, server := range servers {
		conn, err := ldap.DialURL(
			"ldaps://"+server,
			ldap.DialWithDialer(&dialer),
			ldap.DialWithTLSConfig(ldapTLSConfig),
		)

		if err == nil {
			c.Logger.DebugContext(ctx, "Connected to LDAP server", "server", server)
			conn.SetTimeout(ldapRequestTimeout)
			return conn, nil
		}
		lastErr = err

		if c.LocateServer.Enabled {
			// If the connection fails and we're using LocateServer, log that a server failed.
			c.Logger.DebugContext(ctx, "Error connecting to LDAP server, trying next available server", "server", server, "error", err)
		}
	}

	return nil, trace.NotFound("no LDAP servers responded successfully for domain %q: %v", c.Domain, lastErr)
}
