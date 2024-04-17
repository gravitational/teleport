package fake

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/jamf"
)

// TokenExpiryPeriod is the expiration period for bearer tokens.
const TokenExpiryPeriod = 30 * time.Minute

// User holds credentials for an API user.
type User struct {
	Username string
	Password string
}

// API is a fake implementation for the Jamf PRO API.
type API struct {
	clock clockwork.Clock

	// mu guards all fields below it
	mu                 sync.Mutex
	users              []*User
	inventory          []*jamf.ComputerInventory
	issuedTokens       map[string]*authToken // key is authToken.Token
	simulatePagingGaps bool
}

// Opts are the creation options for [API].
type Opts struct {
	Clock clockwork.Clock
}

// New creates a new fake Jamf API.
func New(opts *Opts) *API {
	if opts == nil {
		opts = &Opts{}
	}

	clock := opts.Clock
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &API{
		clock:        clock,
		issuedTokens: make(map[string]*authToken),
	}
}

func (a *API) SetUsers(users []*User) {
	a.mu.Lock()
	a.users = users
	a.mu.Unlock()
}

func (a *API) SetInventory(inv []*jamf.ComputerInventory) {
	a.mu.Lock()
	a.inventory = inv
	a.mu.Unlock()
}

// Handler returns the http.Handler that implements the REST API.
// Prefix is the path before "/v1". For example, use "/api" to get paths like
// "/api/v1/auth/token" and "/api/v1/auth/keep-alive".
func (a *API) Handler(prefix string) http.Handler {
	return &rootHandler{
		API:    a,
		prefix: prefix,
	}
}

// SetSimulatePagingGaps enables simulation of paging gaps.
// If set to true, listing devices on Jamf will return incomplete pages on most
// requests.
// Useful to test undue device deletions during inventory syncs.
func (a *API) SetSimulatePagingGaps(b bool) {
	a.mu.Lock()
	a.simulatePagingGaps = b
	a.mu.Unlock()
}

type rootHandler struct {
	*API
	prefix string
}

func (a *rootHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(req.URL.Path, a.prefix) {
		http.NotFound(w, req)
		return
	}

	// Strip prefix from the path, we route from `/v1` onwards.
	path := strings.TrimPrefix(req.URL.Path, a.prefix)

	// The only unauthorized endpoint is POST /v1/auth/token, anything else
	// requires a bearer token.
	if req.Method == http.MethodPost && path == "/v1/auth/token" {
		a.postAuthToken(w, req)
		return
	}

	// Authorize.
	token, ok := a.isAuthorized(req)
	if !ok {
		a.replyError(w, errorResponse{HTTPStatus: 401})
		return
	}
	req = req.WithContext(context.WithValue(req.Context(), authTokenKey{}, token))

	// Route.
	var handler http.HandlerFunc
	switch req.Method {
	case http.MethodGet:
		const computersInventory = "/v1/computers-inventory"
		const computersInventorySlash = computersInventory + "/"

		// GET /v1/computers-inventory
		if path == computersInventory || path == computersInventorySlash {
			handler = a.getComputersInventory
			break // breaks from switch
		}

		// GET /v1/computers-inventory/{id}
		if id := strings.TrimPrefix(path, computersInventorySlash); id != path {
			n, err := strconv.ParseInt(id, 10, 64)
			switch {
			case err != nil && strings.Contains(id, "/"):
				// Not found.
				// Technically requests like '/v1/computers-inventory/99/' do work, but
				// let's not encourage that.
				break // breaks from switch
			case err != nil: // "Regular" parsing errors.
				a.replyError(w, errorResponse{
					HTTPStatus: 400,
					Errors: []*apiError{
						{
							Code:        "INVALID_ID",
							Description: "id field must be string of positive numeric value or -1",
							ID:          id,
							Field:       "arg0",
						},
					},
				})
				return
			case n > math.MaxInt32:
				// Yep, this happens.
				a.replyError(w, errorResponse{
					HTTPStatus: 500,
					Errors:     []*apiError{},
				})
				return
			default:
				handler = a.getComputersInventoryByID(id)
			}
		}

	case http.MethodPost:
		if path == "/v1/auth/keep-alive" {
			handler = a.postAuthKeepAlive
		}
	}
	if handler == nil {
		a.replyError(w, errorResponse{HTTPStatus: 404})
		return
	}

	handler(w, req)
}

