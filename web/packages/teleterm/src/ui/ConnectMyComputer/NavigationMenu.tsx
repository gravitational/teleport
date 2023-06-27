/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useCallback, useRef, useState } from 'react';
import Popover from 'design/Popover';
import styled from 'styled-components';
import { Box } from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ListItem } from 'teleterm/ui/components/ListItem';

import { NavigationMenuIcon } from './NavigationMenuIcon';

export function NavigationMenu() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const appCtx = useAppContext();

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }, []);

  function openSetupDocument(): void {
    const documentService =
      appCtx.workspacesService.getActiveWorkspaceDocumentService();
    const rootClusterUri = appCtx.workspacesService.getRootClusterUri();
    const document = documentService.createConnectMyComputerSetupDocument({
      rootClusterUri,
    });
    documentService.add(document);
    documentService.open(document.uri);
    setIsPopoverOpened(false);
  }

  return (
    <>
      <NavigationMenuIcon onClick={togglePopover} ref={iconRef} />
      <Popover
        open={isPopoverOpened}
        anchorEl={iconRef.current}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        onClose={() => setIsPopoverOpened(false)}
      >
        <Container width="200px">
          <ListItem onClick={openSetupDocument}>Set up agent</ListItem>
        </Container>
      </Popover>
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.levels.elevated};
`;
