/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package regions

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/account"
	accounttypes "github.com/aws/aws-sdk-go-v2/service/account/types"
	"github.com/gravitational/trace"

	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// ListerGetter gets a client that is capable of listing AWS regions.
type ListerGetter func(ctx context.Context, opts ...awsconfig.OptionsFn) (account.ListRegionsAPIClient, error)

// ListEnabledRegions returns every region enabled for the caller's AWS
// account, using the account:ListRegions API.
func ListEnabledRegions(ctx context.Context, listerGetter ListerGetter, opts ...awsconfig.OptionsFn) ([]string, error) {
	regionsListerClient, err := listerGetter(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	paginator := account.NewListRegionsPaginator(regionsListerClient, &account.ListRegionsInput{
		RegionOptStatusContains: []accounttypes.RegionOptStatus{
			accounttypes.RegionOptStatusEnabled,
			accounttypes.RegionOptStatusEnabledByDefault,
		},
	})

	var enabledRegions []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			convertedErr := libcloudaws.ConvertRequestFailureError(err)
			if trace.IsAccessDenied(convertedErr) {
				return nil, trace.BadParameter("Missing account:ListRegions permission in IAM Role, which is required to iterate over all regions. " +
					"Add this permission to the IAM Role, or enumerate the regions explicitly.")
			}
			return nil, convertedErr
		}

		for _, region := range page.Regions {
			enabledRegions = append(enabledRegions, aws.ToString(region.RegionName))
		}
	}

	return enabledRegions, nil
}
