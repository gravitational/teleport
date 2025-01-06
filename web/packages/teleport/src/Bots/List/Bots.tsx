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

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';

import { Alert, Box, Button, Indicator } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { useAttemptNext } from 'shared/hooks';

import { BotList } from 'teleport/Bots/List/BotList';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import {
  deleteBot,
  editBot,
  fetchBots,
  fetchRoles,
} from 'teleport/services/bot/bot';
import { FlatBot } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { EmptyState } from './EmptyState/EmptyState';

export function Bots() {
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasAddBotPermissions = flags.addBots;
  const canListBots = flags.listBots;

  const [bots, setBots] = useState<FlatBot[]>([]);
  const [selectedBot, setSelectedBot] = useState<FlatBot>();
  const [selectedRoles, setSelectedRoles] = useState<string[]>();
  const { attempt: crudAttempt, run: crudRun } = useAttemptNext();
  const { attempt: fetchAttempt, run: fetchRun } = useAttemptNext(
    canListBots ? 'processing' : 'success'
  );

  useEffect(() => {
    const signal = new AbortController();
    const flags = ctx.getFeatureFlags();

    async function bots(signal: AbortSignal) {
      return await fetchBots(signal, flags);
    }

    if (canListBots) {
      fetchRun(() =>
        bots(signal.signal).then(botRes => {
          setBots(botRes.bots);
        })
      );
    }
    return () => {
      signal.abort();
    };
  }, [ctx, fetchRun, canListBots]);

  async function fetchRoleNames(search: string): Promise<string[]> {
    const flags = ctx.getFeatureFlags();
    const roles = await fetchRoles(search, flags);
    return roles.items.map(r => r.name);
  }

  function onDelete() {
    crudRun(() => deleteBot(flags, selectedBot.name)).then(() => {
      setBots(bots.filter(bot => bot.name !== selectedBot.name));
      onClose();
    });
  }

  function onEdit() {
    crudRun(() =>
      editBot(flags, selectedBot.name, { roles: selectedRoles }).then(
        (updated: FlatBot) => {
          const updatedList: FlatBot[] = bots.map((item: FlatBot): FlatBot => {
            if (item.name !== selectedBot.name) {
              return item;
            }
            return {
              ...item,
              ...updated,
            };
          });

          setBots(updatedList);
          onClose();
        }
      )
    );
  }

  function onClose() {
    setSelectedBot(null);
    setSelectedRoles(null);
  }

  if (fetchAttempt.status === 'processing') {
    return (
      <FeatureBox>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </FeatureBox>
    );
  }

  if (fetchAttempt.status === 'success' && bots.length === 0) {
    return (
      <FeatureBox>
        {!canListBots && (
          <Alert kind="info" mt={4}>
            You do not have permission to access Bots. Missing role permissions:{' '}
            <code>bot.list</code>
          </Alert>
        )}
        <EmptyState />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Bots</FeatureHeaderTitle>
        <Box ml="auto">
          <HoverTooltip
            tipContent={
              hasAddBotPermissions
                ? ''
                : `Insufficient permissions. Reach out to your Teleport administrator
    to request bot creation permissions.`
            }
          >
            <Button
              intent="primary"
              fill={
                fetchAttempt.status === 'success' && bots.length === 0
                  ? 'filled'
                  : 'border'
              }
              ml="auto"
              width="240px"
              as={Link}
              to={cfg.getBotsNewRoute()}
              disabled={!hasAddBotPermissions}
            >
              Enroll New Bot
            </Button>
          </HoverTooltip>
        </Box>
      </FeatureHeader>
      {fetchAttempt.status == 'failed' && (
        <Alert kind="danger" children={fetchAttempt.statusText} />
      )}
      {fetchAttempt.status == 'success' && (
        <BotList
          attempt={crudAttempt}
          bots={bots}
          disabledEdit={!flags.roles || !flags.editBots}
          disabledDelete={!flags.removeBots}
          fetchRoles={fetchRoleNames}
          onClose={onClose}
          onDelete={onDelete}
          onEdit={onEdit}
          selectedBot={selectedBot}
          setSelectedBot={setSelectedBot}
          selectedRoles={selectedRoles}
          setSelectedRoles={setSelectedRoles}
        />
      )}
    </FeatureBox>
  );
}
