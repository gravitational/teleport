// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

// Package pagination supplies vocabulary types and definitions to help make
// paging through record sets less error prone. The goals of this package are to
//  1. make bad conversions impossible by having separate types for the
//     "page request" token used by the caller and "next page" token returned
//     by the callee.
//  2. force the single use of a PageRequestToken value, and returning an error
//     if a token is recycled without being updated.
//
// For example, imagine a service that lists widgets.
//
//	type WidgetLister interface {
//		ListWidgets(context.Context, *pagination.PageRequestToken) ([]Widget, NextPageToken, error)
//	}
//
// Callers can page through the Widget list like so:
//
//	func iterateWidgets(ctx context.Context, widgetLister widgetLister) error {
//		var pageToken pagination.PageRequestToken
//		for {
//			_, nextPage, err := widgetLister.ListWidgets(ctx, &pageToken)
//			if err != nil {
//				return trace.Wrap(err)
//			}
//
//			if nextPage == pagination.EndOfList {
//				break
//			}
//			pageToken.Update(nextPage)
//		}
//		return nil
//	}
//
// A callee can retrieve the underlying token by calling Consume() on the
// supplied PageRequestToken, which will mark the token as stale. Using a stale
// token is an error and will cause the method to fail if a token is recycled.
//
//	func (_ *someType) ListWidgets(context.Context, pageRequestToken *pagination.PageRequestToken) ([]Widget, pagination.NextPageToken, error)
//		pageToken, err := pageRequestToken.Consume()
//		if err != nil {
//			return nil, NextPageToken(""), trace.Wrap(err)
//		}
//
//		// now feed the extracted page token to the underlying service or
//		// backend and do whatever you need to do
//
//		nextPage := pagination.NextPageToken(/* whatever next-page token was returned by backend */)
//		if thisIsTheLastPage {
//			nextPage = pagination.EndOfList
//		}
//		return []Widget{}, nextPage, nil
//	}
package pagination

import "github.com/gravitational/trace"

// NextPageToken is an indication returned by a function showing where the next
// page of data to be returned starts.
type NextPageToken string

const (
	// EndOfList is a NextPageToken that marks the end of the list. No more
	// iteration is required.
	EndOfList NextPageToken = ""
)

// PageRequestToken is a request for a given page of data. The only requirement
// is that the underlying format of the page request is the same as its
// corresponding NextPageToken.
//
// The underlying token value is retrieved by calling `Consume()`, and may only
// be retrieved once before being reset with a `NextPageToken`. This is so we can
// easily detect pagination issues where the request token value is not updated
// between calls and token values are incorrectly recycled.
//
// The zero value of this struct will fit most use cases (i.e., where the first
// page token is the empty string).
type PageRequestToken struct {
	token string
	stale bool
}

// Consume moves the token value out of the PageRequestToken and marks the token
// as stale. If the token is already stale, this method will fail with
// BadParameter.
func (t *PageRequestToken) Consume() (string, error) {
	if t.stale {
		return "", trace.BadParameter("page request token is stale")
	}

	token := t.token
	t.token = ""
	t.stale = true

	return token, nil
}

// Update updates a PageRequestToken with a token value from the supplied
// NextPageToken
func (t *PageRequestToken) Update(token NextPageToken) {
	t.token = string(token)
	t.stale = false
}
