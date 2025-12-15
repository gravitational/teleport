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

import { Box } from 'design';

import { addIndexToViews } from '../flow';
import { Navigation } from './Navigation';

export default {
  title: 'Teleport/StepNavigation',
};

const steps = [
  { title: 'first title' },
  { title: 'second title' },
  { title: 'third title' },
  { title: 'fourth title' },
  { title: 'fifth title' },
  { title: 'sixth title' },
  { title: 'seventh title' },
  { title: 'eighth title' },
];

export const WithoutNesting = () => {
  return (
    <>
      <Box mb={5}>
        <Navigation currentStep={0} views={steps.slice(0, 2)} />
      </Box>
      <Box mb={5}>
        <Navigation currentStep={1} views={steps.slice(0, 2)} />
      </Box>
      <Box mb={5}>
        <Navigation currentStep={2} views={steps.slice(0, 2)} />
      </Box>
      <Box>
        <Navigation currentStep={3} views={steps} />
      </Box>
    </>
  );
};

export const WithNesting = () => {
  const nestedViews = [
    {
      title: 'First Step',
    },
    {
      title: 'Nesting',
      views: [
        {
          title: 'Nesting Again',
          views: [
            {
              title: 'Second Step',
            },
            {
              title: 'Third Step',
            },
          ],
        },
        {
          title: 'Fourth Step',
        },
      ],
    },
    {
      title: 'Fifth Step',
    },
  ];

  const indexedViews = addIndexToViews(nestedViews);
  return (
    <>
      <Box mb={5}>
        <Navigation currentStep={0} views={indexedViews} />
      </Box>
      <Box mb={5}>
        <Navigation currentStep={1} views={indexedViews} />
      </Box>
      <Box mb={5}>
        <Navigation currentStep={2} views={indexedViews} />
      </Box>
      <Box mb={5}>
        <Navigation currentStep={3} views={indexedViews} />
      </Box>
      <Box mb={5}>
        <Navigation currentStep={4} views={indexedViews} />
      </Box>
      <Box>
        <Navigation currentStep={5} views={indexedViews} />
      </Box>
    </>
  );
};
