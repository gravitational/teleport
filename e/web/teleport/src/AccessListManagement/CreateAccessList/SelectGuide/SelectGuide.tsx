import { useEffect } from 'react';
import { Link } from 'react-router';

import { Box, ButtonIcon, Flex, H1 } from 'design';
import { Cross } from 'design/Icon';
import {
  InfoGuideButton,
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { getMissingRoleAccess } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/role';
import cfg from 'e-teleport/config';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';
import useTeleport from 'teleport/useTeleport';

import { NoAccessState } from '../../NoAccessState';
import {
  FeatureLimitReached,
  FeatureLimitReachedBlur,
} from '../../Shared/FeatureLimitReached';
import { useCreateAccessList } from '../CreateAccessListContextProvider';
import { StartForm } from './StartForm';

export function SelectGuide() {
  const ctx = useTeleport();
  const { setInfoGuideConfig } = useInfoGuide();
  const { featureLimitReached, canCreateAccessList, setSpec, spec } =
    useCreateAccessList();
  const { guideEditor } = useAccessListManagementContext();
  const { onGuideSelect } = guideEditor;

  useEffect(() => {
    // Default to having the side panel opened.
    if (canCreateAccessList) {
      setInfoGuideConfig({ guide: <InfoGuide />, viewHasOwnSidePanel: true });
    }
  }, []);

  if (!canCreateAccessList) {
    return <NoAccessState action="create" />;
  }

  const missingRoleAccess = getMissingRoleAccess(
    ctx.storeUser.getRoleAccess(),
    'crud'
  );
  const hasRoleAccess = missingRoleAccess.length === 0;

  function onStart(
    preset: AccessListPreset,
    accessListName: string,
    description: string
  ) {
    if (featureLimitReached || (preset !== '' && !hasRoleAccess)) {
      return;
    }

    setSpec({ ...spec, title: accessListName, description });
    onGuideSelect(preset);
  }

  return (
    <>
      <Flex alignItems={'center'}>
        <ButtonIcon ml={-5} as={Link} to={cfg.getAccessListManagementRoute()}>
          <Cross size="medium" />
        </ButtonIcon>
        <Flex gap={3} justifyContent="space-between" width="100%">
          <H1 my={3}>Create a new Access List</H1>
          <InfoGuideButton config={{ guide: <InfoGuide /> }} />
        </Flex>
      </Flex>

      {featureLimitReached && <FeatureLimitReached />}

      <FeatureLimitReachedBlur featureLimitReached={featureLimitReached}>
        <StartForm
          onStart={onStart}
          hasRoleAccess={hasRoleAccess}
          missingRoleAccess={missingRoleAccess}
          featureLimitReached={featureLimitReached}
        />
      </FeatureLimitReachedBlur>
    </>
  );
}

const infoGuideReferenceLinks = {
  AccessList: {
    title: 'Learn more',
    href: 'https://goteleport.com/docs/reference/access-controls/access-lists',
  },
};

function InfoGuide() {
  return (
    <Box>
      <InfoTitle>What are Access Lists?</InfoTitle>
      <InfoParagraph>
        Access Lists allow Teleport users to be granted access to resources
        managed within Teleport. With Access Lists, administrators and Access
        List owners can regularly audit and control membership to specific roles
        and traits.
      </InfoParagraph>
      <InfoTitle>When to use an Access List</InfoTitle>
      <InfoParagraph>
        Access Lists are ideal for scenarios where users or groups require
        ongoing permissions, typically over a period of months, rather than
        temporary, just-in-time elevation of privileges.
      </InfoParagraph>
      <InfoTitle>Achieve Compliance</InfoTitle>
      <InfoParagraph>
        Access Lists are also useful when you want to enforce regular access
        reviews and maintain a clear audit trail for compliance or security
        purposes
      </InfoParagraph>
      <InfoTitle>Get Visibility into Access</InfoTitle>
      <InfoParagraph>
        See why access has been granted and who has granted it. See people
        responsible for access.
      </InfoParagraph>
      <ReferenceLinks links={Object.values(infoGuideReferenceLinks)} />
    </Box>
  );
}
