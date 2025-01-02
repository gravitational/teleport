import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Box, ButtonPrimary, Flex, H1, Text } from 'design';

import cfg from 'e-teleport/config';

import {
  AchieveCompliance,
  AchieveCompliancePreview,
} from './AchieveCompliance';
import {
  AutomateAccessRequest,
  AutomateAccessRequestPreview,
} from './AutomateAccessRequest';
import { GetVisibility, GetVisibilityPreview } from './GetVisibility';
import {
  ReduceAttackSurface,
  ReduceAttackSurfacePreview,
} from './ReduceAttackSurface';

export function EmptyState() {
  const [currIndex, setCurrIndex] = useState(0);
  const [intervalId, setIntervalId] = useState<any>();

  function handleOnClick(clickedIndex) {
    clearInterval(intervalId);
    setCurrIndex(clickedIndex);
    setIntervalId(null);
  }

  useEffect(() => {
    const id = setInterval(() => {
      setCurrIndex(latestIndex => (latestIndex + 1) % 4);
    }, 3000);
    setIntervalId(id);
    return () => clearInterval(id);
  }, []);

  return (
    <Box mt={4}>
      <Box mb={3}>
        <H1 mb={3}>What are Access Lists?</H1>
        <Text css={{ maxWidth: '1204px' }}>
          <b>Access Lists</b> enable users to gain long-term access to select
          resources within Teleport. List Owners can manage membership and
          conduct periodic audits for compliance needs. Users can create Access
          Lists around specific resources for fine-grained control, or create
          lists around teams and functions to align access to an org’s
          structure. See below for some benefits:
        </Text>
      </Box>
      <FeatureContainer py={2} pr={2}>
        <Box css={{ position: 'relative' }}>
          <FeatureSlider $currIndex={currIndex} />
          <AutomateAccessRequest
            active={currIndex === 0}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(0)}
          />
          <ReduceAttackSurface
            active={currIndex === 1}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(1)}
          />
          <AchieveCompliance
            active={currIndex === 2}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(2)}
          />
          <GetVisibility
            active={currIndex === 3}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(3)}
          />
        </Box>
        <Box>
          {currIndex === 0 && <AutomateAccessRequestPreview />}
          {currIndex === 1 && <ReduceAttackSurfacePreview />}
          {currIndex === 2 && <AchieveCompliancePreview />}
          {currIndex === 3 && <GetVisibilityPreview />}
        </Box>
      </FeatureContainer>
      <Box width="100%" textAlign="center" mt={5}>
        <ButtonPrimary
          width="280px"
          as={Link}
          to={cfg.routes.accessListNew}
          size="large"
        >
          Create Your First Access List
        </ButtonPrimary>
      </Box>
    </Box>
  );
}

const FeatureContainer = styled(Flex)`
  @media (min-width: 1662px) {
    --feature-slider-width: 612px;
    --feature-width: 612px;
    --feature-height: 95px;
    --feature-preview-scale: scale(0.9);
    --feature-text-display: block;
  }

  @media (max-width: 1662px) {
    --feature-slider-width: 512px;
    --feature-width: 512px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.9);
    --feature-text-display: block;
  }

  @media (max-width: 1563px) {
    --feature-slider-width: 412px;
    --feature-width: 412px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.8);
    --feature-text-display: inline;
  }

  @media (max-width: 1462px) {
    --feature-slider-width: 412px;
    --feature-width: 412px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.8);
    --feature-text-display: inline;
  }

  @media (max-width: 1302px) {
    --feature-slider-width: 372px;
    --feature-width: 372px;
    --feature-height: 112px;
    --feature-preview-scale: scale(0.7);
    --feature-text-display: inline;
  }
`;

const FeatureSlider = styled.div<{ $currIndex: number }>`
  z-index: -1;
  position: absolute;
  height: var(--feature-height);
  width: var(--feature-slider-width);

  transition: all 0.3s ease;
  border-radius: ${p => p.theme.radii[3]}px;
  cursor: pointer;

  top: calc(var(--feature-height) * ${p => p.$currIndex});

  background-color: ${p => p.theme.colors.interactive.tonal.primary[0]};
`;
