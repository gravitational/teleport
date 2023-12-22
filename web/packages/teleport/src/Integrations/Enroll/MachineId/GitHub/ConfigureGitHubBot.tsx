import React, { useState } from 'react';
import styled from 'styled-components'

import Box from 'design/Box'
import Select from 'shared/components/Select'
import Validation from 'shared/components/Validation'
import { FlowStepProps } from '../Flow/Flow';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Text from 'design/Text';
import Input from 'design/Input';
import { FlowButtons } from '../Flow/FlowButtons';

export function ConfigureGitHubBot({ nextStep, prevStep }: FlowStepProps) {
  const [selectedLabels, setSelectedLabels] = useState([])
  const [login, setLogin] = useState('')
  const [botName, setBotName] = useState('')

  return (
    <Box as="form" onSubmit={(e) => {
      e.preventDefault()
    }}>
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

            <FlowButtons isFirst={true} isLast={false} nextStep={nextStep} prevStep={prevStep} />
          </>
        )}
      </Validation>
    </Box>
  )
}

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
`