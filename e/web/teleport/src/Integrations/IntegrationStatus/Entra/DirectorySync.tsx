import { formatDistanceStrict } from 'date-fns';
import { useState } from 'react';
import { useHistory } from 'react-router-dom';
import styled from 'styled-components';

import {
  Box,
  ButtonBorder,
  ButtonSecondary,
  Flex,
  H2,
  Label,
  P3,
  Text,
} from 'design';
import { CardTile } from 'design/CardTile/CardTile';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { Edit, SyncAlt, UserList, Users } from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';

import cfg from 'e-teleport/config';
import {
  SettingsType,
  toFrienldyAccessListOwnersSource,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Entra/types';
import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import {
  IntegrationStatusCode,
  type Filters,
  type PluginEntraIdSpec,
  type PluginEntraIDStatusDetails,
  type PluginStatus,
} from 'teleport/services/integrations';

/**
 * DirectorySyncDetails is the details of the directory
 * sync status.
 */
export function DirectorySyncDetails({
  name,
  spec,
  status,
}: {
  name: string;
  spec: PluginEntraIdSpec;
  status: PluginStatus<PluginEntraIDStatusDetails>;
}) {
  const history = useHistory();

  function lastSynced() {
    let msg = `Last Synced: ${getDurationText(status?.lastRun)}`;
    if (status?.code === IntegrationStatusCode.Running) {
      return msg;
    }
    if (
      status.details.imported_users === 0 &&
      status.details.imported_groups === 0
    ) {
      msg = msg + ', failed.';
    } else {
      msg = msg + ', partially succeeded.';
    }

    return msg;
  }

  return (
    <CardTile width="100%">
      <Flex alignItems="center" justifyContent="space-between">
        <H2>Directory Sync</H2>
        <StatusAndOptions
          enabled={true}
          disabled={false}
          options={[
            {
              label: 'Edit Configuration',
              onClick: () =>
                history.push(
                  cfg.oss.getIntegrationStatusRoute(
                    'entra-id',
                    name,
                    SettingsType.GroupImport
                  )
                ),
              Icon: Edit,
            },
            {
              label: 'Go to Users',
              onClick: () => history.push(`${cfg.oss.getUsersRoute()}`),
              Icon: Users,
            },
            {
              label: 'Go to Access Lists',
              onClick: () =>
                history.push(
                  `${cfg.getAccessListManagementRoute()}?search=entra`
                ),
              Icon: UserList,
            },
          ]}
        />
      </Flex>

      <Flex flexWrap="wrap" gap={6} mt={2} px={1}>
        <ImportSummary
          title="Users"
          num={status?.details?.imported_users}
          importSuceeded={status?.code === IntegrationStatusCode.Running}
          desc="Synced as Teleport User Resources"
        />
        <Flex flexDirection="column">
          <span
            css={`
              @media screen and (max-width: ${p =>
                p.theme.breakpoints.tablet}) {
                border-left: none;
                border-top: 1px solid ${p => p.theme.colors.spotBackground[2]};
                width: 100%;
                height: 1px;
              }
              border-left: 1px solid ${p => p.theme.colors.spotBackground[2]};
              height: 100%;
              width: 1px;
            `}
          />
        </Flex>
        <ImportSummary
          title="Groups"
          num={status?.details?.imported_groups}
          importSuceeded={status?.code === IntegrationStatusCode.Running}
          desc="Synced as Teleport Access List Resources"
        />
      </Flex>

      <Flex mt={4} px={1}>
        <span
          css={`
            border-left: none;
            border-top: 1px solid ${p => p.theme.colors.spotBackground[2]};
            width: 100%;
            height: 1px;
          `}
        />
      </Flex>

      <Flex flexDirection="column" px={1} gap={3} mt={3} mb={3}>
        <Text bold>Group Import Settings</Text>

        <SettingContainer>
          <SettingKey>Owners Source:</SettingKey>
          <Text color="text.slightlyMuted" pl={1}>
            <b>
              {toFrienldyAccessListOwnersSource(spec.accessListOwnersSource)}
            </b>
          </Text>
        </SettingContainer>
        <SettingContainer>
          <SettingKey>Default Owners:</SettingKey>
          {spec.defaultOwners?.length ? (
            <Flex
              flexDirection="row"
              flexWrap={'wrap'}
              rowGap={1}
              columnGap={2}
            >
              {spec.defaultOwners.map((label, index) => (
                <Label key={`${label}${index}`} kind="secondary">
                  {label}
                </Label>
              ))}
            </Flex>
          ) : (
            <Text>No default owners.</Text>
          )}
        </SettingContainer>

        <FilterDetails filters={spec.groupFilters} />
      </Flex>

      <Flex alignItems="center" mt={4} gap={2} px={1}>
        <Flex>
          <SyncAlt color="text.slightlyMuted" size="small" />
          <P3 ml={1} color="text.slightlyMuted">
            {lastSynced()}
          </P3>
        </Flex>

        <Flex>
          <ShowError
            statusCode={status.code}
            title={status.errorMessage}
            content={status.lastRawError}
          />
        </Flex>
      </Flex>
    </CardTile>
  );
}

function ImportSummary({
  title,
  num,
  desc,
  importSuceeded,
}: {
  title: string;
  num?: number;
  desc: string;
  importSuceeded: boolean;
}) {
  // Show zero only if import succeeded and [num] is zero because
  // for the failed state, we don't know if the [num] is zero due
  // to having an empty resource in the Entra ID directory or because
  // the reconciler failed despite having resources in the Entra ID
  // directory. Successful import with nullish or NaN is unexpected
  // and will be displayed as "-".
  function showNum() {
    if (importSuceeded && num === 0) {
      return num;
    }
    return num || '-';
  }

  return (
    <Flex flexDirection="column" gap={2}>
      <Text fontWeight={400} fontSize={10} css={{ lineHeight: 1 }}>
        {showNum()}
      </Text>
      <Text bold>{title}</Text>
      <Flex flexDirection="row" alignItems={'center'} gap={2}>
        <SettingKey>{desc}</SettingKey>
        <IconTooltip kind="info">
          <Text>
            Total count is the sum of resources synced during the latest sync
            cycle. Actual resource count in Teleport might be different.
          </Text>
        </IconTooltip>
      </Flex>
    </Flex>
  );
}

function FilterDetails({ filters }: { filters: Filters }) {
  if (!hasIncludeFilters(filters) && !hasExcludeFilters(filters)) {
    return (
      <SettingContainer>
        <SettingKey>Group Filters:</SettingKey>
        <Text color={'text.slightlyMuted'}>
          No filters configured, all groups are being synced.
        </Text>
      </SettingContainer>
    );
  }

  return (
    <>
      {hasIncludeFilters(filters) && (
        <SettingContainer>
          <SettingKey>Include group filters: </SettingKey>
          <Flex flexDirection="row" flexWrap={'wrap'} rowGap={1} columnGap={2}>
            {filters.nameRegex?.map((label, index) => (
              <Label key={`${label}${index}`} kind="secondary">
                {`nameRegex=${label}`}
              </Label>
            ))}
            {filters.id?.map((label, index) => (
              <Label key={`${label}${index}`} kind="secondary">
                {`id=${label}`}
              </Label>
            ))}
          </Flex>
        </SettingContainer>
      )}

      {hasExcludeFilters(filters) && (
        <SettingContainer>
          <SettingKey>Exclude group filters:</SettingKey>
          <Flex flexDirection="row" flexWrap={'wrap'} rowGap={1} columnGap={2}>
            {filters.excludeNameRegex?.map((label, index) => (
              <Label key={`${label}${index}`} kind="secondary">
                {`excludeNameRegex=${label}`}
              </Label>
            ))}
            {filters.excludeId?.map((label, index) => (
              <Label key={`${label}${index}`} kind="secondary">
                {`excludeId=${label}`}
              </Label>
            ))}
          </Flex>
        </SettingContainer>
      )}
    </>
  );
}

function ShowError({
  statusCode,
  title,
  content,
}: {
  statusCode: IntegrationStatusCode;
  title: string;
  content: string;
}) {
  const [open, setOpen] = useState(false);

  if (statusCode === IntegrationStatusCode.Running) {
    return null;
  }

  return (
    <>
      <ButtonBorder size="small" intent="danger" onClick={() => setOpen(true)}>
        View errors
      </ButtonBorder>
      <Dialog onClose={() => setOpen(false)} open={open}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <DialogContent>
          <ErrorPre>
            <code>{content}</code>
          </ErrorPre>
        </DialogContent>
        <DialogFooter>
          <ButtonSecondary onClick={() => setOpen(false)}>
            Close
          </ButtonSecondary>
        </DialogFooter>
      </Dialog>
    </>
  );
}

function hasIncludeFilters(filters: Filters): boolean {
  if (!filters) {
    return false;
  }
  return filters.id?.length > 0 || filters.nameRegex?.length > 0;
}

function hasExcludeFilters(filters: Filters): boolean {
  if (!filters) {
    return false;
  }
  return filters.excludeId?.length > 0 || filters.excludeNameRegex?.length > 0;
}

const SettingKey = styled(Text)`
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
`;

const SettingContainer = styled(Flex)`
  display: grid;
  @media screen and (max-width: ${p => p.theme.breakpoints.large}) {
    grid-template-columns: 1fr 5fr;
  }
  @media screen and (max-width: ${p => p.theme.breakpoints.medium}) {
    grid-template-columns: 1fr;
  }
  grid-template-columns: 1fr 6fr;
`;

function getDurationText(date: Date | undefined) {
  if (!date || date.getTime() <= 0) {
    return 'not recorded yet';
  }
  return formatDistanceStrict(date, new Date(), { addSuffix: true });
}

const ErrorPre = styled(Box).attrs({ as: 'pre' })`
  white-space: pre;
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  tab-size: ${({ theme }) => theme.space[3]}px;
  background-color: ${p => p.theme.colors.levels.sunken};
  border-width: 0;
  border-left-width: ${({ theme }) => theme.space[1]}px;
  border-style: solid;
  border-color: ${({ theme }) => theme.colors.interactive.solid.danger.default};
  padding: 0 ${({ theme }) => theme.space[2]}px;
  overflow: auto;
`;
