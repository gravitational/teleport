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

import { Location } from 'history';
import React from 'react';
import { useLocation } from 'react-router';
import { Link } from 'react-router-dom';

import { ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';
import { CircleCheck } from 'design/Icon';

import cfg from 'teleport/config';
import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import { IntegrationAwsOidc } from 'teleport/services/integrations';

export function FinishDialog({
  integration,
}: {
  integration: IntegrationAwsOidc;
}) {
  const location = useLocation<DiscoverUrlLocationState>();
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={true}
      onClose={close}
      open={true}
    >
      <DialogHeader css={{ margin: '0 auto' }}>
        <CircleCheck mb={4} size={60} color="success.main" />
      </DialogHeader>
      <DialogContent>
        <Text textAlign="center">
          AWS integration "{integration.name}" successfully added
        </Text>
      </DialogContent>
      <DialogFooter css={{ margin: '0 auto' }}>
        <FooterButton location={location} integration={integration} />
      </DialogFooter>
    </Dialog>
  );
}

function FooterButton({
  location,
  integration,
}: {
  location: Location<any>;
  integration: IntegrationAwsOidc;
}): React.ReactElement {
  if (location.state?.discover) {
    return (
      <ButtonPrimary
        size="large"
        as={Link}
        to={{
          pathname: cfg.routes.discover,
          state: {
            integration,
            discover: location.state.discover,
          },
        }}
      >
        Begin AWS Resource Enrollment
      </ButtonPrimary>
    );
  }

  if (location.state?.integration) {
    return (
      <ButtonPrimary
        size="large"
        as={Link}
        to={cfg.getIntegrationEnrollRoute(location.state.integration.kind)}
      >
        {location.state.integration?.redirectText || `Back to integration`}
      </ButtonPrimary>
    );
  }

  return (
    <Flex gap="3">
      <ButtonPrimary as={Link} to={cfg.routes.integrations} size="large">
        Go to Integration List
      </ButtonPrimary>
      <ButtonSecondary
        as={Link}
        to={cfg.getIntegrationEnrollRoute(null)}
        size="large"
      >
        Add Another Integration
      </ButtonSecondary>
    </Flex>
  );
}
