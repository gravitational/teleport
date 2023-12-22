import React, { useState } from 'react';
import styled from 'styled-components'

import Box from 'design/Box'
import Select from 'shared/components/Select'
import Validation from 'shared/components/Validation'
import { FlowStepProps } from '../Flow/Flow';
import Text from 'design/Text';
import Input from 'design/Input';
import { FlowButtons } from '../Flow/FlowButtons';
import { LabelSelector } from 'teleport/components/LabelSelector';

export function ConfigureBot({ nextStep, prevStep }: FlowStepProps) {
  const [selectedLabels, setSelectedLabels] = useState([])
  const [login, setLogin] = useState('')
  const [botName, setBotName] = useState('')

  return (
    <Box as="form" onSubmit={(e) => {
      e.preventDefault()
    }}>
      <Text bold fontSize={4} mb="3">Step 2: Configure Bot Access</Text>
      <Text fontSize={3} mb="3">These next fields will enable Teleport to scope access to only what is needed by your GitHub Actions workflow.</Text>
      <Validation>
        {({ validator }) => (
          <>
            <FormItem>
              <Text>Resource Your GitHub Action Will Connect To *</Text>
              <Select
                isMulti={true}
                isSearchable
                value={selectedLabels}
                onChange={console.log}
                options={[]}
                placeholder='Select Resource Labels...'
              />
            </FormItem>
            <FormItem>
              <Text>SSH User that Your Machine User Can Access</Text>
              <Input
                mb={3}
                label="SSH User that Your Machine User Can Access"
                placeholder="ex. ubuntu"
                value={login}
                onChange={(e) => setLogin(e.target.value)}
              />
            </FormItem>

            <FormItem>
              <Text>Create a Name for Your Machine User</Text>
              <Input
                mb={3}
                label="Create a Name for Your Machine User"
                placeholder="ex. github-actions-deploy"
                value={botName}
                onChange={(e) => setBotName(e.target.value)}
              />
            </FormItem>

            <FlowButtons isFirst={true} nextStep={nextStep} prevStep={prevStep} />
          </>
        )}
      </Validation>
    </Box>
  )
}

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
  max-width: 500px;
`
