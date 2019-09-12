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

import React from 'react'
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import { Services } from './Services'

storiesOf('Gravity/K8s', module)
  .add('Services', () => {
    const props = {
      siteId: 'xxx',
      namespace: 'kube-system',
      onFetch: () => $.Deferred(),
      services
    }

    return (
      <Services {...props} />
    );
  });


const services = [{
    "serviceMap": {
      "metadata": {
        "name": "tiller-deploy",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/tiller-deploy",
        "uid": "9bb77b69-0407-11e9-a200-42010a800006",
        "resourceVersion": "841",
        "creationTimestamp": "2018-12-20T03:30:33Z",
        "labels": {
          "app": "helm",
          "name": "tiller"
        },
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"labels\":{\"app\":\"helm\",\"name\":\"tiller\"},\"name\":\"tiller-deploy\",\"namespace\":\"kube-system\"},\"spec\":{\"ports\":[{\"name\":\"tiller\",\"port\":44134,\"protocol\":\"TCP\",\"targetPort\":\"tiller\"}],\"selector\":{\"app\":\"helm\",\"name\":\"tiller\"},\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}\n"
        }
      },
      "spec": {
        "ports": [{
          "name": "tiller",
          "protocol": "TCP",
          "port": 44134,
          "targetPort": "tiller"
        }],
        "selector": {
          "app": "helm",
          "name": "tiller"
        },
        "clusterIP": "10.100.54.106",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    },
    "namespace": "kube-system",
    "name": "tiller-deploy",
    "clusterIp": "10.100.54.106",
    "labels": [
      "app:helm",
      "name:tiller"
    ],
    "ports": [
      "TCP:44134/tiller"
    ]
  },
  {
    "serviceMap": {
      "metadata": {
        "name": "gravity-site",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/gravity-site",
        "uid": "a965a5ca-0407-11e9-a200-42010a800006",
        "resourceVersion": "1011",
        "creationTimestamp": "2018-12-20T03:30:56Z",
        "labels": {
          "app": "gravity-site"
        },
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{\"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout\":\"3600\",\"service.beta.kubernetes.io/aws-load-balancer-internal\":\"0.0.0.0/0\"},\"creationTimestamp\":null,\"labels\":{\"app\":\"gravity-site\"},\"name\":\"gravity-site\",\"namespace\":\"kube-system\"},\"spec\":{\"ports\":[{\"name\":\"web\",\"nodePort\":32009,\"port\":3009,\"targetPort\":0}],\"selector\":{\"app\":\"gravity-site\"},\"type\":\"LoadBalancer\"},\"status\":{\"loadBalancer\":{}}}\n",
          "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600",
          "service.beta.kubernetes.io/aws-load-balancer-internal": "0.0.0.0/0"
        }
      },
      "spec": {
        "ports": [{
          "name": "web",
          "protocol": "TCP",
          "port": 3009,
          "targetPort": 3009,
          "nodePort": 32009
        }],
        "selector": {
          "app": "gravity-site"
        },
        "clusterIP": "10.100.183.174",
        "type": "LoadBalancer",
        "sessionAffinity": "None",
        "externalTrafficPolicy": "Cluster"
      },
      "status": {
        "loadBalancer": {
          "ingress": [{
            "ip": "35.184.135.171"
          }]
        }
      }
    },
    "namespace": "kube-system",
    "name": "gravity-site",
    "clusterIp": "10.100.183.174",
    "labels": [
      "app:gravity-site"
    ],
    "ports": [
      "TCP:3009/3009"
    ]
  },
  {
    "serviceMap": {
      "metadata": {
        "name": "gravity-public",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/gravity-public",
        "uid": "5714749a-0407-11e9-a200-42010a800006",
        "resourceVersion": "431",
        "creationTimestamp": "2018-12-20T03:28:38Z",
        "labels": {
          "app": "gravity-hub"
        },
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{\"service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout\":\"3600\"},\"creationTimestamp\":null,\"labels\":{\"app\":\"gravity-hub\"},\"name\":\"gravity-public\",\"namespace\":\"kube-system\"},\"spec\":{\"ports\":[{\"name\":\"public\",\"port\":443,\"targetPort\":3009},{\"name\":\"sshtunnel\",\"port\":3024,\"targetPort\":0},{\"name\":\"sshproxy\",\"port\":3023,\"targetPort\":0}],\"selector\":{\"app\":\"gravity-site\"},\"type\":\"LoadBalancer\"},\"status\":{\"loadBalancer\":{}}}\n",
          "service.beta.kubernetes.io/aws-load-balancer-connection-idle-timeout": "3600"
        }
      },
      "spec": {
        "ports": [{
            "name": "public",
            "protocol": "TCP",
            "port": 443,
            "targetPort": 3009,
            "nodePort": 32643
          },
          {
            "name": "sshtunnel",
            "protocol": "TCP",
            "port": 3024,
            "targetPort": 3024,
            "nodePort": 30746
          },
          {
            "name": "sshproxy",
            "protocol": "TCP",
            "port": 3023,
            "targetPort": 3023,
            "nodePort": 32358
          }
        ],
        "selector": {
          "app": "gravity-site"
        },
        "clusterIP": "10.100.115.134",
        "type": "LoadBalancer",
        "sessionAffinity": "None",
        "externalTrafficPolicy": "Cluster"
      },
      "status": {
        "loadBalancer": {
          "ingress": [{
            "ip": "104.154.215.175"
          }]
        }
      }
    },
    "namespace": "kube-system",
    "name": "gravity-public",
    "clusterIp": "10.100.115.134",
    "labels": [
      "app:gravity-hub"
    ],
    "ports": [
      "TCP:443/3009",
      "TCP:3024/3024",
      "TCP:3023/3023"
    ]
  },
  {
    "serviceMap": {
      "metadata": {
        "name": "kube-dns",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/kube-dns",
        "uid": "5752d974-0407-11e9-a200-42010a800006",
        "resourceVersion": "333",
        "creationTimestamp": "2018-12-20T03:28:38Z",
        "labels": {
          "k8s-app": "kube-dns",
          "kubernetes.io/cluster-service": "true",
          "kubernetes.io/name": "KubeDNS"
        }
      },
      "spec": {
        "ports": [{
            "name": "dns",
            "protocol": "UDP",
            "port": 53,
            "targetPort": "dns"
          },
          {
            "name": "dns-tcp",
            "protocol": "TCP",
            "port": 53,
            "targetPort": "dns-tcp"
          }
        ],
        "selector": {
          "k8s-app": "kube-dns"
        },
        "clusterIP": "10.100.0.4",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    },
    "namespace": "kube-system",
    "name": "kube-dns",
    "clusterIp": "10.100.0.4",
    "labels": [
      "k8s-app:kube-dns",
      "kubernetes.io/cluster-service:true",
      "kubernetes.io/name:KubeDNS"
    ],
    "ports": [
      "UDP:53/dns",
      "TCP:53/dns-tcp"
    ]
  },
  {
    "serviceMap": {
      "metadata": {
        "name": "log-collector",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/log-collector",
        "uid": "7aa04a83-0407-11e9-a200-42010a800006",
        "resourceVersion": "515",
        "creationTimestamp": "2018-12-20T03:29:38Z",
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"name\":\"log-collector\",\"namespace\":\"kube-system\"},\"spec\":{\"ports\":[{\"name\":\"rsyslog-udp\",\"port\":514,\"protocol\":\"UDP\",\"targetPort\":5514},{\"name\":\"rsyslog-tcp\",\"port\":514,\"protocol\":\"TCP\",\"targetPort\":5514},{\"name\":\"web-tail\",\"port\":8083,\"protocol\":\"TCP\",\"targetPort\":8083}],\"selector\":{\"role\":\"log-collector\"},\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}\n"
        }
      },
      "spec": {
        "ports": [{
            "name": "rsyslog-udp",
            "protocol": "UDP",
            "port": 514,
            "targetPort": 5514
          },
          {
            "name": "rsyslog-tcp",
            "protocol": "TCP",
            "port": 514,
            "targetPort": 5514
          },
          {
            "name": "web-tail",
            "protocol": "TCP",
            "port": 8083,
            "targetPort": 8083
          }
        ],
        "selector": {
          "role": "log-collector"
        },
        "clusterIP": "10.100.76.244",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    },
    "namespace": "kube-system",
    "name": "log-collector",
    "clusterIp": "10.100.76.244",
    "labels": [],
    "ports": [
      "UDP:514/5514",
      "TCP:514/5514",
      "TCP:8083/8083"
    ]
  },
  {
    "serviceMap": {
      "metadata": {
        "name": "bandwagon",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/bandwagon",
        "uid": "7680a294-0407-11e9-a200-42010a800006",
        "resourceVersion": "459",
        "creationTimestamp": "2018-12-20T03:29:31Z",
        "labels": {
          "app": "bandwagon"
        }
      },
      "spec": {
        "ports": [{
          "protocol": "TCP",
          "port": 80,
          "targetPort": 8000
        }],
        "selector": {
          "app": "bandwagon"
        },
        "clusterIP": "10.100.103.89",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {}
      }
    },
    "namespace": "kube-system",
    "name": "bandwagon",
    "clusterIp": "10.100.103.89",
    "labels": [
      "app:bandwagon"
    ],
    "ports": [
      "TCP:80/8000"
    ]
  }
]