// authTokenKey is used to save the current *authToken in the request's context.
type authTokenKey struct{}

func (a *API) isAuthorized(req *http.Request) (*authToken, bool) {
	authzHeader := req.Header.Get("Authorization")
	if authzHeader == "" || !strings.HasPrefix(authzHeader, "Bearer ") {
		return nil, false
	}
	token := strings.TrimPrefix(authzHeader, "Bearer ")
	if token == "" {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Token exists?
	issuedToken, ok := a.issuedTokens[token]
	if !ok {
		return nil, false
	}

	// Token expired?
	now := a.clock.Now().UTC()
	if now.After(issuedToken.Expires) {
		delete(a.issuedTokens, token)
		return nil, false
	}

	return issuedToken, true
}

type errorResponse struct {
	HTTPStatus int         `json:"httpStatus"`
	Errors     []*apiError `json:"errors"`
}

type apiError struct {
	Code        string      `json:"code"`
	Description string      `json:"description"`
	ID          string      `json:"id"`
	Field       interface{} `json:"field"` // Only seen as `null`.
}

type authToken struct {
	jamf.AuthToken
	owner string
}

func (a *API) postAuthToken(w http.ResponseWriter, req *http.Request) {
	user, pass, ok := req.BasicAuth()
	if !ok {
		a.replyError(w, errorResponse{HTTPStatus: 401})
		return
	}

	// Find user.
	match := false
	a.mu.Lock()
	for _, u := range a.users {
		if u.Username == user && u.Password == pass {
			match = true
			break
		}
	}
	a.mu.Unlock()
	if !match {
		a.replyError(w, errorResponse{HTTPStatus: 401})
		return
	}

	token, err := a.newAuthToken(user)
	if err != nil {
		// Error not observed in practice.
		a.replyError(w, errorResponse{
			HTTPStatus: 500,
			Errors:     []*apiError{{Description: err.Error()}},
		})
		return
	}

	// Commit token to memory.
	a.mu.Lock()
	a.issuedTokens[token.Token] = token
	a.mu.Unlock()

	// Reply.
	a.replyJSON(w, 200, token)
}

func (a *API) newAuthToken(owner string) (*authToken, error) {
	// An opaque string is good enough for our purposes.
	// Size is arbitrary.
	token := make([]byte, 40)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("reading random bytes: %w", err)
	}

	tokenB64 := base64.StdEncoding.EncodeToString(token)
	expires := a.clock.Now().Add(TokenExpiryPeriod).UTC()
	return &authToken{
		AuthToken: jamf.AuthToken{
			Token:   tokenB64,
			Expires: expires,
		},
		owner: owner,
	}, nil
}

func (a *API) postAuthKeepAlive(w http.ResponseWriter, req *http.Request) {
	currentToken := req.Context().Value(authTokenKey{}).(*authToken)

	newToken, err := a.newAuthToken(currentToken.owner)
	if err != nil {
		// Unexpected. Error not observed in practice.
		a.replyError(w, errorResponse{HTTPStatus: 500})
		return
	}

	a.mu.Lock()

	// Issue new token for user.
	a.issuedTokens[newToken.Token] = newToken

	// Rescind old token.
	delete(a.issuedTokens, currentToken.Token)

	a.mu.Unlock()

	// Reply.
	a.replyJSON(w, 200, newToken)
}

