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

import { Flex } from 'design';
import Dialog, { DialogTitle } from 'design/Dialog';
import * as Icons from 'design/Icon';

import { TabIcon } from 'teleport/components/TabIcon';
import useTeleport from 'teleport/useTeleport';

import { Automatically } from './Automatically';
import { Manually } from './Manually';
import useAddApp, { State } from './useAddApp';

export default function Container(props: Props) {
  const ctx = useTeleport();
  const state = useAddApp(ctx);
  return <AddApp {...state} {...props} />;
}

export function AddApp({
  user,
  onClose,
  createToken,
  isEnterprise,
  version,
  attempt,
  automatic,
  setAutomatic,
  isAuthTypeLocal,
  token,
  labels,
  setLabels,
}: State & Props) {
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '600px',
        width: '100%',
        minHeight: '330px',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <Flex flex="1" flexDirection="column">
        <Flex alignItems="center" justifyContent="space-between" mb="4">
          <DialogTitle mr="auto">Add Application</DialogTitle>
          {isEnterprise && (
            <>
              <TabIcon
                Icon={Icons.Wand}
                title="Automatically"
                active={automatic}
                onClick={() => setAutomatic(true)}
              />
              <TabIcon
                Icon={Icons.Cog}
                title="Manually"
                active={!automatic}
                onClick={() => setAutomatic(false)}
              />
            </>
          )}
        </Flex>
        {automatic && (
          <Automatically
            onClose={onClose}
            onCreate={createToken}
            attempt={attempt}
            token={token}
            labels={labels}
            setLabels={setLabels}
          />
        )}
        {!automatic && (
          <Manually
            isAuthTypeLocal={isAuthTypeLocal}
            isEnterprise={isEnterprise}
            onClose={onClose}
            user={user}
            version={version}
            createToken={createToken}
            attempt={attempt}
            token={token}
          />
        )}
      </Flex>
    </Dialog>
  );
}

type Props = {
  onClose(): void;
};
