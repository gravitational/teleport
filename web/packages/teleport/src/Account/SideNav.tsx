/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useHistory } from 'react-router';
import { useLocation } from 'react-router-dom';
import styled from 'styled-components';

import cfg from 'teleport/config';

import { preferencesHeadings } from './Preferences';
import { securityHeadings } from './SecuritySettings';

export interface Heading {
  name: string;
  id: string;
}

export type Headings = Heading[];

export interface SideNavProps {
  recoveryEnabled?: boolean;
  trustedDevicesEnabled?: boolean;
}

export function SideNav({
  recoveryEnabled = false,
  trustedDevicesEnabled = false,
}: SideNavProps) {
  const history = useHistory();
  const location = useLocation();

  const navigateTo = (path: string) => {
    const [basePath, idToScrollTo] = path.split('#');
    const isSamePage = location.pathname === basePath;

    if (isSamePage) {
      window.history.replaceState(null, '', path);
    } else {
      history.replace(path);
    }

    // If there's an ID found, scroll to it
    if (idToScrollTo) {
      // setTimeout is used to ensure the DOM is rendered before
      // trying to scroll to it. The DOM re-renders if the user
      // clicks on a subheading that is on a different page.
      setTimeout(() => {
        const element = document.getElementById(idToScrollTo);
        if (element) {
          element.scrollIntoView({ behavior: 'smooth' });
        }
      }, 0);
    }
  };

  const navItems = generateHeadings(recoveryEnabled, trustedDevicesEnabled);

  return (
    <SideNavWrapper>
      {navItems.map(group => {
        // Check if this section is active based on the current path
        const isSectionActive = location.pathname === group.page.link;

        return (
          <div key={group.page.name}>
            <SectionTitle
              className={isSectionActive ? 'active' : ''}
              onClick={() => navigateTo(group.page.link)}
            >
              {group.page.name}
            </SectionTitle>
            <LinkList>
              {group.headings.map(heading => (
                <li key={heading.name}>
                  <HeadingItem
                    href={`${group.page.link}#${heading.id}`}
                    onClick={e => {
                      e.preventDefault();
                      navigateTo(`${group.page.link}#${heading.id}`);
                    }}
                  >
                    {heading.name}
                  </HeadingItem>
                </li>
              ))}
            </LinkList>
          </div>
        );
      })}
    </SideNavWrapper>
  );
}

function generateHeadings(
  recoveryEnabled: boolean,
  trustedDevicesEnabled: boolean
) {
  const secHeadings = securityHeadings();

  if (recoveryEnabled) {
    secHeadings.push({ name: 'Recovery Code', id: 'recovery-code' });
  }

  if (trustedDevicesEnabled) {
    secHeadings.push({ name: 'Trusted Devices', id: 'trusted-devices' });
  }

  const prefHeadings = preferencesHeadings();

  return [
    {
      page: { name: 'Security', link: cfg.routes.accountSecurity },
      headings: secHeadings,
    },
    {
      page: { name: 'Preferences', link: cfg.routes.accountPreferences },
      headings: prefHeadings,
    },
  ];
}

// TODO(danielashare): Have the side nav move down the page as the user scrolls.
//                     Not needed currently as there aren't many elements.
const SideNavWrapper = styled.aside`
  display: flex;
  flex-direction: column;
  width: 100%;
`;

const SectionTitle = styled.div`
  display: flex;
  flex-basis: 100%;
  padding-top: ${p => p.theme.space[2]}px;
  padding-bottom: ${p => p.theme.space[2]}px;
  padding-left: ${p => p.theme.space[2]}px;
  border-radius: ${p => p.theme.radii[2]}px;
  cursor: pointer;
  text-decoration: none;
  color: ${p => p.theme.colors.text.main};
  font-weight: ${p => p.theme.bold};
  font-size: ${p => p.theme.fontSizes[3]}px;

  &.active {
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
    border-left: ${p => p.theme.borders[3]}
      ${p => p.theme.colors.interactive.solid.primary.default};
    padding-left: ${p => p.theme.space[1]}px;
  }

  &:hover:not(.active) {
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }
`;

const LinkList = styled.ul`
  list-style: none;
  padding: ${p => p.theme.space[0]}px;
  margin: ${p => p.theme.space[0]}px;
`;

const HeadingItem = styled.a`
  text-decoration: none;
  display: flex;
  flex-basis: 100%;
  padding-top: ${p => p.theme.space[2]}px;
  padding-bottom: ${p => p.theme.space[2]}px;
  padding-left: ${p => p.theme.space[4]}px;
  color: ${p => p.theme.colors.text.slightlyMuted};

  &:hover {
    color: ${p => p.theme.colors.text.main};
  }
`;
