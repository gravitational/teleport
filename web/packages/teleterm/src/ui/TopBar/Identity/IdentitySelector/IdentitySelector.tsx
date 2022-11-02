import React, { forwardRef } from 'react';
import { Box } from 'design';

import { getUserWithClusterName } from 'teleterm/ui/utils';

import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';

import { UserIcon } from './UserIcon';
import { PamIcon } from './PamIcon';

interface IdentitySelectorProps {
  isOpened: boolean;
  userName: string;
  clusterName: string;

  onClick(): void;
  makeTitle: (userWithClusterName: string | undefined) => string;
}

export const IdentitySelector = forwardRef<
  HTMLButtonElement,
  IdentitySelectorProps
>((props, ref) => {
  const isSelected = props.userName && props.clusterName;
  const selectorText = isSelected && getUserWithClusterName(props);
  const title = props.makeTitle(selectorText);

  return (
    <TopBarButton
      isOpened={props.isOpened}
      ref={ref}
      onClick={props.onClick}
      title={title}
    >
      {isSelected ? (
        <Box>
          <UserIcon letter={props.userName[0]} />
        </Box>
      ) : (
        <PamIcon />
      )}
    </TopBarButton>
  );
});