func (a *API) getComputersInventory(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()

	// page.
	var page int
	if val := q.Get("page"); val != "" {
		// API ignores errors/negative.
		n, err := strconv.Atoi(val)
		if err == nil && n > 0 {
			page = n
		}
	}

	// page-size.
	pageSize := 100 // default
	if val := q.Get("page-size"); val != "" {
		// API ignores errors/negative.
		n, err := strconv.Atoi(val)
		if err == nil && n > 0 {
			pageSize = n
		}
	}

	// section.
	sections := q["section"]
	if _, err := copySections(&jamf.ComputerInventory{}, sections); err != nil {
		a.replyError(w, errorResponse{
			HTTPStatus: 400,
			Errors: []*apiError{
				{
					Code:        "INVALID_REQUEST_PARAMETER_VALUE",
					Description: err.Error(),
					ID:          "0",
				},
			},
		})
		return
	}

	// sort.
	sorter := byGeneralName
	switch val, ok := q["sort"]; {
	case len(val) > 1:
		a.replyError(w, errorResponse{
			HTTPStatus: 500,
			Errors: []*apiError{
				{Description: "multiple sort values not supported by the fake API"},
			},
		})
		return
	case ok:
		tmp := strings.Split(val[0], ":")
		field := tmp[0]
		// verb is tmp[1], defaults to "asc".

		switch field {
		case "id":
			sorter = byID
		case "udid":
			sorter = byUDID
		case "general.name":
			sorter = byGeneralName
		case "general.reportDate":
			sorter = byGeneralReportDate
		case "general.lastContactTime":
			sorter = byGeneralLastContactTime
		// Many more fields are supported by the actual API, but not by us.
		default:
			a.replyError(w, errorResponse{
				HTTPStatus: 400,
				Errors: []*apiError{
					{
						Code: "INVALID_FIELD",
						// Example of an actual error:
						// "No property '$field' found for type 'ComputersDenormalizedEntity'!"
						Description: fmt.Sprintf("sorting by %q not implemented by fake", field),
						ID:          "0",
					},
				},
			})
			return
		}

		// Verb validation is lenient, any errors are ignored.
		// Note: unsure if verb matching is case-sensitive.
		if len(tmp) > 1 && tmp[1] == "desc" {
			prev := sorter
			sorter = func(a []*jamf.ComputerInventory) sort.Interface {
				return sort.Reverse(prev(a))
			}
		}
	}

	// Lock inventory, then:
	// - Sort underlying inventory
	// - Paginate
	// - Copy devices applying section filters
	a.mu.Lock()
	totalCount := len(a.inventory)

	// Sort.
	sort.Sort(sorter(a.inventory))

	// Paginate results.
	start := page * pageSize
	if start > totalCount {
		start = totalCount
	}
	end := start + pageSize
	if end > totalCount {
		end = totalCount
	}
	inv := a.inventory[start:end]

	if a.simulatePagingGaps && len(inv) > 0 {
		inv = inv[1:]
	}

	// Copy and apply sections.
	resp := make([]*jamf.ComputerInventory, 0, pageSize)
	for _, c := range inv {
		// It's strange for a computer to be entirely new, but we let it happen so
		// we can harden our production implementation.
		if c == nil {
			resp = append(resp, nil)
			continue
		}

		// err safe to swallow, sections are validated above.
		cp, _ := copySections(c, sections)
		resp = append(resp, cp)
	}
	a.mu.Unlock()

	a.replyJSON(w, 200, &jamf.GetComputersInventoryResponse{
		TotalCount: totalCount,
		Results:    resp,
	})
}

func (a *API) getComputersInventoryByID(id string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate "section" parameter.
		q := r.URL.Query()
		sections := q["section"]
		if _, err := copySections(&jamf.ComputerInventory{}, sections); err != nil {
			a.replyError(w, errorResponse{
				HTTPStatus: 400,
				Errors: []*apiError{
					{
						Code:        "INVALID_REQUEST_PARAMETER_VALUE",
						Description: err.Error(),
						ID:          "0",
					},
				},
			})
			return
		}

		a.mu.Lock()
		defer a.mu.Unlock()
		for _, c := range a.inventory {
			if c != nil && c.ID == id {
				// err safe to swallow, sections are validated above.
				cp, _ := copySections(c, sections)
				a.replyJSON(w, 200, cp)
				return
			}
		}

		a.replyError(w, errorResponse{
			HTTPStatus: 404,
			Errors: []*apiError{
				{
					Code:        "INVALID_ID",
					Description: "computer with given id does not exist",
					ID:          id,
				},
			},
		})
	}
}

