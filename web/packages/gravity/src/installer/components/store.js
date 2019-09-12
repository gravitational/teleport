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
import cfg from 'gravity/config';
import { forEach, at, set, map, merge, keys, values, unset } from 'lodash';
import { OpStateEnum, ProviderEnum } from 'gravity/services/enums';
import { Store, useStore } from 'gravity/lib/stores';

export const StepEnum = {
  LICENSE: 'license',
  NEW_APP: 'new_app',
  PROVISION: 'provision',
  PROGRESS: 'progress',
  USER: 'user',
}

const defaultServiceSubnet = '10.100.0.0/16';
const defaultPodSubnet = '10.244.0.0/16';

export default class InstallerStore extends Store {

  state = {

    // Current installation step
    step: '',

    // Defined installation steps
    stepOptions: [],

    // License required for installation
    license: null,

    // Store status
    status: 'loading',

    // Installer config which has app custom installer settings
    config: merge({}, cfg.modules.installer),

    // Indicates of user accepted EULA agreement
    eulaAccepted: false,

    // Application data
    app: {
    },

    // Install operation data
    operation: null,

    // Cluster tags
    tags: {},

    // Entered cluster name
    clusterName: '',

    // Service subnet
    serviceSubnet: defaultServiceSubnet,
    // Pod subnet
    podSubnet: defaultPodSubnet,

    // Available app flavors
    flavors: null,

    // Parameters for selected flavor and connected servers
    provision: {
      profiles: {
      },
      servers: []
    },

    // Joined onprem servers
    agentServers: []
  }

  acceptEula = () => {
    this.setState({
      eulaAccepted: true
    })
  }

  setError(err) {
    this.setState({
      status: 'error',
      statusText: err.message
    })
  }

  setLicense(license) {
    this.setState({
      license,
      step: StepEnum.NEW_APP
    })
  }

  setClusterTags(tags) {
    this.setState({
      tags: {
        ...tags
      }
    })
  }

  setStepProgress() {
    this.setState({
      step: StepEnum.PROGRESS
    })
  }

  setOnpremSubnets(serviceSubnet, podSubnet) {
    this.setState({
      serviceSubnet,
      podSubnet
    })
  }

  setClusterName(clusterName) {
    this.setState({
      clusterName
    })
  }

  makeOnpremRequest() {
    const { clusterName, license, tags, serviceSubnet, podSubnet } = this.state;
    const { packageId } = this.state.app;

    const request = {
      app_package: packageId,
      domain_name: clusterName,
      provider: null,
      license,
      labels: tags
    };

    request.provider = {
      provisioner: ProviderEnum.ONPREM,
      [ProviderEnum.ONPREM]: {
        pod_cidr: podSubnet,
        service_cidr: serviceSubnet
      }
    };

    return request;
  }

  makeAgentRequest() {
    const { siteId, id: opId } = this.state.operation;
    return {
      siteId,
      opId
    }
  }

  makeStartInstallRequest() {
    const request = {
      siteId: this.state.operation.siteId,
      opId: this.state.operation.id,
      profiles: {},
      servers: []
    };


    keys(this.state.provision.profiles).forEach(key => {
      const { instanceType, count } = this.state.provision.profiles[key];
      request.profiles[key] = {
        instance_type: instanceType,
        count
      }
    })

    const serverMap = this.state.provision.servers;
    keys(serverMap).map(role => {
      values(serverMap[role]).map(server => {
        const os = server.os;
        const role = server.role;
        const system_state = null;
        const advertise_ip = server.ip;
        const hostname = server.hostname;
        const mounts = map(server.mounts, mount => ({
          name: mount.name,
          source: mount.value
        }));

        request.servers.push({
          os,
          role,
          system_state,
          advertise_ip,
          hostname,
          mounts,
        })
      })
    })

    return request;
  }

  initWithApp(app) {
    let step = StepEnum.LICENSE;

    const stepOptions = [
      { value: StepEnum.LICENSE, title: 'License' },
      { value: StepEnum.NEW_APP, title: 'Cluster name' },
      { value: StepEnum.PROVISION, title: 'Capacity' },
      { value: StepEnum.PROGRESS, title: 'Installation' },
      { value: StepEnum.USER, title: 'Create Admin' }
    ]

    // remove license step
    if (!app.licenseRequired) {
      stepOptions.shift();
      step = StepEnum.NEW_APP;
    }

    // remove bandwagon step
    if (app.bandwagon) {
      stepOptions.unshift()
    }

    const [
      installerConfig,
      agentReportConfig
    ] = at(app,
      [
        'config.modules.installer',
        'config.agentReport'
      ]);

    // TODO: fixme
    // overrides default agent report config
    merge(cfg, { agentReportConfig });

    const config = merge(this.state.config, installerConfig);
    this.setState({
      status: 'ready',
      stepOptions,
      app,
      step,
      config,
    })
  }

  initWithCluster(details) {
    const { app, operation, flavors } = details;
    const step = mapOpStateToStep(operation.state);
    this.initWithApp(app);
    this.setState({
      flavors,
      step,
      operation,
      eulaAccepted: true
    })
  }

  setProvisionProfiles(profiles) {
    const provisitProfiles = {}

    forEach(profiles, p => {
      provisitProfiles[p.name] = {
        count: p.count
      }
    });

    const provision = {
      ...this.state.provision,
      profiles: provisitProfiles
    }

    this.setState({
      provision
    })
  }

  setAgentServers(agentServers) {
    this.setState({
      agentServers
    })
  }

  setServerVars({ role, hostname, ip, mounts }) {
    set(this.state.provision, ['servers', role, hostname], {
      role,
      hostname,
      ip,
      mounts
    })

    this.setState({
      ...this.state.provision
    })
  }

  removeServerVars({ role, hostname }) {
    unset(this.state.provision, ['servers', role, hostname]);
    this.setState({
      ...this.state.provision
    })
  }
}

const installerContext = React.createContext({});

function mapOpStateToStep(state) {
  let step;
  switch (state) {
    case OpStateEnum.CREATED:
    case OpStateEnum.INSTALL_INITIATED:
    case OpStateEnum.INSTALL_PRECHECKS:
    case OpStateEnum.INSTALL_SETTING_CLUSTER_PLAN:
      step = StepEnum.PROVISION;
      break;
    default:
      step = StepEnum.PROGRESS;
  }

  return step;
}

export const Provider = installerContext.Provider;

export function useInstallerContext() {
  return React.useContext(installerContext);
}

export function useInstallerStore() {
  const store = useInstallerContext()
  return useStore(store);
}