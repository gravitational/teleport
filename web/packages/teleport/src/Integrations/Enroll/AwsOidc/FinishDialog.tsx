/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Location } from 'history';
import { useLocation } from 'react-router';
import { Link } from 'react-router-dom';
import { CircleCheck } from 'design/Icon';
import { ButtonPrimary, ButtonSecondary, Text, Flex } from 'design';
import Dialog, {
  DialogHeader,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import cfg from 'teleport/config';
import { Integration } from 'teleport/services/integrations';
import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';

export function FinishDialog({ integration }: { integration: Integration }) {
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
  integration: Integration;
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
