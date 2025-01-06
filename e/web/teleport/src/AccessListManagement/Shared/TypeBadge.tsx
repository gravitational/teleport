import { Flex, ResourceIcon, Text } from 'design';

import { AccessListType } from 'e-teleport/services/accessmanagement';

/**
 * TypeBadge renders an icon and title of an integration for which
 * or from which the Access List was created.
 */
export const TypeBadge = ({ type }: { type: AccessListType }) => {
  if (type === AccessListType.Unspecified || !type) return null;

  function RenderBadge() {
    let iconName, title;
    switch (type) {
      case AccessListType.AwsIdentityCenter:
        iconName = 'aws';
        title = 'AWS';
        break;
      case AccessListType.Okta:
        iconName = 'okta';
        title = 'Okta';
        break;
    }
    return (
      <>
        <ResourceIcon name={iconName} height="16px" />
        <Text typography="body3">{title}</Text>
      </>
    );
  }
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      gap={1}
      css={`
        background: ${props => props.theme.colors.spotBackground[0]};
        border-radius: 35px;
        min-width: 60px;
        height: 22px;
        color: ${p => p.theme.colors.text.slightlyMuted};
      `}
    >
      <RenderBadge />
    </Flex>
  );
};
