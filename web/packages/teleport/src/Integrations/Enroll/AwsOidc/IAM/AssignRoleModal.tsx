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
import styled from 'styled-components';

import { AddIcon } from 'design/SVGIcon';

import { NextButton } from './common';

const Modal = styled.div`
  position: absolute;
  top: 0;
  bottom: 0;
  right: 0;
  left: 0;
  z-index: 9;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
`;

const AssignRole = styled.div<{ active: boolean }>`
  border-radius: 4px;
  border: 1px solid #bdbdbd;
  overflow: hidden;
  width: 500px;
`;

const AssignRoleClose = styled.div`
  transform: rotate(45deg);
  position: relative;
  top: 2px;
`;

const AssignRoleHeader = styled.div`
  background: #eee;
  display: flex;
  justify-content: space-between;
  font-size: 16px;
  padding: 10px 20px;
  border-bottom: 1px solid #dddddd;
`;

const AssignRoleFooter = styled.div`
  background: #eee;
  display: flex;
  justify-content: flex-end;
  font-size: 16px;
  padding: 10px 20px;
  border-top: 1px solid #dddddd;
`;

const AssignRoleContent = styled.div`
  background: white;
  padding: 10px 20px;
  display: flex;
  justify-content: space-between;
`;

const AssignRoleOption = styled.div<{ active: boolean }>`
  display: flex;
  background: ${p => (p.active ? '#f0f9ff' : 'white')};
  border: 1px solid ${p => (p.active ? '#99c2ee' : '#e6e6e6')};
  border-radius: 4px;
  flex: 0 0 200px;
  padding: 8px 12px 10px;
`;

const AssignRoleOptionBulletContainer = styled.div`
  flex: 0 0 25px;
  padding: 4px 0;
`;

const AssignRoleOptionBullet = styled.div<{ active: boolean }>`
  width: 14px;
  height: 14px;
  background: ${p => (p.active ? '#1066bb' : '#cccccc')};
  border-radius: 50%;
  position: relative;
  &:after {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: ${p => (p.active ? 6 : 11)}px;
    height: ${p => (p.active ? 6 : 11)}px;
    content: '';
    background: white;
    border-radius: 50%;
  }
`;

const CancelButton = styled.div`
  color: #1869bb;
  padding: 5px 15px;
  font-size: 14px;
  font-weight: 700;
  margin-right: 10px;
`;

const AssignRoleOptionDescription = styled.div`
  font-size: 12px;
  line-height: 1.2;
  color: #999;
`;

export function AssignRoleModal({
  clusterPublicUri,
}: {
  clusterPublicUri: string;
}) {
  return (
    <Modal>
      <div>
        <AssignRole>
          <AssignRoleHeader>
            Assign role for {clusterPublicUri}
            <AssignRoleClose>
              <AddIcon fill="#444444" size={16} />
            </AssignRoleClose>
          </AssignRoleHeader>
          <AssignRoleContent>
            <AssignRoleOption active={true}>
              <AssignRoleOptionBulletContainer>
                <AssignRoleOptionBullet active={true} />
              </AssignRoleOptionBulletContainer>
              <div>
                <div>Create a new role</div>
                <AssignRoleOptionDescription>
                  Create a new role for this identity provider and add
                  permissions to the new role.
                </AssignRoleOptionDescription>
              </div>
            </AssignRoleOption>
            <AssignRoleOption>
              <AssignRoleOptionBulletContainer>
                <AssignRoleOptionBullet active={false} />
              </AssignRoleOptionBulletContainer>
              <div>
                <div>Use an existing role</div>
                <AssignRoleOptionDescription>
                  Associate an existing role by adding this IdP to the trusted
                  entities of the role.
                </AssignRoleOptionDescription>
              </div>
            </AssignRoleOption>
          </AssignRoleContent>
          <AssignRoleFooter>
            <CancelButton>Cancel</CancelButton>
            <NextButton>Next</NextButton>
          </AssignRoleFooter>
        </AssignRole>
      </div>
    </Modal>
  );
}
