/**
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

import { useState } from 'react';

import { Box, Indicator } from 'design';
import { Danger } from 'design/Alert';
import { P } from 'design/Text/Text';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';
import { isIamPermError } from 'teleport/Discover/Shared/Aws/error';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  integrationService,
  Regions,
  Vpc,
} from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import useTeleport from 'teleport/useTeleport';

import { Header } from '../../Shared';
import { AutoDiscoverToggle } from './AutoDiscoverToggle';
import { AutoEnrollment } from './AutoEnrollment';
import { SingleEnrollment } from './SingleEnrollment';
import { VpcOption, VpcSelector } from './VpcSelector';

export function EnrollRdsDatabase() {
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();

  const { agentMeta, emitErrorEvent } = useDiscover();

  // This attempt is used for both fetching vpc's and for
  // fetching databases since each fetching is done at separate
  // times and relies on one fetch result (vpcs) to be complete
  // before performing the next fetch (databases, but only after user
  // has selected a vpc).
  const { attempt: fetchAttempt, setAttempt: setFetchAttempt } = useAttempt('');

  const [vpcs, setVpcs] = useState<Vpc[] | undefined>();
  const [selectedVpc, setSelectedVpc] = useState<VpcOption>();
  const [wantAutoDiscover, setWantAutoDiscover] = useState(false);
  const [selectedRegion, setSelectedRegion] = useState<Regions>();

  function onNewVpc(selectedVpc: VpcOption) {
    setSelectedVpc(selectedVpc);
  }

  function onNewRegion(region: Regions) {
    setSelectedVpc(null);
    setSelectedRegion(region);
    fetchVpcs(region);
  }

  async function fetchVpcs(region: Regions) {
    setFetchAttempt({ status: 'processing' });
    try {
      const { spec, name: integrationName } = agentMeta.awsIntegration;
      const { awsAccountId } = splitAwsIamArn(spec.roleArn);

      // Get a list of every vpcs.
      let fetchedVpcs: Vpc[] = [];
      let nextPage = '';
      do {
        const { vpcs, nextToken } =
          await integrationService.fetchAwsDatabasesVpcs(
            integrationName,
            clusterId,
            {
              region: region,
              accountId: awsAccountId,
              nextToken: nextPage,
            }
          );

        fetchedVpcs = [...fetchedVpcs, ...vpcs];
        nextPage = nextToken;
      } while (nextPage);

      setVpcs(fetchedVpcs);
      setFetchAttempt({ status: '' });
    } catch (err) {
      const message = getErrMessage(err);
      setFetchAttempt({
        status: 'failed',
        statusText: message,
      });
      emitErrorEvent(`failed to fetch vpcs: ${message}`);
    }
  }

  function refreshVpcsAndDatabases() {
    clear();
    fetchVpcs(selectedRegion);
  }

  /**
   * Used when user changes a region.
   */
  function clear() {
    setFetchAttempt({ status: '' });
    if (selectedVpc) {
      setSelectedVpc(null);
    }
  }

  const hasIamPermError = isIamPermError(fetchAttempt);
  const showVpcSelector = !hasIamPermError && !!vpcs;
  const showAutoEnrollToggle =
    fetchAttempt.status !== 'failed' && !!selectedVpc;
  const hasVpcs = vpcs?.length > 0;

  const mainContentProps = {
    vpc: selectedVpc?.value,
    region: selectedRegion,
    fetchAttempt,
    onFetchAttempt: setFetchAttempt,
    disableBtns:
      fetchAttempt.status === 'processing' ||
      hasIamPermError ||
      fetchAttempt.status === 'failed',
  };

  return (
    <Box maxWidth="800px">
      <Header>Enroll RDS Database</Header>
      {fetchAttempt.status === 'failed' && !hasIamPermError && (
        <Danger mt={3}>{fetchAttempt.statusText}</Danger>
      )}
      <P mt={4}>
        Select a AWS Region and a VPC ID you would like to see databases for:
      </P>
      <AwsRegionSelector
        onFetch={onNewRegion}
        onRefresh={refreshVpcsAndDatabases}
        clear={clear}
        disableSelector={fetchAttempt.status === 'processing'}
      />
      {!vpcs && fetchAttempt.status === 'processing' && (
        <Box width="320px" textAlign="center">
          <Indicator delay="none" />
        </Box>
      )}
      {showVpcSelector && hasVpcs && (
        <VpcSelector
          vpcs={vpcs}
          selectedVpc={selectedVpc}
          onSelectedVpc={onNewVpc}
          selectedRegion={selectedRegion}
        />
      )}
      {showVpcSelector && !hasVpcs && (
        // TODO(lisa): negative margin was required since the
        // AwsRegionSelector added too much bottom margin.
        // Refactor AwsRegionSelector so margins can be controlled
        // outside of the component (or use flex columns with gap prop)
        <P mt={-3}>
          There are no VPCs defined in the selected region. Try another region.
        </P>
      )}
      {hasIamPermError && (
        <Box mb={5}>
          <ConfigureIamPerms
            kind="rds"
            region={selectedRegion}
            integrationRoleArn={agentMeta.awsIntegration.spec.roleArn}
          />
        </Box>
      )}
      {showAutoEnrollToggle && (
        <AutoDiscoverToggle
          wantAutoDiscover={wantAutoDiscover}
          toggleWantAutoDiscover={() => setWantAutoDiscover(b => !b)}
          disabled={fetchAttempt.status === 'processing'}
        />
      )}
      {wantAutoDiscover ? (
        <AutoEnrollment {...mainContentProps} key={mainContentProps.vpc?.id} />
      ) : (
        <SingleEnrollment
          {...mainContentProps}
          key={mainContentProps.vpc?.id}
        />
      )}
    </Box>
  );
}
