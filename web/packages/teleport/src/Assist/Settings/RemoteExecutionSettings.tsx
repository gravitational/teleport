/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import Select, { Option } from 'shared/components/Select';
import { CloseIcon } from 'design/SVGIcon';

import { DraggableList } from 'design/DraggableList';

import { Description, Title } from 'teleport/Assist/Settings/shared';

import { getNodesFromQuery } from 'teleport/Assist/service';
import useStickyClusterId from 'teleport/useStickyClusterId';

import type { Node } from 'teleport/services/nodes/types';

interface RemoteExecutionSettingsProps {
  preferredLogins: string[];
  onChange: (preferredLogins: string[]) => void;
}

const Login = styled.div`
  display: flex;
  justify-content: space-between;
  background: ${p => p.theme.colors.spotBackground[0]};
  width: 420px;
  padding: 10px 15px;
  border-radius: 5px;
  overflow: hidden;
  user-select: none;
  cursor: grab;
  position: relative;
  height: 40px;
  box-sizing: border-box;
`;

const DeleteButton = styled.div`
  cursor: pointer;
  padding: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: ${p => p.theme.colors.spotBackground[0]};
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;

  &:hover {
    background: ${p => p.theme.colors.error.main};

    svg path {
      stroke: white;
    }
  }
`;

const ErrorMessage = styled.div`
  color: ${p => p.theme.colors.error.main};
`;

const Loading = styled.div`
  color: ${props => props.theme.colors.text.muted};
`;

const LoginContainer = styled.div`
  height: 50px;
`;

function getLoginsForNodes(nodes: Node[]) {
  const logins = new Set<string>();

  for (const node of nodes) {
    for (const login of node.sshLogins) {
      logins.add(login);
    }
  }

  return Array.from(logins);
}

export function RemoteExecutionSettings(props: RemoteExecutionSettingsProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [logins, setLogins] = useState(props.preferredLogins);
  const [, setCount] = useState(0);

  const order = useRef<number[]>(logins.map((_, index) => index));

  const [options, setOptions] = useState<Option[]>([]);

  const { clusterId } = useStickyClusterId();

  async function fetchLogins() {
    try {
      const { agents } = await getNodesFromQuery('', clusterId);

      const logins = getLoginsForNodes(agents)
        .filter(login => !props.preferredLogins.includes(login))
        .map(login => ({ label: login, value: login }));

      setOptions(logins);
    } catch {
      setError(true);
    }

    setLoading(false);
  }

  useEffect(() => {
    fetchLogins();
  }, []);

  const loginsByOrder = order.current.map(index => logins[index]);

  function handleReorder(newOrder: number[]) {
    order.current = newOrder;

    props.onChange(order.current.map(index => logins[index]));

    // force a re-render to update the number prefixed to the login (1., 2., etc.)
    setCount(count => count + 1);
  }

  function onChangeOption(e: Option) {
    const newLogins = [...logins, e.value];

    order.current = [...order.current, newLogins.length - 1];

    setOptions(options.filter(o => o.value !== e.value));
    setLogins(newLogins);

    props.onChange(order.current.map(index => newLogins[index]));
  }

  function handleRemove(login: string) {
    const newLogins = logins.filter(l => l !== login);

    order.current = newLogins.map((_, index) => index);

    setLogins(newLogins);
    setOptions([...options, { label: login, value: login }]);

    props.onChange(order.current.map(index => newLogins[index]));
  }

  let content;
  if (error) {
    content = (
      <ErrorMessage>
        There was an issue fetching the available logins.
      </ErrorMessage>
    );
  } else if (loading) {
    content = <Loading>Loading logins...</Loading>;
  } else {
    content = (
      <>
        <div style={{ marginBottom: 20 }}>
          <Select
            placeholder="Select a login to add..."
            value={null}
            onChange={onChangeOption}
            options={options}
            hasError={false}
            maxMenuHeight={400}
            isSearchable
            isSimpleValue={false}
            isClearable={false}
          />
        </div>

        <DraggableList onOrderChange={handleReorder}>
          {logins.map(item => (
            <LoginContainer key={item}>
              <Login>
                <div>
                  {loginsByOrder.indexOf(item) + 1}. {item}
                </div>

                <DeleteButton onClick={() => handleRemove(item)}>
                  <CloseIcon size={16} />
                </DeleteButton>
              </Login>
            </LoginContainer>
          ))}
        </DraggableList>
      </>
    );
  }

  return (
    <div>
      <Title>Preferred logins</Title>

      <Description>
        Set a list of preferred logins to be used when running remote commands
        on nodes. Assist will use the first login in this list if it can be used
        to connect to all the nodes.
      </Description>

      <Description>
        Hint: drag and drop the logins to change the order of preference.
      </Description>

      {content}
    </div>
  );
}