func copySections(c *jamf.ComputerInventory, sections []string) (*jamf.ComputerInventory, error) {
	cp := &jamf.ComputerInventory{
		ID:   c.ID,
		UDID: c.UDID,
	}
	if len(sections) == 0 {
		sections = []string{jamf.SectionGeneral} // default
	}

	for _, s := range sections {
		// Note: section matching _IS_ case-sensitive.
		switch s {
		case jamf.SectionGeneral:
			cp.General = c.General
		case jamf.SectionHardware:
			cp.Hardware = c.Hardware
		case jamf.SectionLocalUserAccounts:
			cp.LocalUserAccounts = c.LocalUserAccounts
		case jamf.SectionOperatingSystem:
			cp.OperatingSystem = c.OperatingSystem
		default:
			allSections := []string{jamf.SectionGeneral, jamf.SectionHardware, jamf.SectionLocalUserAccounts, jamf.SectionOperatingSystem}
			// Error message copied from actual API.
			return nil, fmt.Errorf(
				"invalid value of request parameter: %v, Possible values: %v",
				s, strings.Join(allSections, ","))
		}
	}

	return cp, nil
}

func byID(a []*jamf.ComputerInventory) sort.Interface {
	return computerInventorySorter{
		elems: a,
		less: func(c1, c2 *jamf.ComputerInventory) bool {
			var id1, id2 string
			if c1 != nil {
				id1 = c1.ID
			}
			if c2 != nil {
				id2 = c2.ID
			}
			return id1 < id2
		},
	}
}

func byUDID(a []*jamf.ComputerInventory) sort.Interface {
	return computerInventorySorter{
		elems: a,
		less: func(c1, c2 *jamf.ComputerInventory) bool {
			var id1, id2 string
			if c1 != nil {
				id1 = c1.UDID
			}
			if c2 != nil {
				id2 = c2.UDID
			}
			return id1 < id2
		},
	}
}

func byGeneralName(a []*jamf.ComputerInventory) sort.Interface {
	return computerInventorySorter{
		elems: a,
		less: func(c1, c2 *jamf.ComputerInventory) bool {
			var n1, n2 string
			if c1 != nil && c1.General != nil {
				n1 = c1.General.Name
			}
			if c2 != nil && c2.General != nil {
				n2 = c2.General.Name
			}
			return n1 < n2
		},
	}
}

func byGeneralLastContactTime(a []*jamf.ComputerInventory) sort.Interface {
	return computerInventorySorter{
		elems: a,
		less: func(c1, c2 *jamf.ComputerInventory) bool {
			var t1, t2 time.Time
			if c1 != nil && c1.General != nil {
				t1 = c1.General.LastContactTime
			}
			if c2 != nil && c2.General != nil {
				t2 = c2.General.LastContactTime
			}
			return t1.Before(t2)
		},
	}
}

func byGeneralReportDate(a []*jamf.ComputerInventory) sort.Interface {
	return computerInventorySorter{
		elems: a,
		less: func(c1, c2 *jamf.ComputerInventory) bool {
			var t1, t2 time.Time
			if c1 != nil && c1.General != nil {
				t1 = c1.General.ReportDate
			}
			if c2 != nil && c2.General != nil {
				t2 = c2.General.ReportDate
			}
			return t1.Before(t2)
		},
	}
}

type computerInventorySorter struct {
	elems []*jamf.ComputerInventory
	less  func(c1, c2 *jamf.ComputerInventory) bool
}

func (s computerInventorySorter) Len() int {
	return len(s.elems)
}

func (s computerInventorySorter) Less(i int, j int) bool {
	return s.less(s.elems[i], s.elems[j])
}

func (s computerInventorySorter) Swap(i int, j int) {
	s.elems[i], s.elems[j] = s.elems[j], s.elems[i]
}

func (a *API) replyError(w http.ResponseWriter, resp errorResponse) {
	if resp.Errors == nil {
		resp.Errors = []*apiError{} // Marshal empty instead of null.
	}

	a.replyJSON(w, resp.HTTPStatus, resp)
}

func (a *API) replyJSON(w http.ResponseWriter, code int, resp any) {
	body, err := json.Marshal(resp)
	if err != nil {
		slog.WarnContext(context.Background(),
			"Failed to marshal JSON response",
			"error", err,
		)
	}

	w.WriteHeader(code)
	w.Write(body)
}
