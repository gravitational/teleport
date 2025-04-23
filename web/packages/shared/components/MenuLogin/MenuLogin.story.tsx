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

import { useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Flex, H3 } from 'design';

import { MenuLogin } from './MenuLogin';
import { LoginItem, MenuLoginHandle } from './types';

export default {
  title: 'Shared/MenuLogin',
};

export const MenuLoginStory = () => <MenuLoginExamples />;

function MenuLoginExamples() {
  return (
    <Flex
      inline
      p={4}
      gap="128px"
      justifyContent="flex-start"
      bg="levels.surface"
    >
      <Example>
        <H3>No logins</H3>
        <MenuLogin
          getLoginItems={() => []}
          onSelect={() => null}
          placeholder="Please provide user name…"
        />
      </Example>
      <Example>
        <H3>Processing state</H3>
        <MenuLogin
          getLoginItems={() => new Promise(() => {})}
          placeholder="MenuLogin in processing state"
          onSelect={() => null}
        />
      </Example>
      <Example>
        <H3>With logins</H3>
        <SampleMenu loginItems={loginItems} open />
      </Example>
      <Example>
        <H3>With a lot of logins</H3>
        <SampleMenu loginItems={aLotOfLoginItems} />
      </Example>
    </Flex>
  );
}

const Example = styled(Flex).attrs({
  gap: 2,
  flexDirection: 'column',
  alignItems: 'flex-start',
})``;

const SampleMenu = ({
  loginItems,
  open = false,
}: {
  loginItems: LoginItem[];
  open?: boolean;
}) => {
  const menuRef = useRef<MenuLoginHandle>();

  useEffect(() => {
    if (open) {
      menuRef.current.open();
    }
  }, [open]);

  return (
    <MenuLogin
      ref={menuRef}
      getLoginItems={() => loginItems}
      onSelect={() => null}
    />
  );
};

const makeLoginItem = (login: string) => ({ url: '', login });

const loginItems = ['root', 'jazrafiba', 'evubale', 'ipizodu'].map(
  makeLoginItem
);
const aLotOfLoginItems = [
  'root',
  'nyvpr@freire42.arg',
  'obo@qngnubfg.pb',
  'puneyvr@zlqbznva.bet',
  'qnir@erzbgrobk.pbz',
  'rir@flfgrzyvax.qri',
  'senax@pybhqlfcnpr.vb',
  'tenpr@grpuuho.hf',
  'unax@frpherybtva.ovm',
  'vil@argpbaarpg.gi',
  'wvyy@fnsrnpprff.ceb',
  'xra@erzbgryno.pb',
  'yran@qribcf.pybhq',
  'zvxr@ulcreabqr.bet',
  'avan@ybtzrva.klm',
  'bfpne@frpherubfg.arg',
  'cnhy@dhvpxpbaarpg.pb',
  'dhvaa@yvaxzr.ceb',
  'ehgu@snfgqngn.vb',
  'fgrir@npprffcbvag.qri',
  'gvan@pbzchgryvax.hf',
  'htb@frphercbeg.ovm',
  'iren@freirenpprff.gi',
  'jnyg@ybtvafgngvba.pbz',
  'kran@fnsrubfg.arg',
  'lhev@erzbgrfreire.pb',
  'mnen@pbaarpgyvax.vb',
  'nqnz@ploreabqr.pybhq',
  'orgu@flfgrztngr.bet',
  'pney@snfgybtva.ceb',
  'qvan@qngnjbeyq.ovm',
  'rq@ybtoevqtr.gi',
  'snl@frpherjnl.qri',
  'tvy@grpunpprff.hf',
  'uny@erzbgryvax.arg',
  'vqn@freirecbvag.pbz',
  'wnxr@pbaarpgceb.vb',
  'xnen@ybtfgngvba.bet',
  'yrb@npprffarg.pb',
  'znln@ploreyvax.gi',
  'abnu@erzbgrfcnpr.ovm',
  'bytn@frpherqngn.ceb',
  'crgr@dhvpxabqr.qri',
  'dhvaa@flfgrznpprff.hf',
  'eurn@ybtabqr.pbz',
  'fnen@erzbgrnpprff.arg',
  'gbz@pybhqfgngvba.pb',
  'hefhyn@ulcreyvax.vb',
  'ivp@frpheryvax.gi',
  'jvyy@freiretngr.ceb',
  'last@item.com',
].map(makeLoginItem);
