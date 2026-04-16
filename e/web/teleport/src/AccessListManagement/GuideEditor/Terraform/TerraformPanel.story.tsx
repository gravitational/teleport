import { useState } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';

import { defaultSidePanelWidth } from '../Shared';
import { TerraformPanel, TerraformProps } from './TerraformPanel';
import { TerraformSideTab } from './TerraformSideTab';

export default {
  title: 'TeleportE/AccessLists/Terraform/TerraformPanel',
};

export function Empty() {
  return <StoryPanel terraform={{ data: '', loading: false, error: null }} />;
}

export function WithData() {
  return (
    <StoryPanel
      terraform={{ data: sampleTerraform, loading: false, error: null }}
    />
  );
}

export function WithError() {
  return (
    <StoryPanel
      terraform={{
        data: '',
        loading: false,
        error: new Error('Some kind of error message'),
      }}
    />
  );
}

export function Loading() {
  return (
    <StoryPanel
      terraform={{
        data: '',
        loading: true,
        error: null,
      }}
    />
  );
}

function StoryPanel({ terraform }: { terraform: TerraformProps }) {
  const [panelWidth, setPanelWidth] = useState(defaultSidePanelWidth);

  function toggleSidePanel() {
    setPanelWidth(panelWidth === 0 ? defaultSidePanelWidth : 0);
  }

  return (
    <Container>
      <Spacer>
        <NavSpacer>
          <TerraformSideTab onClick={toggleSidePanel} panelWidth={panelWidth} />
        </NavSpacer>
      </Spacer>
      <TerraformPanel
        panelWidth={panelWidth}
        updatePanelWidth={setPanelWidth}
        terraform={terraform}
      />
    </Container>
  );
}

const Container = styled(Flex)`
  flex: 1;
  width: 100%;
  height: 100vh;
  overflow: auto;
`;

const NavSpacer = styled(Flex)`
  position: relative;
  margin-top: ${p => p.theme.space[3]}px;
`;

const Spacer = styled(Flex)`
  flex: 1;
  min-width: 600px;
  flex-direction: column;
  overflow: auto;
`;

const sampleTerraform = `resource "teleport_access_list" "engineering_access" {
  header = {
    metadata = {
      name = "engineering-access"
    }
  }

  spec = {
    title       = "Engineering Access"
    description = "Access for the engineering team"

    grants = {
      roles = ["access", "editor"]
    }

    audit = {
      recurrence = {
        frequency    = "6 months"
        day_of_month = "1"
      }
    }
  }
}

resource "teleport_access_list" "engineering_access2" {
  header = {
    metadata = {
      name = "engineering-access"
    }
  }

  spec = {
    title       = "Engineering Access 2"
    description = "Access for the engineering team"

    grants = {
      roles = ["access", "editor"]
    }

    audit = {
      recurrence = {
        frequency    = "6 months"
        day_of_month = "1"
      }
    }
  }
}

resource "teleport_access_list" "engineering_access3" {
  header = {
    metadata = {
      name = "engineering-access"
    }
  }

  spec = {
    title       = "Engineering Access 3"
    description = "Access for the engineering team"

    grants = {
      roles = ["access", "editor"]
    }

    audit = {
      recurrence = {
        frequency    = "6 months"
        day_of_month = "1"
      }
    }
  }
}
`;
