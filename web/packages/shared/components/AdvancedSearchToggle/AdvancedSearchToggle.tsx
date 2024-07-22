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

import React from 'react';

import { Text, Toggle, Link, Flex, H2 } from 'design';

import { ToolTipInfo } from 'shared/components/ToolTip';

const GUIDE_URL =
  'https://goteleport.com/docs/reference/predicate-language/#resource-filtering';

export function AdvancedSearchToggle(props: {
  isToggled: boolean;
  onToggle(): void;
  px?: number | string;
  gap?: number;
  className?: string;
}) {
  const gap = props.gap || 2;
  return (
    <Flex
      gap={gap}
      alignItems="center"
      px={props.px}
      className={props.className}
    >
      <Toggle isToggled={props.isToggled} onToggle={props.onToggle} />
      <Text typography="body2">Advanced</Text>
      <ToolTipInfo trigger="click">
        <PredicateDocumentation />
      </ToolTipInfo>
    </Flex>
  );
}

function PredicateDocumentation() {
  return (
    <>
      <Text typography="paragraph2" id="predicate-documentation">
        Advanced search allows you to perform more sophisticated searches using
        the predicate language. The language supports the basic operators:{' '}
        <Text as="span" bold>
          <code>==</code>{' '}
        </Text>
        ,{' '}
        <Text as="span" bold>
          <code>!=</code>
        </Text>
        ,{' '}
        <Text as="span" bold>
          <code>&&</code>
        </Text>
        , and{' '}
        <Text as="span" bold>
          <code>||</code>
        </Text>
      </Text>
      <H2 mt={2} mb={1}>
        Usage Examples
      </H2>
      <Text typography="paragraph2">
        Label Matching:{' '}
        <Text ml={1} as="span" bold>
          <code>labels["key"] == "value" && labels["key2"] != "value2"</code>{' '}
        </Text>
        <br />
        Fuzzy Searching:{' '}
        <Text ml={1} as="span" bold>
          <code>search("foo", "bar", "some phrase")</code>
        </Text>
        <br />
        Combination:{' '}
        <Text ml={1} as="span" bold>
          <code>labels["key1"] == "value1" && search("foo")</code>
        </Text>
      </Text>
      <Text typography="paragraph2" mt={2}>
        Check out our{' '}
        <Link href={GUIDE_URL} target="_blank">
          predicate language guide
        </Link>{' '}
        for a more in-depth explanation of the language.
      </Text>
    </>
  );
}
