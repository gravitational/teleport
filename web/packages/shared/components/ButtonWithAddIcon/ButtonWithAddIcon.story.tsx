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

import { useState } from 'react';

import { ButtonBorder, ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';

import { ButtonWithAddIcon } from './ButtonWithAddIcon';

export default {
  title: 'Shared/ButtonWithAddIcon',
};

export const Button = () => {
  const [label, setLabel] = useState('Add Item (click me)');
  return (
    <Flex gap={2} width="300px" flexWrap={'wrap'}>
      <div>
        <ButtonWithAddIcon label={'Add Item'} onClick={() => null} />
      </div>
      <div>
        <ButtonWithAddIcon
          label={label}
          onClick={() => setLabel('Add More Item (click me)')}
        />
      </div>
      <div>
        <ButtonWithAddIcon
          label={'Add Item Disabled'}
          onClick={() => null}
          disabled={true}
        />
      </div>
      <div>
        <ButtonWithAddIcon
          label={'Add Item with Medium Icon Size'}
          onClick={() => null}
          iconSize={'medium'}
        />
      </div>
      <div>
        <ButtonWithAddIcon
          Button={ButtonPrimary}
          label={'Add Item'}
          onClick={() => null}
        />
      </div>
      <div>
        <ButtonWithAddIcon
          Button={ButtonSecondary}
          label={'Add Item'}
          onClick={() => null}
        />
      </div>
      <div>
        <ButtonWithAddIcon
          Button={ButtonBorder}
          label={'Add Item'}
          onClick={() => null}
        />
      </div>
    </Flex>
  );
};

Button.storyName = 'ButtonWithAddIcon';
