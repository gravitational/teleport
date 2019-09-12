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
import styled from 'styled-components';
import { withRouter } from 'react-router';
import cfg from 'gravity/config';
import $ from 'jQuery';
import api from 'gravity/services/api';
import { Flex } from 'design';
import Indicator from 'design/Indicator';
import { Danger } from 'design/Alert';
import history from 'gravity/services/history';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';

/**
 * Monitoring sets up Grafana context and creates an iframe
 * hosting grafana UI
 */
export class Monitoring extends React.Component {

  state = {
    isInitializing: true,
    canLoadGrafana: false,
    isError: false,
  }

  iframeRef = React.createRef();

  initGrafana() {
    const { match, location } = this.props;
    const { siteId, grafanaPath } = parseRouteParams(match, location);
    const contextUrl = cfg.getSiteGrafanaContextUrl(siteId);

    // fetch grafana proxy endpoint URL
    return api.put(contextUrl).then(json  => {
      json = json || {};
      const proxyEndpoint = json.url

      if(!proxyEndpoint){
        return $.Deferred().reject(new Error("Could not resolve grafana endpoints"));
      }

      return makeGrafanaUrl(proxyEndpoint, grafanaPath);
    })
  }

  componentDidMount(){
    this.initGrafana().done(url => {
      this.url = url;
      this.setState({ canLoadGrafana: true }, () => {
        this._tweakGrafana();
      });
    })
    .fail(err => {
      var errorText = api.getErrorText(err);
      this.setState({
        isError: true,
        errorText
      })
    })
  }

  render() {
    const { canLoadGrafana } = this.state;
    const $status = this.renderStatus();
    return (
      <FeatureBox>
        <FeatureHeader alignItems="center">
          <FeatureHeaderTitle>
            Monitoring
          </FeatureHeaderTitle>
        </FeatureHeader>
        <StyledGrafana height="100%" width="100%" borderRadius="3">
          {$status}
          { canLoadGrafana && <iframe ref={this.iframeRef} src={this.url} frameBorder="0" /> }
        </StyledGrafana>
      </FeatureBox>
    )
  }

  renderStatus() {
    const { isInitializing, isError, errorText } = this.state;
    if (isError){
      return (
        <StyledStatusBox>
          <Danger width="100%">
            {errorText}
          </Danger>
        </StyledStatusBox>
      )
    }

    if (isInitializing){
      return (
        <StyledStatusBox>
          <Indicator />
        </StyledStatusBox>
      )
    }

    return null
  }

  _tweakGrafana(){
    const $iframe = $(this.iframeRef.current);
    $iframe.on('load', () => {
      this.setState({ isInitializing: false });
      $iframe.contents()
        .find('head')
        .append($(grafanaStyleOverrides))

      $iframe.addClass("--loaded");
    })
  }
}

const StyledStatusBox = styled.div`
  position: absolute;
  align-items: center;
  display: flex;
  height: 100px;
  width: 100%;
  justify-content: center;
`

const StyledGrafana = styled(Flex)`
  position: relative;
  overflow: auto;
  iframe {
    width: 100%;
    height: 100%;
    visibility: hidden;
  }

  iframe.--loaded {
    visibility: initial;
  }
`
/**
 * grafanaStyleOverrides is the style tag to be injected to
 * Grafana iframe for custom overrides.
 */
const grafanaStyleOverrides = `
  <style>
    /* ---- grafana 5x ---- */
    /* main nav menu button */
    .navbar-button--share{
      display: none;
    }

    /* left app menu */
    .grafana-app > sidemenu {
      display: none;
    }
  </style>
`

/**
 * parseRouteParams returns siteId and grafana path taken from
 * current URL.
 *
 * Ex: /web/site/clusterX/monitor/dashboard/db/pods?query=""
 * Result: { siteId: 'clusterX', grafanaPath: 'dashboard/db/pods?query=""` }
 */
function parseRouteParams(match, location){
  const featurePath = match.url;
  const { siteId } = match.params;
  const { pathname, search } = location;

  let grafanaPath = '';
  if(pathname.indexOf(featurePath) === 0){
    grafanaPath = pathname.substring(featurePath.length) + search;
  }

  return { siteId,  grafanaPath};
}

function makeGrafanaUrl(proxyEndpoint, dashboardUrl) {
  const grafanaDefaultDashboardUrl = cfg.getSiteDefaultDashboard();

  let url = `${proxyEndpoint}/${dashboardUrl}`;
  url = url.replace(/\/\/+/g, '/');

  // if empty query, use default dashboard if provided
  if (url === proxyEndpoint+'/' && grafanaDefaultDashboardUrl) {
    url = `${proxyEndpoint}/${grafanaDefaultDashboardUrl}`;
  }

  return history.ensureBaseUrl(url);
}


export default withRouter(Monitoring);
