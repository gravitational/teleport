/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package watchers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

type awsPagesAPI[Input, Page any] func(aws.Context, *Input, func(*Page, bool) bool, ...request.Option) error

func getAWSPages[Input, Page any](ctx context.Context, api awsPagesAPI[Input, Page], input *Input) ([]*Page, error) {
	var pageNum int
	var pages []*Page
	if input == nil {
		input = new(Input)
	}
	err := api(ctx, input, func(page *Page, _ bool) bool {
		pageNum++
		pages = append(pages, page)
		return pageNum <= common.MaxPages
	})
	return pages, awslib.ConvertRequestFailureError(err)
}

func pagesToItems[Page, Item any](pages []*Page, pageToItems func(*Page) []Item) (items []Item) {
	for _, page := range pages {
		items = append(items, pageToItems(page)...)
	}
	return items
}
