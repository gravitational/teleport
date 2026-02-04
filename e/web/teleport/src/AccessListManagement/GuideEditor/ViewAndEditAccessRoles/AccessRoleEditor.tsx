import { useMemo } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex } from 'design';
import { Cross, UserList } from 'design/Icon';

import { RoleToDelete } from 'e-teleport/AccessListManagement/ViewEditAccessList/DeleteAccessList/types';
import { BaseView } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import { Role } from 'teleport/services/resources';

import { useAccessListManagementContext } from '../../AccessListManagementContext';
import { NavView } from '../../CreateAccessList/types';
import { DefineAccess } from '../Preset/DefineAccess/DefineAccess';
import { DefineIdentities } from '../Preset/DefineIdentities/DefineIdentities';

/**
 * Renders like a dialog that fills the current view.
 * An editor just for editing roles for an access list.
 */
export function AccessRoleEditor({
  onClose,
  onUpdateAccess,
}: {
  onClose(): void;
  onUpdateAccess(accessRoles: Role[]): Promise<RoleToDelete[]>;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { preset, currentStep, undoEditRoleChanges } = guideEditor;

  const editViews: BaseView<NavView>[] = useMemo(() => {
    return [
      {
        title: 'Edit Resource Access',
        Component: <DefineAccess />,
      },
      {
        title: 'Edit Resource Identities',
        Component: (
          <DefineIdentities accessRoleEditor={{ onUpdateAccess, onClose }} />
        ),
      },
    ];
  }, []);

  function handleCancelEditor() {
    undoEditRoleChanges();
    onClose();
  }

  let views: BaseView<NavView>[] = [];
  let navTitle = '';
  switch (preset) {
    case 'long-term':
      navTitle = `Editing Resource Access (standing access)`;
      views = editViews;
      break;
    case 'short-term':
      navTitle = `Editing Resource Access (JIT access)`;
      views = editViews;
      break;
    case '':
      break;
    default:
      preset satisfies never;
  }

  return (
    <Box
      css={`
        position: absolute;
        top: 0px;
        left: 0px;
        right: 0;
        bottom: 0;
        background: ${p => p.theme.colors.levels.sunken};
        z-index: 13;
      `}
    >
      <StickyNav px={2} py={2}>
        <Flex mb={1} gap={2}>
          <ButtonIcon aria-label="Close" onClick={handleCancelEditor}>
            <Cross size="medium" />
          </ButtonIcon>
          <Navigation
            currentStep={currentStep}
            views={views}
            startWithIcon={{
              title: navTitle,
              component: <UserList size={20} />,
            }}
          />
        </Flex>
      </StickyNav>
      <Box>{views[currentStep].Component}</Box>
    </Box>
  );
}

const StickyNav = styled(Box)`
  position: sticky;
  top: 0;
  background-color: ${props => props.theme.colors.levels.sunken};
  border-bottom: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  z-index: 1;
`;
