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
import { useFluxStore } from 'gravity/components/nuclear';
import cfg from 'gravity/config';
import { getters as namespaceGetters } from 'gravity/cluster/flux/k8sNamespaces';
import { Switch, Redirect, Route } from 'gravity/components/Router';
import history from 'gravity/services/history';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import { Flex, Text, Box } from 'design'
import k8sContext from './k8sContext';
import Pods from './Pods';
import Services from './Services';
import Jobs from './Jobs';
import Secrets from './Secrets';
import Tabs, { TabItem } from './Tabs';
import Deployments from './Deployments';
import DaemotSets from './DaemonSets';
import NamespaceMenu from './SelectNamespace';
import ConfigMaps from './ConfigMaps';
import K8sResourceDialog from './K8sResourceDialog';
import { withState } from 'shared/hooks';

export class K8s extends React.Component {

  state = {
    resourceToView: null
  }

  onChangeNamespace = namespace => {
    const { category, history } = this.props;
    const newRoute = cfg.getSiteK8sRoute(namespace, category);
    history.push(newRoute);
  }

  onCloseResource = () => {
    this.setState({
      resourceToView: null
    })
  }

  onViewResource = (name, resourceMap) => {
    const resource = resourceMap.toJSON();
    this.setState({
      resourceToView: {
        name,
        resource
      }
    })
  }

  render() {
    const { namespace, category, namespaces } = this.props;
    const { resourceToView } = this.state;
    const { onViewResource } = this;

    // when accessing the index route, redirect to the first tab
    if(!namespace || !category){
      return (
        <Switch>
          <Redirect exact to={cfg.getSiteK8sConfigMapsRoute('default')}/>
        </Switch>
      )
    }

    return (
      <k8sContext.Provider value={{
        namespace,
        onViewResource
      }}>
        <FeatureBox>
          <FeatureHeader alignItems="center" mb="4">
            <FeatureHeaderTitle mr="4">
              Kubernetes
            </FeatureHeaderTitle>
            <Flex bg="primary.light" alignItems="center">
              <Text typography="body2" color="text.primary" px="3">
                NAMESPACE:
              </Text>
              <NamespaceMenu onChange={this.onChangeNamespace} options={namespaces} value={namespace} />
            </Flex>
          </FeatureHeader>
          <Tabs>
            <TabItem to={cfg.getSiteK8sConfigMapsRoute(namespace)} title="ConfigMaps" />
            <TabItem to={cfg.getSiteK8sSecretsRoute(namespace)} title="Secrets" />
            <TabItem to={cfg.getSiteK8sPodsRoute(namespace)} title="Pods" />
            <TabItem to={cfg.getSiteK8sServicesRoute(namespace)} title="Services" />
            <TabItem to={cfg.getSiteK8sJobsRoute(namespace)} title="Jobs" />
            <TabItem to={cfg.getSiteK8sDaemonsRoute(namespace)} title="Daemon Sets" />
            <TabItem to={cfg.getSiteK8sDeploymentsRoute(namespace)} title="Deployments" />
          </Tabs>
          <Box mt="4">
            <Switch>
              <Route title="Config Maps"path={cfg.routes.siteK8sConfigMaps} component={ConfigMaps}/>
              <Route title="Pods" path={cfg.routes.siteK8sPods} component={Pods}/>
              <Route title="Secrets" path={cfg.routes.siteK8sSecrets} component={Secrets}/>
              <Route title="Services" path={cfg.routes.siteK8sServices} component={Services}/>
              <Route title="Jobs" path={cfg.routes.siteK8sJobs} component={Jobs}/>
              <Route title="Daemot Sets" path={cfg.routes.siteK8sDaemonSets} component={DaemotSets}/>
              <Route title="Deployments" path={cfg.routes.siteK8sDeployments} component={Deployments}/>
            </Switch>
          </Box>
          {resourceToView && (
            <K8sResourceDialog
              onClose={this.onCloseResource}
              namespace={namespace}
              name={resourceToView.name}
              resource={resourceToView.resource}
            />
          )}
        </FeatureBox>
      </k8sContext.Provider>
    )
  }
}

export default withState(props => {
  const namespaces = useFluxStore(namespaceGetters.namespaceNames);
  const { namespace, category } = props.match.params;
  return {
    namespaces,
    category,
    namespace,
    history,
  }

})(K8s);