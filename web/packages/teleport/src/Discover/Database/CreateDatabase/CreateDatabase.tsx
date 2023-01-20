/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import {
  Text,
  Box,
  Flex,

  // AnimatedProgressBar,
  // ButtonPrimary,
  // ButtonSecondary,
} from 'design';
import { Danger } from 'design/Alert';

// import Dialog, { DialogContent } from 'design/DialogConfirmation';
// import * as Icons from 'design/Icon';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import TextEditor from 'shared/components/TextEditor';

// import { Timeout } from 'teleport/Discover/Shared/Timeout';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  LabelsCreater,
  Mark,
  // TextIcon,
} from '../../Shared';
import { dbCU } from '../../yamlTemplates';
import { DatabaseLocation, getDatabaseProtocol } from '../resources';

import { useCreateDatabase, State } from './useCreateDatabase';

import type { AgentStepProps } from '../../types';
import type { AgentLabel } from 'teleport/services/agents';
// import type { Attempt } from 'shared/hooks/useAttemptNext';
import type { AwsRds } from 'teleport/services/databases';

export function CreateDatabase(props: AgentStepProps) {
  const state = useCreateDatabase(props);
  return <CreateDatabaseView {...state} />;
}

export function CreateDatabaseView({
  attempt,
  // clearAttempt,
  registerDatabase,
  canCreateDatabase,
  // pollTimeout,
  dbEngine,
  dbLocation,
}: State) {
  const [dbName, setDbName] = useState('');
  const [dbUri, setDbUri] = useState('');
  const [labels, setLabels] = useState<AgentLabel[]>([]);

  // TODO(lisa): default ports depend on type of database.
  const [dbPort, setDbPort] = useState('5432');

  // TODO(lisa): refactor using ryan's example (reusable).
  const [awsAccountId, setAwsAccountId] = useState('');
  const [awsResourceId, setAwsResourceId] = useState('');

  function handleOnProceed(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    let awsRds: AwsRds;
    if (dbLocation === DatabaseLocation.AWS) {
      awsRds = {
        accountId: awsAccountId,
        resourceId: awsResourceId,
      };
    }

    registerDatabase({
      labels,
      name: dbName,
      uri: `${dbUri}:${dbPort}`,
      protocol: getDatabaseProtocol(dbEngine),
      awsRds,
    });
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box maxWidth="800px">
          <Header>Register a Database</Header>
          <HeaderSubtitle>
            Create a new database resource for the database server.
          </HeaderSubtitle>
          {attempt.status === 'failed' && (
            <Danger children={attempt.statusText} />
          )}
          {!canCreateDatabase && (
            <Box>
              <Text>
                You don't have permission to register a database.
                <br />
                Please ask your Teleport administrator to update your role and
                add the <Mark>db</Mark> rule:
              </Text>
              <Flex minHeight="195px" mt={3}>
                <TextEditor
                  readOnly={true}
                  data={[{ content: dbCU, type: 'yaml' }]}
                />
              </Flex>
            </Box>
          )}
          {canCreateDatabase && (
            <>
              <Box width="500px">
                <FieldInput
                  label="Database Name"
                  // We need this name to comply with AWS policy name
                  // since it will be used as part of the policy name
                  // for the AWS flow.
                  rule={requiredField('database name is required')}
                  autoFocus
                  value={dbName}
                  placeholder="Enter database name"
                  onChange={e => setDbName(e.target.value)}
                  toolTipContent="An identifier name for this new database for Teleport."
                />
              </Box>
              <Flex width="500px">
                <FieldInput
                  label="Database Connection Endpoint"
                  rule={requiredField('connection endpoint is required')}
                  value={dbUri}
                  placeholder="db.example.com"
                  onChange={e => setDbUri(e.target.value)}
                  toolTipContent="Database location and connection information."
                  width="50%"
                  mr={2}
                />
                <FieldInput
                  label="Endpoint Port"
                  rule={requirePort}
                  value={dbPort}
                  placeholder="5432"
                  onChange={e => setDbPort(e.target.value)}
                  width="50%"
                />
              </Flex>
              {dbLocation === DatabaseLocation.AWS && (
                <>
                  <Box width="500px">
                    <FieldInput
                      label="AWS Account ID"
                      rule={requiredAwsAccountId}
                      value={awsAccountId}
                      placeholder="123456789012"
                      onChange={e => setAwsAccountId(e.target.value)}
                      toolTipContent="A 12-digit number that uniquely identifies your AWS account."
                    />
                  </Box>
                  <Box width="500px" mb={6}>
                    <FieldInput
                      label="Resource ID"
                      value={awsResourceId}
                      rule={requiredField('database resource ID is required')}
                      placeholder="db-ABCDE1234567..."
                      onChange={e => setAwsResourceId(e.target.value)}
                      toolTipContent={
                        <Text>
                          The unique identifier for your resource. For AWS
                          database, may have the prefix <Mark light>db-</Mark>{' '}
                          then follow with alphanumerics.
                        </Text>
                      }
                    />
                  </Box>
                </>
              )}
              <Box mt={3}>
                <Text bold>Labels (optional)</Text>
                <Text mb={2}>
                  Labels make this new database discoverable by the database
                  service. <br />
                  Not defining labels is equivalent to asteriks (any database
                  service can discover this database).
                </Text>
                <LabelsCreater
                  labels={labels}
                  setLabels={setLabels}
                  isLabelOptional={true}
                  disableBtns={attempt.status === 'processing'}
                />
              </Box>
            </>
          )}
          <ActionButtons
            onProceed={() => handleOnProceed(validator)}
            // On failure, allow user to attempt again.
            disableProceed={
              attempt.status === 'processing' || !canCreateDatabase
            }
          />
          {/* {(attempt.status === 'processing' || attempt.status === 'failed') && (
            <CreateDatabaseDialog
              pollTimeout={pollTimeout}
              attempt={attempt}
              retry={() => handleOnProceed(validator)}
              close={clearAttempt}
            />
          )} */}
        </Box>
      )}
    </Validation>
  );
}

