import { useMemo } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';
import { getPlatform } from 'design/platform';

import { FeatureBox } from 'teleport/components/Layout/Layout';
import { useNoMinWidth } from 'teleport/Main';
import useTeleport from 'teleport/useTeleport';

import { SectionFrame } from './components/SectionFrame';
import { StepCard } from './components/StepCard';
import { TableOfContents } from './components/TableOfContents';
import { buildSections, pickDefaultAuth } from './contents';
import { useHashNavigation } from './useHashNavigation';
import { useScrollSpy } from './useScrollSpy';

export function BeamsQuickstart() {
  const { storeUser } = useTeleport();
  useNoMinWidth();

  const clusterPublicUrl = storeUser.getClusterPublicUrl();
  const clusterVersion = storeUser.getClusterAuthVersion();
  const username = storeUser.getUsername();
  const defaultAuth = pickDefaultAuth(storeUser);
  const platform = getPlatform();

  const sections = useMemo(
    () =>
      buildSections({
        clusterPublicUrl,
        clusterVersion,
        username,
        defaultAuth,
        platform,
      }),
    [clusterPublicUrl, clusterVersion, username, defaultAuth, platform]
  );
  const sectionIds = useMemo(() => sections.map(s => s.id), [sections]);

  const active = useScrollSpy(sectionIds);
  const onSelect = useHashNavigation(sectionIds, active);

  return (
    <FeatureBox>
      <Layout>
        <TableOfContents
          sections={sections}
          activeId={active}
          onSelect={onSelect}
        />
        <Content>
          {sections.map(s => (
            <SectionFrame
              key={s.id}
              id={s.id}
              title={s.title}
              active={active === s.id}
            >
              {s.cards.map((c, i) => (
                <StepCard key={c.id ?? c.steps?.[0]?.eyebrow ?? i} card={c} />
              ))}
            </SectionFrame>
          ))}
        </Content>
      </Layout>
    </FeatureBox>
  );
}

const Layout = styled(Flex).attrs({ gap: 9, alignItems: 'flex-start' })`
  max-width: 1104px;
  margin: 0 auto;
  width: 100%;
  @media (max-width: ${p => p.theme.breakpoints.medium}) {
    gap: 0;
  }
`;

const Content = styled(Flex).attrs({ flexDirection: 'column', gap: 7 })`
  flex: 1;
  min-width: 0;
`;
