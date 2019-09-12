import styled from 'styled-components';
import { sortBy, keys, values } from 'lodash';
import React from 'react';
import { Image, Flex, Box, Text, Card } from 'design';
import clusterLogo from 'design/assets/images/kube-logo.svg';
import appLogo from 'design/assets/images/app-logo.svg';
import * as Icons from 'design/Icon';
import ActionMenu from '../../ClusterActionMenu';
import Status from './Status';
import history from 'gravity/services/history';

export default function ClusterTile({ cluster, ...styles }) {
  const {
    apps: appMap,
    siteUrl,
    createdBy,
    createdText,
    id,
    labels,
    logo,
    serverCount,
    status,
    packageName,
    packageVersion,
  } = cluster;

  const title = `${packageName.toUpperCase()} v.${packageVersion}`;
  const nodeText = `${serverCount} - node${serverCount === 1 ? '' : 's'}`;
  const refCard = React.useRef();
  const labelStr = keys(labels)
    .map(key => `${key}: ${labels[key]}`)
    .join(',');
  const apps = sortBy(values(appMap), 'updated');
  const contentProps = {
    apps,
    createdText,
    createdBy,
    logo,
    packageName,
    packageVersion,
  };

  function onTileClick(e) {
    // ignore if text selection
    const selection = window.getSelection();
    if (selection.toString().length !== 0) {
      return;
    }

    // ignore if outside of card component
    if (!refCard.current.contains(e.target)) {
      return;
    }

    window.open(history.ensureBaseUrl(siteUrl), '_blank');
  }

  return (
    <StyledClusterTile
      tabIndex="0"
      ref={refCard}
      onClick={onTileClick}
      as={Flex}
      flexDirection="column"
      minHeight="300px"
      width="500px"
      {...styles}
    >
      <Flex
        borderTopLeftRadius="3"
        borderTopRightRadius="3"
        bg="primary.main"
        px="3"
        py="2"
        alignItems="center"
      >
        <Status status={status} mr="4" />
        <Box overflow="auto">
          <Text typography="h5" bold caps>
            {id}
          </Text>
          <Text typography="body1" color="text.primary">
            {title}
          </Text>
        </Box>
        <ActionMenu cluster={cluster} />
      </Flex>
      <Flex alignItems="center" flex="1" px="5">
        {apps.length > 0 && <BundleContent {...contentProps} />}
        {apps.length === 0 && <ClusterContent {...contentProps} />}
      </Flex>
      <Flex
        borderBottomLeftRadius="3"
        borderBottomRightRadius="3"
        height="50px"
        bg="primary.main"
        color="text.primary"
        px="3"
        alignItems="center"
      >
        <Flex mr="5" alignItems="center" flex="0 0 auto">
          <Icons.Layers mr="2" />
          <Text typography="body2">{nodeText}</Text>
        </Flex>
        <Flex alignItems="center" style={{ overflow: 'auto' }}>
          <Icons.Label mr="2" />
          <Text title={labelStr} typography="body2" style={{ whiteSpace: 'nowrap' }}>
            {labelStr}
          </Text>
        </Flex>
      </Flex>
    </StyledClusterTile>
  );
}

const StyledClusterTile = styled(Card)`
  cursor: pointer;
  outline: none;
  :hover,
  :focus {
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.56);
  }
`;

function ClusterContent({ createdText, createdBy }) {
  return (
    <>
      <LogoBox>
        <Image width="40px" height="40px" src={clusterLogo} />
      </LogoBox>
      <SubTitle created={createdText} owner={createdBy} />
    </>
  );
}

function BundleContent({ apps, createdText, createdBy }) {
  const $icons = renderLogos(apps);
  const appNames = apps.map(app => `${app.chartName} ${app.chartVersion}`).join(', ');
  return (
    <>
      <LogoBox p="1">{$icons}</LogoBox>
      <div>
        <Text typography="h5" mb="1" bold style={{ wordBreak: 'break-all' }}>
          {appNames}
        </Text>
        <SubTitle created={createdText} owner={createdBy} />
      </div>
    </>
  );
}

function renderLogos(apps) {
  const iconSize = '28px';
  const iconMargin = '2px';

  return apps.map(({ icon, name }, number) => {
    if (number >= 4) {
      return;
    }

    // render an icon with a number of remaining items
    if (number === 3 && apps.length > 4) {
      return (
        <Text
          typography="body2"
          as={Flex}
          alignItems="center"
          justifyContent="center"
          bg="primary.light"
          borderRadius="2"
          key={name}
          m={iconMargin}
          width={iconSize}
          height={iconSize}
        >
          {`+${apps.length - number}`}
        </Text>
      );
    }

    const logoSvg = icon || appLogo;

    return <Image key={name} m={iconMargin} width={iconSize} height={iconSize} src={logoSvg} />;
  });
}

function SubTitle({ created, owner }) {
  return (
    <Text typography="body1" fontSize="1" color="text.primary">
      CREATED: {created}
      <br />
      OWNER: {owner}
    </Text>
  );
}

function LogoBox(props) {
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      bg="primary.dark"
      flexWrap="wrap"
      width="80px"
      height="80px"
      borderRadius="2"
      mr="3"
      flex="0 0 auto"
      {...props}
    />
  );
}
