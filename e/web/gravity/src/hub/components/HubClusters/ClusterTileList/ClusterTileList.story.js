import React from 'react'
import { storiesOf } from '@storybook/react'
import ClusterTileList from './ClusterTileList'
import { icon } from './fixtures';

storiesOf('GravityHub/Clusters', module)
  .add('ClusterTileList', () => {
    return (
      <ClusterTileList clusters={clusters} />
    );
  })

  const clusters = [
    {
      "id": "dev-cluster-with-many-apps",
      "createdBy": "vodoh@dumse.la",
      "serverCount": 2,
      "packageName": "everything",
      "packageVersion": "1.2.3-beta",
      "apps": [
        {
          "name": "esteemed-bobcat",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:25.000Z",
          "updatedText": "30/04/2019 14:35:25",

        },
        {
          "name": "idolized-clownfish",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:23.000Z",
          "updatedText": "30/04/2019 14:35:23",
          "icon": icon
        },
        {
          "name": "kissed-eagle",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:27.000Z",
          "updatedText": "30/04/2019 14:35:27",
          "icon": icon
        },
        {
          "name": "Sivteufu",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:27.000Z",
          "updatedText": "30/04/2019 14:35:27",
          "icon": icon
        },
        {
          "name": "Takoog",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:27.000Z",
          "updatedText": "30/04/2019 14:35:27",
          "icon": icon
        }
      ],
      "labels": {
        "Name": "kindlumiere2836",
        "Cisupaj": "Acoziwrog",
        "Jeratmu": "Zoajazed",
        "Kelomag": "Rufjocke",
        "Nimsara": "Ozmobo",
      },
      "location": "",
      "logo": "",
      "provider": "onprem",
      "webConfig": {},
      "installerUrl": "/web/installer/site/kindlumiere2836",
      "siteUrl": "/web/site/kindlumiere2836",
      "state": "active",
      "status": "ready",
      "created": new Date("2019-04-30T17:59:40.955Z"),
      "createdText": "30/04/2019 13:59:40",
      "features": {
        "monitoringEnabled": true,
        "k8sEnabled": true,
        "logsEnabled": true
      }
    },
    {
      "id": "dev-cluster-no-logo",
      "createdBy": "gipriwgaf@wuhetfuh.com",
      "serverCount": 2,
      "packageName": "telekube",
      "packageVersion": "1.2.3-beta",
      "apps": [ ],
      "labels": {
        "Name": "dev-cluster"
      },
      "location": "",
      "logo": "",
      "provider": "onprem",
      "webConfig": {},
      "installerUrl": "/web/installer/site/dev-cluster",
      "siteUrl": "/web/site/dev-cluster",
      "state": "active",
      "status": "processing",
      "created": new Date("2019-04-30T17:59:40.955Z"),
      "createdText": "30/04/2019 13:59:40",
      "features": {
        "monitoringEnabled": true,
        "k8sEnabled": true,
        "logsEnabled": true
      }
    },
    {
      "id": "dev-cluster-with-apps",
      "createdBy": "valir@ug.mk",
      "serverCount": 2,
      "packageName": "everything",
      "packageVersion": "1.2.3-beta",
      "apps": [
        {
          "name": "esteemed-bobcat",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:25.000Z",
          "updatedText": "30/04/2019 14:35:25",

        },
        {
          "name": "kissed-eagle",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:27.000Z",
          "updatedText": "30/04/2019 14:35:27",
          "icon": icon
        },
        {
          "name": "Ruddolsog",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:27.000Z",
          "updatedText": "30/04/2019 14:35:27",
          "icon": icon
        },
        {
          "name": "Uwapeuma",
          "namespace": "default",
          "description": "Deploy a basic Alpine Linux pod",
          "chartName": "alpine",
          "chartVersion": "0.1.0",
          "version": "3.3",
          "status": "DEPLOYED",
          "endpoints": [],
          "updated": "2019-04-30T18:35:27.000Z",
          "updatedText": "30/04/2019 14:35:27",
          "icon": icon
        }
      ],
      "labels": {
        "Name": "dev-cluster"
      },
      "location": "Asia Pacific (Hong Kong)",
      "logo": "",
      "provider": "onprem",
      "webConfig": {},
      "installerUrl": "/web/installer/site/dev-cluster",
      "siteUrl": "/web/site/dev-cluster",
      "state": "active",
      "status": "error",
      "created": new Date("2019-04-30T17:59:40.955Z"),
      "createdText": "30/04/2019 13:59:40",
      "features": {
        "monitoringEnabled": true,
        "k8sEnabled": true,
        "logsEnabled": true
      }
    },
    {
      "id": "dev-cluster-with-logo",
      "createdBy": "valir@ug.mk",
      "serverCount": 2,
      "packageName": "everything",
      "packageVersion": "1.2.3-beta",
      "apps": [],
      "labels": {
        "Name": "dev-cluster"
      },
      "location": "Asia Pacific (Hong Kong)",
      "logo": icon,
      "provider": "onprem",
      "webConfig": {},
      "installerUrl": "/web/installer/site/dev-cluster",
      "siteUrl": "/web/site/dev-cluster",
      "state": "active",
      "status": "error",
      "created": new Date("2019-04-30T17:59:40.955Z"),
      "createdText": "30/04/2019 13:59:40",
      "features": {
        "monitoringEnabled": true,
        "k8sEnabled": true,
        "logsEnabled": true
      }
    }
  ]