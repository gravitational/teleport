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

package db

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

func TestOpenSearchFetcher(t *testing.T) {
	t.Parallel()

	tags := map[string][]*opensearchservice.Tag{}
	prod, prodDBs := makeOpenSearchDomain(t, tags, "os1", "us-east-1", "prod")
	prodDisabled, _ := makeOpenSearchDomain(t, tags, "os2", "us-east-1", "prod", func(status *opensearchservice.DomainStatus) {
		status.Created = aws.Bool(false)
	})

	prodVPC, prodVPCDBs := makeOpenSearchDomain(t, tags, "os3", "us-east-1", "prod", mocks.WithOpenSearchVPCEndpoint("vpc"))
	prodCustom, prodCustomDBs := makeOpenSearchDomain(t, tags, "os4", "us-east-1", "prod", mocks.WithOpenSearchCustomEndpoint("opensearch.example.com"))

	test, testDBs := makeOpenSearchDomain(t, tags, "os5", "us-east-1", "test")

	tests := []awsFetcherTest{
		{
			name: "fetch all",
			inputClients: &cloud.TestCloudClients{
				OpenSearch: &mocks.OpenSearchMock{
					Domains:   []*opensearchservice.DomainStatus{prod, test},
					TagsByARN: tags,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherOpenSearch, "us-east-1", wildcardLabels),
			wantDatabases: append(append(types.Databases{}, prodDBs...), testDBs...),
		},
		{
			name: "fetch prod",
			inputClients: &cloud.TestCloudClients{
				OpenSearch: &mocks.OpenSearchMock{
					Domains:   []*opensearchservice.DomainStatus{prod, test},
					TagsByARN: tags,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherOpenSearch, "us-east-1", envProdLabels),
			wantDatabases: prodDBs,
		},
		{
			name: "skip unavailable",
			inputClients: &cloud.TestCloudClients{
				OpenSearch: &mocks.OpenSearchMock{
					Domains:   []*opensearchservice.DomainStatus{prod, prodDisabled},
					TagsByARN: tags,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherOpenSearch, "us-east-1", wildcardLabels),
			wantDatabases: prodDBs,
		},
		{
			name: "prod default",
			inputClients: &cloud.TestCloudClients{
				OpenSearch: &mocks.OpenSearchMock{
					Domains:   []*opensearchservice.DomainStatus{prod, prodVPC, prodCustom},
					TagsByARN: tags,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherOpenSearch, "us-east-1", map[string]string{"endpoint-type": apiawsutils.OpenSearchDefaultEndpoint}),
			wantDatabases: types.Databases{prodDBs[0], prodCustomDBs[0]}, // domain with custom endpoint will still have default endpoint populated
		},
		{
			name: "prod custom",
			inputClients: &cloud.TestCloudClients{
				OpenSearch: &mocks.OpenSearchMock{
					Domains:   []*opensearchservice.DomainStatus{prod, prodVPC, prodCustom},
					TagsByARN: tags,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherOpenSearch, "us-east-1", map[string]string{"endpoint-type": apiawsutils.OpenSearchCustomEndpoint}),
			wantDatabases: types.Databases{prodCustomDBs[1]}, // domain with custom endpoint will still have default endpoint populated
		},
		{
			name: "prod vpc",
			inputClients: &cloud.TestCloudClients{
				OpenSearch: &mocks.OpenSearchMock{
					Domains:   []*opensearchservice.DomainStatus{prod, prodVPC, prodCustom},
					TagsByARN: tags,
				},
			},
			inputMatchers: makeAWSMatchersForType(types.AWSMatcherOpenSearch, "us-east-1", map[string]string{"endpoint-type": apiawsutils.OpenSearchVPCEndpoint}),
			wantDatabases: prodVPCDBs,
		},
	}
	testAWSFetchers(t, tests...)
}

func makeOpenSearchDomain(t *testing.T, tagMap map[string][]*opensearchservice.Tag, name, region, env string, opts ...func(status *opensearchservice.DomainStatus)) (*opensearchservice.DomainStatus, types.Databases) {
	domain := mocks.OpenSearchDomain(name, region, opts...)

	tags := []*opensearchservice.Tag{{
		Key:   aws.String("env"),
		Value: aws.String(env),
	}}

	tagMap[aws.StringValue(domain.ARN)] = tags

	databases, err := common.NewDatabasesFromOpenSearchDomain(domain, tags)
	require.NoError(t, err)

	for _, db := range databases {
		common.ApplyAWSDatabaseNameSuffix(db, types.AWSMatcherOpenSearch)
	}
	return domain, databases
}
