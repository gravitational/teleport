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

import React, { useEffect, useState } from 'react';
import { useAttemptNext } from 'shared/hooks';
import { Link } from 'react-router-dom';
import { HoverTooltip } from 'shared/components/ToolTip';
import { Alert, Box, ButtonPrimary, Indicator } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { BotList } from 'teleport/Bots/List/BotList';
import {
  deleteBot,
  editBot,
  fetchBots,
  fetchRoles,
} from 'teleport/services/bot/bot';
import { FlatBot } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import cfg from 'teleport/config';

export function Bots() {
  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasAddBotPermissions = flags.addBots;

  const [bots, setBots] = useState<FlatBot[]>();
  const [selectedBot, setSelectedBot] = useState<FlatBot>();
  const [selectedRoles, setSelectedRoles] = useState<string[]>();
  const { attempt: crudAttempt, run: crudRun } = useAttemptNext();
  const { attempt: fetchAttempt, run: fetchRun } = useAttemptNext('processing');

  useEffect(() => {
    const signal = new AbortController();
    const flags = ctx.getFeatureFlags();

    async function bots(signal: AbortSignal) {
      return await fetchBots(signal, flags);
    }

    fetchRun(() =>
      bots(signal.signal).then(botRes => {
        setBots(botRes.bots);
      })
    );
    return () => {
      signal.abort();
    };
  }, [ctx, fetchRun]);

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
            <ButtonPrimary
              ml="auto"
              width="240px"
              as={Link}
              to={cfg.getBotsNewRoute()}
              disabled={!hasAddBotPermissions}
            >
              Enroll New Bot
            </ButtonPrimary>
          </HoverTooltip>
        </Box>
      </FeatureHeader>
      {fetchAttempt.status == 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
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
