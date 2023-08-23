/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

import typography from './typography';
import { Text, Box } from './../';

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
            <Text typography="h5">H5</Text>
          </td>
          <td>
            {typography.h5.fontSize}/{typography.h5.lineHeight}
          </td>
          <td>{typography.h5.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="h6">H6</Text>
          </td>
          <td>
            {typography.h6.fontSize}/{typography.h6.lineHeight}
          </td>
          <td>{typography.h6.fontWeight}</td>
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
            <Text typography="paragraph">Paragraph</Text>
          </td>
          <td>
            {typography.paragraph.fontSize}/{typography.paragraph.lineHeight}
          </td>
          <td>{typography.paragraph.fontWeight}</td>
        </tr>
        <tr>
          <td>
            <Text typography="paragraph2">Paragraph2</Text>
          </td>
          <td>
            {typography.paragraph2.fontSize}/{typography.paragraph2.lineHeight}
          </td>
          <td>{typography.paragraph2.fontWeight}</td>
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
            <Text typography="h5">H5</Text>
          </td>
          <td>
            <Text typography="h5">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="h6">H6</Text>
          </td>
          <td>
            <Text typography="h6">{sample}</Text>
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
            <Text typography="paragraph">Paragraph</Text>
          </td>
          <td>
            <Text typography="paragraph">{sample}</Text>
          </td>
        </tr>
        <tr>
          <td>
            <Text typography="paragraph2">Paragraph2</Text>
          </td>
          <td>
            <Text typography="paragraph2">{sample}</Text>
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
