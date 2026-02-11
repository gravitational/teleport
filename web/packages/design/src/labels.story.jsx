/*
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

import Flex from './Flex';
import { CircleCheck, Cross, Plus } from './Icon';
import Label from './Label';
import { LabelButtonWithIcon } from './Label/LabelButtonWithIcon';
import LabelState from './LabelState';

export default {
  title: 'Design/Label',
};

export const Labels = () => (
  <>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
      gap={4}
    >
      <Label kind="primary">primary</Label>
      <Label kind="secondary">secondary</Label>
      <Label kind="warning">warning</Label>
      <Label kind="danger">danger</Label>
      <Label kind="success">success</Label>
    </Flex>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
      gap={4}
    >
      <Label kind="primary" css={{ visibility: 'hidden' }}>
        Primary
      </Label>
      <Label kind="outline-primary" withHoverState>
        outline-primary
      </Label>
      <Label kind="outline-secondary" withHoverState>
        outline-secondary
      </Label>
      <Label kind="outline-success" withHoverState>
        outline-success
      </Label>
      <Label kind="outline-warning" withHoverState>
        outline-warning
      </Label>
      <Label kind="outline-danger" withHoverState>
        outline-danger
      </Label>
      <Label kind="success" css={{ visibility: 'hidden' }}>
        Success
      </Label>
    </Flex>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
      gap={4}
    >
      <LabelState kind="success">Success</LabelState>
      <LabelState kind="secondary">Secondary</LabelState>
      <LabelState kind="warning">Warning</LabelState>
      <LabelState kind="danger">Danger</LabelState>
    </Flex>
    <Flex
      height="100px"
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
      gap={4}
    >
      <LabelState shadow kind="success">
        Success
      </LabelState>
      <LabelState shadow kind="secondary">
        Secondary
      </LabelState>
      <LabelState shadow kind="warning">
        Warning
      </LabelState>
      <LabelState shadow kind="danger">
        Danger
      </LabelState>
    </Flex>

    <Flex
      pb={4}
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
      gap={4}
      flexWrap={'wrap'}
    >
      <LabelButtonWithIcon
        kind="outline-primary"
        IconLeft={Plus}
        withHoverState
      >
        LabelButtonWithIcon: outline-primary
      </LabelButtonWithIcon>
      <LabelButtonWithIcon
        kind="outline-secondary"
        IconRight={Cross}
        withHoverState
      >
        LabelButtonWithIcon: outline-secondary
      </LabelButtonWithIcon>
      <LabelButtonWithIcon
        kind="outline-warning"
        IconRight={Cross}
        withHoverState
      >
        LabelButtonWithIcon: outline-warning
      </LabelButtonWithIcon>
      <LabelButtonWithIcon
        kind="outline-danger"
        IconRight={Cross}
        withHoverState
      >
        LabelButtonWithIcon: outline-danger
      </LabelButtonWithIcon>
      <LabelButtonWithIcon
        kind="outline-success"
        IconLeft={CircleCheck}
        withHoverState
      >
        LabelButtonWithIcon: outline-success
      </LabelButtonWithIcon>
    </Flex>

    <Flex
      pb={4}
      bg="levels.surface"
      justifyContent="center"
      alignItems="center"
      gap={4}
      flexWrap={'wrap'}
    >
      <LabelButtonWithIcon kind="primary" IconLeft={Plus} withHoverState>
        LabelButtonWithIcon: primary
      </LabelButtonWithIcon>
      <LabelButtonWithIcon kind="secondary" IconRight={Cross} withHoverState>
        LabelButtonWithIcon: secondary
      </LabelButtonWithIcon>
      <LabelButtonWithIcon kind="warning" IconRight={Cross} withHoverState>
        LabelButtonWithIcon: warning
      </LabelButtonWithIcon>
      <LabelButtonWithIcon kind="danger" IconRight={Cross} withHoverState>
        LabelButtonWithIcon: danger
      </LabelButtonWithIcon>
      <LabelButtonWithIcon kind="success" IconLeft={CircleCheck} withHoverState>
        LabelButtonWithIcon: success
      </LabelButtonWithIcon>
    </Flex>
  </>
);
