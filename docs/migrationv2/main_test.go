package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_migrateFigures(t *testing.T) {
	input := `<Figure width="700">![Architecture of the setup you will complete in this guide](../img/linux-server-diagram.png)</Figure>`
	expected := `![Architecture of the setup you will complete in this guide](../img/linux-server-diagram.png)`
	assert.Equal(t, migrateFigures(input), expected)
}

func Test_migrateTabs(t *testing.T) {
	input := `<Tabs>
  <TabItem label="Public internet deployment with Let's Encrypt">
    Let's Encrypt verifies that you control the domain name of your Teleport cluster by communicating with the HTTPS server listening on port 443 of your Teleport Proxy Service.
  </TabItem>
  <TabItem label="Private network deployment">
    On your Teleport host, place a valid private key and a certificate chain in /var/lib/teleport/privkey.pem and /var/lib/teleport/fullchain.pem respectively.
  </TabItem>
</Tabs>`
	expected := `<Tabs>
  <Tab title="Public internet deployment with Let's Encrypt">
    Let's Encrypt verifies that you control the domain name of your Teleport cluster by communicating with the HTTPS server listening on port 443 of your Teleport Proxy Service.
  </Tab>
  <Tab title="Private network deployment">
    On your Teleport host, place a valid private key and a certificate chain in /var/lib/teleport/privkey.pem and /var/lib/teleport/fullchain.pem respectively.
  </Tab>
</Tabs>`

	assert.Equal(t, migrateTabs(input), expected)
}

func Test_migrateTipAdmonitions(t *testing.T) {
	input := `<Admonition type="tip" title="OS User Mappings">The users that you specify in the logins flag (e.g., root, ubuntu and ec2-user in our examples) must exist on your Linux host. Otherwise, you will get authentication errors later in this tutorial.</Admonition>`
	expected := `<Tip>The users that you specify in the logins flag (e.g., root, ubuntu and ec2-user in our examples) must exist on your Linux host. Otherwise, you will get authentication errors later in this tutorial.</Tip>`
	assert.Equal(t, migrateTipAdmonitions(input), expected)
}

func Test_migrateNoteAdmonitions(t *testing.T) {
	input := `<Admonition type="note">apt, yum, and zypper repos don't expose packages for all distribution variants. When following installation instructions, you might need to replace ID with ID_LIKE to install packages of the closest supported distribution.</Admonition>`
	expected := `<Note>apt, yum, and zypper repos don't expose packages for all distribution variants. When following installation instructions, you might need to replace ID with ID_LIKE to install packages of the closest supported distribution.</Note>`

	assert.Equal(t, migrateNoteAdmonitions(input), expected)
}

func Test_migrateWarningAdmonitions(t *testing.T) {
	input := `<Admonition type="warning" title="Preview">Login Rules are currently in Preview mode.</Admonition>`
	expected := `<Warning>Login Rules are currently in Preview mode.</Warning>`
	assert.Equal(t, migrateWarningAdmonitions(input), expected)
}

func Test_migrateTipNotices(t *testing.T) {
	input := `<Notice type="tip">lorem ipsum</Notice>`
	expected := `<Tip>lorem ipsum</Tip>`
	assert.Equal(t, migrateTipNotices(input), expected)
}

func Test_migrateWarningNotices(t *testing.T) {
	input := `<Notice type="warning">warning lorem ipsum</Notice>`
	expected := `<Warning>warning lorem ipsum</Warning>`
	assert.Equal(t, migrateWarningNotices(input), expected)
}

func Test_migrateDetails(t *testing.T) {
	input := `<Details title="Logging in via the CLI">

  In addition to Teleport's Web UI, you can access resources in your
  infrastructure via the tsh client tool.
  
  Install tsh on your local workstation:
  
  Log in to receive short-lived certificates from Teleport:
  
  </Details>`
	expected := `<Accordion title="Logging in via the CLI">

  In addition to Teleport's Web UI, you can access resources in your
  infrastructure via the tsh client tool.
  
  Install tsh on your local workstation:
  
  Log in to receive short-lived certificates from Teleport:
  
  </Accordion>`

	assert.Equal(t, migrateDetails(input), expected)
}

func Test_migrateVarComponent(t *testing.T) {
	input := `Variable is <Var name="hello world" />`
	expected := `Variable is hello world`

	assert.Equal(t, migrateVarComponent(input), expected)
}

func Test_migrateVariables(t *testing.T) {
	input := `---
title: "Page Title"
description: "Page Description"
---

## Header

The cluster name is (=clusterDefaults.clusterName=)`
	expected := `---
title: "Page Title"
description: "Page Description"
---

import { clusterDefaults } from "/snippets/variables.mdx";

## Header

The cluster name is {clusterDefaults.clusterName}`

	assert.Equal(t, migrateVariables(input), expected)
}

func Test_migrateSnippets(t *testing.T) {

	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "", // TODO
			input: `---
title: "Page Title"
description: "Page Description"
---

## Header

(!docs/pages/includes/page.mdx!)`,
			expected: `---
title: "Page Title"
description: "Page Description"
---

import Page from "/snippets/includes/page.mdx";

## Header

<Page />`,
		},
		{
			input: `---
title: "Page Title"
description: "Page Description"
---

## Header

(!docs/pages/includes/plugins/enroll.mdx name="the Mattermost integration"!)`,
			expected: `---
title: "Page Title"
description: "Page Description"
---

import Enroll from "/snippets/includes/plugins/enroll.mdx";

## Header

<Enroll name="the Mattermost integration" />`,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {

			assert.Equal(t, migrateSnippets(c.input), c.expected)
		})
	}
}

func Test_migrateSnippetTemplateBinding(t *testing.T) {
	input := `{{ service="your Teleport instance" }}

Grant {{ service }} access to credentials that it can use to authenticate to AWS. If you are running {{ service }} on an EC2 instance, you should use the EC2 Instance Metadata Service method. Otherwise, you must use environment variables:`
	expected := `
Grant { service || "your Teleport instance" } access to credentials that it can use to authenticate to AWS. If you are running { service || "your Teleport instance" } on an EC2 instance, you should use the EC2 Instance Metadata Service method. Otherwise, you must use environment variables:`
	assert.Equal(t, migrateSnippetTemplateBinding(input), expected)
}