// const CreateDatabaseDialog = ({
//   pollTimeout,
//   attempt,
//   retry,
//   close,
// }: {
//   pollTimeout: number;
//   attempt: Attempt;
//   retry(): void;
//   close(): void;
// }) => {
//   return (
//     <Dialog disableEscapeKeyDown={false} open={true}>
//       <DialogContent
//         width="400px"
//         alignItems="center"
//         mb={0}
//         textAlign="center"
//       >
//         {attempt.status !== 'failed' ? (
//           <>
//             {' '}
//             <Text bold caps mb={4}>
//               Registering Database
//             </Text>
//             <AnimatedProgressBar />
//             <TextIcon
//               css={`
//                 white-space: pre;
//               `}
//             >
//               <Icons.Restore fontSize={4} />
//               <Timeout
//                 timeout={pollTimeout}
//                 message=""
//                 tailMessage={' seconds left'}
//               />
//             </TextIcon>
//           </>
//         ) : (
//           <Box width="100%">
//             <Text bold caps mb={3}>
//               Database Register Failed
//             </Text>
//             <Text mb={5}>
//               <Icons.Warning ml={1} mr={2} color="danger" />
//               Error: {attempt.statusText}
//             </Text>
//             <Flex>
//               <ButtonPrimary mr={2} width="50%" onClick={retry}>
//                 Retry
//               </ButtonPrimary>
//               <ButtonSecondary width="50%" onClick={close}>
//                 Close
//               </ButtonSecondary>
//             </Flex>
//           </Box>
//         )}
//       </DialogContent>
//     </Dialog>
//   );
// };

// PORT_REGEXP only allows digits with length 4.
export const PORT_REGEX = /^\d{4}$/;
const requirePort = value => () => {
  const isValidId = value.match(PORT_REGEX);
  if (!isValidId) {
    return {
      valid: false,
      message: 'port must be 4 digits',
    };
  }
  return {
    valid: true,
  };
};

// AWS_ACC_ID_REGEX only allows digits with length 12.
export const AWS_ACC_ID_REGEX = /^\d{12}$/;
const requiredAwsAccountId = value => () => {
  const isValidId = value.match(AWS_ACC_ID_REGEX);
  if (!isValidId) {
    return {
      valid: false,
      message: 'aws account id must be 12 digits',
    };
  }
  return {
    valid: true,
  };
};

// TODO(lisa): this check and the backend check does not match
// re-visit and let backend do the checking for now.
//
// // AWS_POLICY_NAME_REGEX only allows alphanumeric including the
// // following common characters: plus (+), equal (=), comma (,),
// // period (.), at (@), underscore (_), and hyphen (-).
// // As defined in: https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-quotas.html
// export const AWS_POLICY_NAME_REGEX = /^[\w@+=,.:/-]+$/;
// const conformNameWithAWSPolicyNameReq = value => () => {
//   const isValid = value.match(AWS_POLICY_NAME_REGEX);
//   if (!isValid) {
//     return {
//       valid: false,
//       message:
//         'name must be alphanumerics, including characters such as _ @ = , . + -',
//     };
//   }
//   return {
//     valid: true,
//   };
// };
