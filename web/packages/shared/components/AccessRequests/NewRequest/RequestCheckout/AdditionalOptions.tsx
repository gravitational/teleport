/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { Box, ButtonIcon, Flex, LabelInput, Text } from 'design';
import * as Icon from 'design/Icon';
import { IconTooltip } from 'design/Tooltip';
import Select, { Option } from 'shared/components/Select';
import { AccessRequest } from 'shared/services/accessRequests';

import { getFormattedDurationTxt } from '../../Shared/utils';

export function AdditionalOptions({
  selectedMaxDurationTimestamp,
  setPendingRequestTtl,
  pendingRequestTtl,
  dryRunResponse,
  pendingRequestTtlOptions,
}: {
  selectedMaxDurationTimestamp: number;
  setPendingRequestTtl(o: Option<number>): void;
  pendingRequestTtl: Option<number>;
  dryRunResponse: AccessRequest;
  pendingRequestTtlOptions: Option<number>[];
}) {
  const [expanded, setExpanded] = useState(false);
  const ArrowIcon = expanded ? Icon.ChevronDown : Icon.ChevronRight;

  return (
    <>
      <Flex
        mt={1}
        mb={2}
        pb={2}
        justifyContent="space-between"
        alignItems="center"
        height="34px"
        css={`
          border-color: ${props => props.theme.colors.spotBackground[1]};
        `}
      >
        <Text mr={2} typography="body3">
          Additional Options
        </Text>
        <ButtonIcon
          onClick={() => setExpanded(e => !e)}
          data-testid="additional-info-btn"
        >
          <ArrowIcon size="medium" />
        </ButtonIcon>
      </Flex>
      {expanded && (
        <Box data-testid="reviewers">
          {pendingRequestTtlOptions.length > 0 && (
            <LabelInput color="text.slightlyMuted" mb={3}>
              <Flex alignItems="center">
                <Text mr={1}>Request expires if not reviewed in</Text>
                <IconTooltip>
                  The request TTL which is the amount of time this request will
                  be in the PENDING state before it expires.
                </IconTooltip>
              </Flex>
              <Select
                options={pendingRequestTtlOptions}
                onChange={(option: Option<number>) =>
                  setPendingRequestTtl(option)
                }
                value={pendingRequestTtl}
              />
            </LabelInput>
          )}
          <LabelInput color="text.slightlyMuted">
            <Flex alignItems="center">
              <Text mr={1}>Access Request Lifetime</Text>
              <IconTooltip>
                The max duration of an access request, starting from its
                creation, until it expires.
              </IconTooltip>
            </Flex>
            <Text>
              {getFormattedDurationTxt({
                start: dryRunResponse.created,
                end: new Date(selectedMaxDurationTimestamp),
              })}
            </Text>
          </LabelInput>
        </Box>
      )}
    </>
  );
}
