import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { ResourceSelectedIcon } from 'shared/components/UnifiedResources/shared/ResourceSelectedIcon';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';

import {
  DefinableResourceAccessFields,
  definableResourceAccessFields,
} from '../role/listaccess';
import { SimpleResourceIcon } from '../SimpleResourceIcon';

/**
 * Used as a side section for DefineAccess.tsx where users
 * can select from different resource access to define.
 */
export function ResourceTabs({
  selectedTab,
  onTabSelect,
}: {
  selectedTab: DefinableResourceAccessFields;
  onTabSelect(kind: DefinableResourceAccessFields): void;
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { definedAccess } = guideEditor;

  return (
    <Box
      width="245px"
      css={`
        border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
        border-top-left-radius: ${p => p.theme.radii[2]}px;
        border-bottom-left-radius: ${p => p.theme.radii[2]}px;
        border-right-style: none;
        height: var(--guide-section-height);
        overflow: scroll;
      `}
    >
      {definableResourceAccessFields.map(field => {
        const sharedProps = {
          hasAccess: definedAccess(field),
          active: selectedTab === field,
          onClick: () => onTabSelect(field),
          icon: <SimpleResourceIcon field={field} />,
          dataTestId: field,
        };
        switch (field) {
          case 'app_labels':
            return (
              <ResourceTab
                key={field}
                {...sharedProps}
                firstTab
                label="Application"
              />
            );

          case 'awsIc':
            return (
              <ResourceTab
                key={field}
                {...sharedProps}
                label="Identity Center"
              />
            );

          case 'db_labels':
            return (
              <ResourceTab key={field} {...sharedProps} label="Database" />
            );

          case 'github_permissions':
            return (
              <ResourceTab key={field} {...sharedProps} label="Git Server" />
            );

          case 'kubernetes_labels':
            return (
              <ResourceTab key={field} {...sharedProps} label="Kubernetes" />
            );

          case 'node_labels':
            return <ResourceTab key={field} {...sharedProps} label="Server " />;

          case 'windows_desktop_labels':
            return (
              <ResourceTab
                key={field}
                {...sharedProps}
                label="Windows Desktop"
              />
            );

          case 'linux_desktop_labels':
            return (
              <ResourceTab key={field} {...sharedProps} label="Linux Desktop" />
            );

          default:
            field satisfies never;
        }
      })}
    </Box>
  );
}

function ResourceTab({
  label,
  active,
  firstTab,
  hasAccess,
  onClick,
  icon,
  dataTestId,
}: {
  label: string;
  active: boolean;
  firstTab?: boolean;
  hasAccess: boolean;
  onClick(): void;
  icon: React.ReactNode;
  dataTestId?: string;
}) {
  return (
    <ResourceTabContainer
      active={active}
      firstTab={firstTab}
      onClick={onClick}
      data-testid={dataTestId}
    >
      <Flex
        p={3}
        width="100%"
        alignItems="center"
        justifyContent="space-between"
      >
        <Flex gap={2}>
          {icon}
          <Text bold={active}>{label}</Text>
        </Flex>
        {hasAccess && <ResourceSelectedIcon />}
      </Flex>
    </ResourceTabContainer>
  );
}

const ResourceTabContainer = styled(Flex)<{
  active: boolean;
  firstTab: boolean;
}>`
  cursor: pointer;
  border-bottom: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  ${p => p.firstTab && `border-top-left-radius: ${p.theme.radii[2]}px;`};

  ${p =>
    p.active &&
    `
  background-color: ${p.theme.colors.levels.elevated};
  border-left: 4px solid
    ${p.theme.colors.interactive.solid.primary.default};
  `};
`;
