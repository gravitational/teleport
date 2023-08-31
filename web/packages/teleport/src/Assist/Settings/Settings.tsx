/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useRef, useState } from 'react';
import styled from 'styled-components';

import { CloseIcon, DisplayIcon, TerminalIcon } from 'design/SVGIcon';

import Flex from 'design/Flex';

import { AssistUserPreferences, ViewMode } from 'teleport/Assist/types';
import { useAssist } from 'teleport/Assist/context/AssistContext';
import { Tooltip } from 'teleport/Assist/shared/Tooltip';
import { HeaderIcon } from 'teleport/Assist/shared';

import { DisplaySettings } from 'teleport/Assist/Settings/DisplaySettings';

import { RemoteExecutionSettings } from 'teleport/Assist/Settings/RemoteExecutionSettings';
import { ErrorBanner, ErrorList } from 'teleport/Assist/ErrorBanner';
import { useUser } from 'teleport/User/UserContext';

interface SettingsProps {
  onClose: () => void;
  debugMenuEnabled: boolean;
  onDebugMenuToggle: (enabled: boolean) => void;
}

const Container = styled.div<{ viewMode: ViewMode }>`
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  right: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 9999;
`;

const SettingsContainer = styled.div`
  background: ${p => p.theme.colors.levels.popout};
  border-radius: 15px;
  box-shadow: 0 30px 60px 0 rgba(0, 0, 0, 0.4);
  width: 740px;
  height: 520px;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  overflow: hidden;
`;

const Sidebar = styled.ul`
  flex: 0 0 200px;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
  list-style: none;
  margin: 0;
  padding: 15px 10px 20px 0;
`;

const SettingsPage = styled.div`
  flex: 1;
  padding: 20px 20px;
  overflow-y: auto;
  height: 100%;
  box-sizing: border-box;
`;

const SidebarItemIcon = styled.div`
  flex: 0 0 28px;
  display: flex;
  align-items: center;
`;

const SidebarItem = styled.div`
  display: flex;
  align-items: center;
  margin-bottom: 5px;
  border-radius: 7px;
  padding: 5px 12px;
  cursor: pointer;
  font-weight: ${p => (p.active ? 600 : 400)};
  color: ${p => (p.active ? 'white' : p.theme.colors.text.primary)};
  background: ${p => (p.active ? p.theme.colors.brand : 'transparent')};

  ${SidebarItemIcon} {
    opacity: ${p => (p.active ? 1 : 0.7)};

    path {
      fill: ${p => (p.active ? 'white' : 0.7)};
    }
  }

  &:hover {
    background: ${p =>
      p.active ? p.theme.colors.brand : p.theme.colors.spotBackground[0]};
  }
`;

const Content = styled.div`
  display: flex;
  padding: 0 0 0 15px;
  flex: 1;
  min-height: 0;
`;

const DisplayIconContainer = styled.div`
  position: relative;
  top: 2px;
`;

const Header = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
  padding: 8px 15px;
`;

const Title = styled.h2`
  margin: 0;
  font-size: 16px;
`;

enum Page {
  Display,
  RemoteExecution,
}

export function Settings(props: SettingsProps) {
  const savingTimeoutRef = useRef<number | null>(null);

  const { toggleSidebar } = useAssist();
  const { preferences, updatePreferences } = useUser();

  const [saving, setSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [selectedPage, setSelectedPage] = useState(Page.Display);

  async function saveSettings(settings: Partial<AssistUserPreferences>) {
    setSaving(true);

    window.clearTimeout(savingTimeoutRef.current);

    try {
      await updatePreferences({ assist: settings });
    } catch {
      setErrorMessage('Failed to save settings');
    }

    // Stop "Saving..." from flickering really fast by showing it for at least half a second
    savingTimeoutRef.current = window.setTimeout(() => setSaving(false), 500);
  }

  function handleChangeViewMode(viewMode: ViewMode) {
    if (viewMode === preferences.assist.viewMode) {
      return;
    }

    toggleSidebar(viewMode === ViewMode.PopupExpandedSidebarVisible);
    saveSettings({ viewMode });
  }

  function handleChangePreferredLogins(preferredLogins: string[]) {
    // We can't compare the arrays directly because they will be different instances.
    // Instead, we stringify them to JSON (['foo','bar']) and compare the strings.
    if (
      JSON.stringify(preferredLogins) ===
      JSON.stringify(preferences.assist.preferredLogins)
    ) {
      return;
    }

    saveSettings({ preferredLogins });
  }

  function handleClose(event: React.MouseEvent) {
    event.stopPropagation();
    props.onClose();
  }

  return (
    <Container viewMode={preferences.assist.viewMode} onClick={handleClose}>
      <SettingsContainer onClick={e => e.stopPropagation()}>
        <Header>
          <Title>Teleport Assist Settings</Title>

          <Flex alignItems="center">
            {saving && <>Saving settings...</>}

            <HeaderIcon onClick={() => props.onClose()}>
              <CloseIcon size={24} />

              <Tooltip position="right">Close Settings</Tooltip>
            </HeaderIcon>
          </Flex>
        </Header>

        {errorMessage && (
          <ErrorList>
            <ErrorBanner onDismiss={() => setErrorMessage(null)}>
              There was an error saving the settings.
            </ErrorBanner>
          </ErrorList>
        )}

        <Content>
          <Sidebar>
            <SidebarItem
              active={selectedPage === Page.Display}
              onClick={() => setSelectedPage(Page.Display)}
            >
              <SidebarItemIcon>
                <DisplayIconContainer>
                  <DisplayIcon size={16} />
                </DisplayIconContainer>
              </SidebarItemIcon>
              Displays
            </SidebarItem>
            <SidebarItem
              active={selectedPage === Page.RemoteExecution}
              onClick={() => setSelectedPage(Page.RemoteExecution)}
            >
              <SidebarItemIcon>
                <TerminalIcon size={16} />
              </SidebarItemIcon>
              Remote execution
            </SidebarItem>
          </Sidebar>

          <SettingsPage>
            {selectedPage === Page.Display && (
              <DisplaySettings
                viewMode={preferences.assist.viewMode}
                onChange={handleChangeViewMode}
              />
            )}

            {selectedPage === Page.RemoteExecution && (
              <RemoteExecutionSettings
                preferredLogins={preferences.assist.preferredLogins}
                onChange={handleChangePreferredLogins}
              />
            )}
          </SettingsPage>
        </Content>
      </SettingsContainer>
    </Container>
  );
}
