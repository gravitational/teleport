import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import {
  Box,
  ButtonSecondary,
  Link as ExternalLink,
  Flex,
  H1,
  Text,
} from 'design';
import { HoverTooltip } from 'design/Tooltip';
import {
  InfoGuideButton,
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';
import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import useTeleport from 'teleport/useTeleport';

import { NoAccessState } from '../../NoAccessState';
import { FeatureLimitReached } from '../../Shared/FeatureLimitReached';
import { useCreateAccessList } from '../CreateAccessListContextProvider';

type RoleAccess = {
  hasAccess: boolean;
  name: string;
};

export function SelectGuide() {
  const ctx = useTeleport();
  const { setInfoGuideConfig } = useInfoGuide();
  const { featureLimitReached, canCreateAccessList } = useCreateAccessList();
  const { guideEditor } = useAccessListManagementContext();
  const { setPreset } = guideEditor;

  useEffect(() => {
    // Default to having the side panel opened.
    if (canCreateAccessList) {
      setInfoGuideConfig({ guide: <InfoGuide /> });
    }
  }, []);

  if (!canCreateAccessList) {
    return <NoAccessState action="create" />;
  }

  // Using a "preset" guide requires all CRUD access for roles
  // since Teleport will perform these operations under the hood.
  const roleAccess = ctx.storeUser.getRoleAccess();
  const missingRoleAccess: RoleAccess[] = [
    { hasAccess: roleAccess.create, name: 'role.create' },
    { hasAccess: roleAccess.edit, name: 'role.update' },
    { hasAccess: roleAccess.list, name: 'role.list' },
    { hasAccess: roleAccess.read, name: 'role.read' },
    { hasAccess: roleAccess.remove, name: 'role.delete' },
  ].filter(rule => !rule.hasAccess);

  const hasRoleAccess = missingRoleAccess.length === 0;

  function onClickFlowKind(
    event: React.MouseEvent<HTMLDivElement>,
    preset: AccessListPreset
  ) {
    const targetElement = event.target as HTMLElement;
    if (targetElement.tagName === 'A') {
      // If user clicked on a link, do not set the preset type.
      return;
    }

    if (!featureLimitReached && hasRoleAccess) {
      setPreset(preset);
    }
  }

  return (
    <>
      <Flex gap={3} justifyContent="space-between">
        <H1 my={3}>Create a New Access List</H1>
        <InfoGuideButton config={{ guide: <InfoGuide /> }} />
      </Flex>

      {featureLimitReached && <FeatureLimitReached />}
      <Box style={featureLimitReached ? featureLimitReachedBlurCss : null}>
        <Text mb={3} mt={1}>
          Select the type of Access List you want to create:
        </Text>
        <Flex gap={3} flexWrap="wrap">
          <GuideCard
            onClick={
              hasRoleAccess ? e => onClickFlowKind(e, 'short-term') : undefined
            }
          >
            {hasRoleAccess && <RecommendedBadge>Recommended</RecommendedBadge>}
            {!hasRoleAccess && (
              <MissingRoleAccess missingRoleAccess={missingRoleAccess} />
            )}
            <HoverTooltip
              tipContent={
                <Box>
                  <Text mb={2}>
                    This guide will help you create an access list that will
                    require members of this list to request for access to the
                    resources that will be defined by this guide.
                  </Text>
                  <Text mb={2}>
                    After access request approval, members of this list will be
                    granted short term access to the resources requested.
                  </Text>
                  <Text>
                    Access duration will depend on the duration requested by
                    member in which maximum duration is capped to their
                    remaining web session TTL.
                  </Text>
                </Box>
              }
            >
              <GuideInnerCard hasAccess={hasRoleAccess}>
                <Text bold>Just-in-Time Access</Text>
                <Text>
                  A step-by-step guide on creating an access list that grants
                  members temporary access to Teleport resources via{' '}
                  <ExternalLink
                    target="_blank"
                    href="https://goteleport.com/docs/identity-governance/access-requests/"
                  >
                    access requests
                  </ExternalLink>
                  .
                </Text>
              </GuideInnerCard>
            </HoverTooltip>
          </GuideCard>

          <GuideCard
            onClick={
              hasRoleAccess ? e => onClickFlowKind(e, 'long-term') : undefined
            }
          >
            {!hasRoleAccess && (
              <MissingRoleAccess missingRoleAccess={missingRoleAccess} />
            )}
            <HoverTooltip
              tipContent={
                <Box>
                  <Text>
                    This guide will help you create an access list that will
                    give members of this list long-lived access to the resources
                    defined by this guide.
                  </Text>
                  <Text mt={3}>
                    Different from just-in-time access where members can access
                    resources immediately upon login.
                  </Text>
                </Box>
              }
            >
              <GuideInnerCard hasAccess={hasRoleAccess}>
                <Text bold>Standing Access</Text>
                <Text>
                  A step-by-step guide on creating an access list that grants
                  members long-lived access to Teleport resources.
                </Text>
              </GuideInnerCard>
            </HoverTooltip>
          </GuideCard>

          <GuideCard onClick={e => onClickFlowKind(e, '')}>
            <GuideInnerCard hasAccess={true}>
              <Text bold>Custom</Text>
              <Text>
                Provides an unguided form where you can define access in free
                form.
              </Text>
            </GuideInnerCard>
          </GuideCard>
        </Flex>

        <ButtonSecondary
          mt={4}
          as={Link}
          to={cfg.getAccessListManagementRoute()}
        >
          Back
        </ButtonSecondary>
      </Box>
    </>
  );
}

export function MissingRoleAccess({
  missingRoleAccess,
}: {
  missingRoleAccess: RoleAccess[];
}) {
  const missingAccessNames = missingRoleAccess.map(a => a.name);
  return (
    <Box zIndex={1} position="relative" css={{ cursor: 'default' }}>
      <ToolTipNoPermBadge>
        <Box>
          You cannot use this guide. Missing role permissions:{' '}
          <code>{missingAccessNames.join(', ')}</code>
        </Box>
      </ToolTipNoPermBadge>
    </Box>
  );
}

export const featureLimitReachedBlurCss: React.CSSProperties = {
  filter: 'blur(2px)',
  pointerEvents: 'none',
  userSelect: 'none',
  position: 'relative',
};

export const PresetTile = styled(Flex)`
  color: inherit;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  border-radius: ${({ theme }) => theme.radii[2]}px;
  padding: ${({ theme }) => theme.space[3]}px;
  gap: ${({ theme }) => theme.space[3]}px;
  height: 170px;
  width: 170px;
  background-color: ${({ theme }) => theme.colors.buttons.secondary.default};
  text-align: center;
  transition: background-color 200ms ease;

  &:hover,
  &:focus-visible {
    background-color: ${p => p.theme.colors.buttons.secondary.hover};
  }
`;

const GuideCard = styled.div`
  position: relative;

  border-radius: ${props => props.theme.radii[3]}px;
  transition: all 150ms;

  &:hover {
    background-color: ${props => props.theme.colors.levels.surface};

    // We use a pseudo element for the shadow with position: absolute in order
    // to prevent the shadow from increasing the size of the layout and causing
    // scrollbar flicker.
    &:after {
      box-shadow: ${props => props.theme.boxShadow[3]};
      border-radius: ${props => props.theme.radii[3]}px;
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      z-index: -1;
      width: 100%;
      height: 100%;
    }
  }
`;

const GuideInnerCard = styled.div<{ hasAccess?: boolean }>`
  display: inline-block;
  box-sizing: border-box;
  margin: 0;
  appearance: auto;
  text-align: left;

  height: 128px;
  width: 300px;

  border: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.spotBackground[0]};
  border-radius: ${props => props.theme.radii[3]}px;

  padding: 12px;
  color: ${props => props.theme.colors.text.main};
  line-height: inherit;
  font-size: inherit;
  font-family: inherit;
  cursor: ${props => (props.hasAccess ? 'pointer' : 'default')};

  opacity: ${props => (props.hasAccess ? '1' : '0.45')};

  &:focus-visible {
    outline: none;
    box-shadow: 0 0 0 3px ${props => props.theme.colors.brand};
  }

  &:hover,
  &:focus-visible {
    // Make the border invisible instead of removing it,
    // this is to prevent things from shifting due to the size change.
    border: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);
  }
`;

const RecommendedBadge = styled.div`
  position: absolute;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  padding: 0px 6px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: 0px;
  right: 0px;
  font-size: 10px;
  line-height: 18px;
`;

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
