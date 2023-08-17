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

import React from 'react';

import { AddIcon, LockIcon } from 'design/SVGIcon';

import {
  BrowserContainer,
  BrowserContentContainer,
  BrowserTab,
  BrowserTabClose,
  BrowserTabFavicon,
  BrowserTabs,
  BrowserTabTitle,
  BrowserTitleBarButton,
  BrowserTitleBarButtons,
  BrowserTitleBarContainer,
  BrowserURL,
  BrowserURLContainer,
  BrowserURLIcon,
} from 'teleport/Integrations/Enroll/AwsOidc/browser/BrowserComponents';

import { Stage } from 'teleport/Integrations/Enroll/AwsOidc/stages';

interface BrowserProps {
  stage: Stage;
}

export function Browser(props: React.PropsWithChildren<BrowserProps>) {
  const tabs = [];
  if (props.stage >= Stage.CreateNewRole) {
    tabs.push(
      <BrowserTab
        active={
          props.stage <= Stage.ClickCreatePolicy ||
          props.stage >= Stage.AssignPolicyToRole
        }
        key="create-role"
      >
        <BrowserTabFavicon />
        <BrowserTabTitle>
          {props.stage >= Stage.ListRoles ? 'Roles' : 'Create role'}
        </BrowserTabTitle>
        <BrowserTabClose>
          <AddIcon size={14} />
        </BrowserTabClose>
      </BrowserTab>
    );
  }

  if (
    props.stage >= Stage.CreatePolicy &&
    props.stage <= Stage.ClickCreatePolicyButton
  ) {
    tabs.push(
      <BrowserTab active={true} key="create-policy">
        <BrowserTabFavicon />
        <BrowserTabTitle>Create policy</BrowserTabTitle>
        <BrowserTabClose>
          <AddIcon size={14} />
        </BrowserTabClose>
      </BrowserTab>
    );
  }

  //<BrowserTab active={true}>
  //       <BrowserTabFavicon />
  //       <BrowserTabTitle>Create policy</BrowserTabTitle>
  //       <BrowserTabClose>
  //         <AddIcon size={14} />
  //       </BrowserTabClose>
  //     </BrowserTab>

  return (
    <BrowserContainer>
      <BrowserTitleBarContainer>
        <BrowserTitleBarButtons>
          <BrowserTitleBarButton style={{ backgroundColor: '#f95e57' }} />
          <BrowserTitleBarButton style={{ backgroundColor: '#fbbe2e' }} />
          <BrowserTitleBarButton style={{ backgroundColor: '#31c842' }} />
        </BrowserTitleBarButtons>

        <BrowserURLContainer>
          <BrowserURL>
            <BrowserURLIcon>
              <LockIcon fill="white" size={12} />
            </BrowserURLIcon>
            console.aws.amazon.com
          </BrowserURL>
        </BrowserURLContainer>
      </BrowserTitleBarContainer>

      <BrowserTabs>
        <BrowserTab>
          <BrowserTabFavicon />
          <BrowserTabTitle>IAM Management Console</BrowserTabTitle>
          <BrowserTabClose>
            <AddIcon size={14} />
          </BrowserTabClose>
        </BrowserTab>
        {tabs}
      </BrowserTabs>

      <BrowserContentContainer>{props.children}</BrowserContentContainer>
    </BrowserContainer>
  );
}
