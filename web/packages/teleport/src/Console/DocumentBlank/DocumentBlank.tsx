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

import { ButtonPrimary, Flex } from 'design';
import * as Icons from 'design/Icon';

import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import * as stores from 'teleport/Console/stores';

import Document from './../Document';

export default function DocumentBlank(props: PropTypes) {
  const { visible, doc } = props;
  const ctx = useConsoleContext();

  function onClick() {
    ctx.gotoNodeTab(doc.clusterId);
  }

  return (
    <Document visible={visible}>
      <Flex flexDirection="column" alignItems="center" flex="1">
        <Icons.Cli
          size={256}
          mt={10}
          mb={6}
          css={`
            color: ${props => props.theme.colors.spotBackground[1]};
          `}
        />
        <ButtonPrimary onClick={onClick} children="Start a New Session" />
      </Flex>
    </Document>
  );
}

type PropTypes = {
  visible: boolean;
  doc: stores.DocumentBlank;
};
