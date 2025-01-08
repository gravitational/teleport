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

import { Box, Text } from '..';
import typography from './typography';

export default {
  title: 'Design/Theme/Typography',
};

export const Typography = () => (
  <>
    <Specs />
    <Example />
  </>
);

const Specs = () => (
  <Box bg="levels.surface" p={2} textAlign="left">
    <Text typography="h2" mb={3}>
      Specs
    </Text>
    <table css={tableCss}>
      <thead>
        <tr>
          <th width="100">Name</th>
          <th width="100">Size</th>
          <th>Weight</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>
            <Text typography="h1">H1</Text>
          </td>
          <td>
            {typography.h1.fontSize}/{typography.h1.lineHeight}
          </td>
          <td>{typography.h1.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="h2">H2</Text>
          </td>
          <td>
            {typography.h2.fontSize}/{typography.h2.lineHeight}
          </td>
          <td>{typography.h2.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="h3">H3</Text>
          </td>
          <td>
            {typography.h3.fontSize}/{typography.h3.lineHeight}
          </td>
          <td>{typography.h3.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="h4">H4</Text>
          </td>
          <td>
            {typography.h4.fontSize}/{typography.h4.lineHeight}
          </td>
          <td>{typography.h4.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="body1">Body1</Text>
          </td>
          <td>
            {typography.body1.fontSize}/{typography.body1.lineHeight}
          </td>
          <td>{typography.body1.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="body2">Body2</Text>
          </td>
          <td>
            {typography.body2.fontSize}/{typography.body2.lineHeight}
          </td>
          <td>{typography.body2.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="subtitle1">SubTitle1</Text>
          </td>
          <td>
            {typography.subtitle1.fontSize}/{typography.subtitle1.lineHeight}
          </td>
          <td>{typography.subtitle1.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="subtitle2">subtitle2</Text>
          </td>
          <td>
            {typography.subtitle2.fontSize}/{typography.subtitle2.lineHeight}
          </td>
          <td>{typography.subtitle2.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="subtitle3">subtitle3</Text>
          </td>
          <td>
            {typography.subtitle3.fontSize}/{typography.subtitle3.lineHeight}
          </td>
          <td>{typography.subtitle3.fontWeight}</td>
        </tr>
      </tbody>
    </table>
  </Box>
);

const Example = () => (
  <Box bg="levels.surface" p={2} width="600px" mt={5} textAlign="left">
    <Text typography="h2" mb={3}>
      Examples
    </Text>
    <table css={tableCss} border={1}>
      <thead>
        <tr>
          <th width="100px">Name</th>
          <th>Sample Text</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>
            <Text typography="h1">H1</Text>
          </td>
          <td>
            <Text typography="h1">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="h2">H2</Text>
          </td>
          <td>
            <Text typography="h2">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="h3">H3</Text>
          </td>
          <td>
            <Text typography="h3">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="h4">H4</Text>
          </td>
          <td>
            <Text typography="h4">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="body1">Body1</Text>
          </td>
          <td>
            <Text typography="body1">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="body2">Body2</Text>
          </td>
          <td>
            <Text typography="body2">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="subtitle1">SubTitle1</Text>
          </td>
          <td>
            <Text typography="subtitle1">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="subtitle2">subtitle2</Text>
          </td>
          <td>
            <Text typography="subtitle2">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="subtitle3">subtitle3</Text>
          </td>
          <td>
            <Text typography="subtitle3">{sample}</Text>
          </td>
        </tr>
      </tbody>
    </table>
  </Box>
);

const sample = `Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.`;
const tableCss = `
  vertical-align: top;
  width: 100%;
  margin-top: 20px;

  th {
    text-align: left;
    font-weight: bold;
  }

  td, th {
    padding: 8px;
  }
`;
