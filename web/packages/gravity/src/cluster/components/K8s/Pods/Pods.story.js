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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { Pods } from './Pods'
import { AccessListRec } from 'gravity/flux/userAcl/store';
import { K8sPodDisplayStatusEnum } from 'gravity/services/enums'

const defaultProps = {
  monitoringEnabled: true,
  logsEnabled: true,
  namespace: 'kube-system',
  onFetch: () => $.Deferred(),
  userAcl: new AccessListRec({
    sshLogins: ['one', 'two']
  }),
}

storiesOf('Gravity/K8s', module)
  .add('Pods', () => {
    const props = {
      ...defaultProps,
      podInfos
    }

    return (
      <Router history={createMemoryHistory()}>
        <Pods {...props} />
      </Router>
    );
  })
  .add('Pods (monitoring&logs disabled)', () => {
    const props = {
      ...defaultProps,
      monitoringEnabled: false,
      logsEnabled: false,
      podInfos
    }

    return (
      <Router history={createMemoryHistory()}>
        <Pods {...props} />
      </Router>
    );
  });

const podInfos = [
  {
    "podMap": {
      "metadata": {
        "generateName": "gravity-site-",
        "annotations": {
          "kubernetes.io/psp": "privileged",
          "scheduler.alpha.kubernetes.io/critical-pod": "",
          "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
        },
        "selfLink": "/api/v1/namespaces/kube-system/pods/gravity-site-sl9mk",
        "resourceVersion": "4957048",
        "name": "gravity-site-sl9mk",
        "uid": "c145fe8c-299c-11e9-a200-42010a800006",
        "creationTimestamp": "2019-02-05T23:21:24Z",
        "namespace": "kube-system",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "DaemonSet",
            "name": "gravity-site",
            "uid": "a95925be-0407-11e9-a200-42010a800006",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "labels": {
          "app": "gravity-site",
          "controller-revision-hash": "2853577482",
          "pod-template-generation": "1"
        }
      },
      "spec": {
        "nodeSelector": {
          "gravitational.io/k8s-role": "master"
        },
        "restartPolicy": "Always",
        "serviceAccountName": "default",
        "schedulerName": "default-scheduler",
        "hostNetwork": true,
        "terminationGracePeriodSeconds": 30,
        "nodeName": "demo-gravitational-io-node-0",
        "securityContext": {
          "runAsUser": 1000
        },
        "containers": [
          {
            "resources": {},
            "readinessProbe": {
              "httpGet": {
                "path": "/readyz",
                "port": 3010,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 10,
              "timeoutSeconds": 5,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 3
            },
            "terminationMessagePath": "/dev/termination-log",
            "name": "gravity-site",
            "command": [
              "/usr/bin/dumb-init",
              "/bin/sh",
              "/opt/start.sh"
            ],
            "livenessProbe": {
              "httpGet": {
                "path": "/healthz",
                "port": 3010,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 600,
              "timeoutSeconds": 5,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 3
            },
            "env": [
              {
                "name": "PATH",
                "value": "/opt/gravity:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
              },
              {
                "name": "POD_IP",
                "valueFrom": {
                  "fieldRef": {
                    "apiVersion": "v1",
                    "fieldPath": "status.podIP"
                  }
                }
              },
              {
                "name": "GRAVITY_CONFIG",
                "valueFrom": {
                  "configMapKeyRef": {
                    "name": "gravity-hub",
                    "key": "gravity.yaml"
                  }
                }
              },
              {
                "name": "GRAVITY_TELEPORT_CONFIG",
                "valueFrom": {
                  "configMapKeyRef": {
                    "name": "gravity-hub",
                    "key": "teleport.yaml"
                  }
                }
              }
            ],
            "ports": [
              {
                "name": "web",
                "hostPort": 3009,
                "containerPort": 3009,
                "protocol": "TCP"
              },
              {
                "name": "agents",
                "hostPort": 3007,
                "containerPort": 3007,
                "protocol": "TCP"
              },
              {
                "name": "sshproxy",
                "hostPort": 3023,
                "containerPort": 3023,
                "protocol": "TCP"
              },
              {
                "name": "sshtunnel",
                "hostPort": 3024,
                "containerPort": 3024,
                "protocol": "TCP"
              },
              {
                "name": "teleport",
                "hostPort": 3080,
                "containerPort": 3080,
                "protocol": "TCP"
              },
              {
                "name": "profile",
                "hostPort": 6060,
                "containerPort": 6060,
                "protocol": "TCP"
              }
            ],
            "imagePullPolicy": "Always",
            "volumeMounts": [
              {
                "name": "certs",
                "mountPath": "/etc/ssl/certs"
              },
              {
                "name": "docker-certs",
                "mountPath": "/etc/docker/certs.d"
              },
              {
                "name": "var-state",
                "mountPath": "/var/state"
              },
              {
                "name": "import",
                "mountPath": "/opt/gravity-import"
              },
              {
                "name": "config",
                "mountPath": "/opt/gravity/config"
              },
              {
                "name": "hub-config",
                "mountPath": "/opt/gravity/hub"
              },
              {
                "name": "secrets",
                "mountPath": "/var/lib/gravity/secrets"
              },
              {
                "name": "secrets",
                "mountPath": "/var/lib/gravity/site/secrets"
              },
              {
                "name": "site",
                "mountPath": "/var/lib/gravity/site"
              },
              {
                "name": "tmp",
                "mountPath": "/tmp"
              },
              {
                "name": "kubectl",
                "mountPath": "/usr/bin/kubectl"
              },
              {
                "name": "kubeconfigs",
                "mountPath": "/etc/kubernetes"
              },
              {
                "name": "assets",
                "mountPath": "/usr/local/share/gravity"
              },
              {
                "name": "default-token-bjg6w",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePolicy": "File",
            "image": "leader.telekube.local:5000/gravity-site:5.2.3"
          }
        ],
        "serviceAccount": "default",
        "volumes": [
          {
            "name": "tmp",
            "hostPath": {
              "path": "/tmp",
              "type": ""
            }
          },
          {
            "name": "certs",
            "hostPath": {
              "path": "/etc/ssl/certs",
              "type": ""
            }
          },
          {
            "name": "docker-certs",
            "hostPath": {
              "path": "/etc/docker/certs.d",
              "type": ""
            }
          },
          {
            "name": "var-state",
            "hostPath": {
              "path": "/var/state",
              "type": ""
            }
          },
          {
            "name": "import",
            "hostPath": {
              "path": "/var/lib/gravity/local",
              "type": ""
            }
          },
          {
            "name": "config",
            "configMap": {
              "name": "gravity-site",
              "defaultMode": 420
            }
          },
          {
            "name": "hub-config",
            "configMap": {
              "name": "gravity-hub",
              "defaultMode": 420
            }
          },
          {
            "name": "secrets",
            "hostPath": {
              "path": "/var/lib/gravity/secrets",
              "type": ""
            }
          },
          {
            "name": "site",
            "hostPath": {
              "path": "/var/lib/gravity/site",
              "type": ""
            }
          },
          {
            "name": "kubectl",
            "hostPath": {
              "path": "/usr/bin/kubectl",
              "type": ""
            }
          },
          {
            "name": "kubeconfigs",
            "hostPath": {
              "path": "/etc/kubernetes",
              "type": ""
            }
          },
          {
            "name": "assets",
            "emptyDir": {}
          },
          {
            "name": "default-token-bjg6w",
            "secret": {
              "secretName": "default-token-bjg6w",
              "defaultMode": 420
            }
          }
        ],
        "dnsPolicy": "ClusterFirst",
        "tolerations": [
          {
            "key": "gravitational.io/runlevel",
            "operator": "Equal",
            "value": "system"
          },
          {
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists",
            "effect": "NoSchedule"
          },
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute"
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute"
          },
          {
            "key": "node.kubernetes.io/disk-pressure",
            "operator": "Exists",
            "effect": "NoSchedule"
          },
          {
            "key": "node.kubernetes.io/memory-pressure",
            "operator": "Exists",
            "effect": "NoSchedule"
          }
        ]
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-05T23:21:24Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-05T23:21:38Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-05T23:21:24Z"
          }
        ],
        "hostIP": "10.128.0.6",
        "podIP": "10.128.0.6",
        "startTime": "2019-02-05T23:21:24Z",
        "containerStatuses": [
          {
            "name": "gravity-site",
            "state": {
              "running": {
                "startedAt": "2019-02-05T23:21:25Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/gravity-site:5.2.3",
            "imageID": "docker-pullable://leader.telekube.local:5000/gravity-site@sha256:5b65b93cdb2d93f1b6bb93d9188c0f69fa3a6b605f2e149eb44082207d57e0da",
            "containerID": "docker://14572de8f46abd1167ff0680c78b2e307d6c5f051660dc63d6b78c4201f71d48"
          }
        ],
        "qosClass": "BestEffort"
      }
    },
    "podLogUrl": "/web/site/demo.gravitational.io/logs?query=pod%3Agravity-site-sl9mk",
    "podMonitorUrl": "/web/site/demo.gravitational.io/monitor/dashboard/db/pods?var-namespace=kube-system&var-podname=gravity-site-sl9mk",
    "name": "gravity-site-sl9mk",
    "namespace": "kube-system",
    "podHostIp": "10.128.0.6",
    "podIp": "10.128.0.6",
    "containers": [
      {
        "name": "gravity-site",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Agravity-site",
        "phaseText": "running"
      }
    ],
    "containerNames": [
      "gravity-site"
    ],
    "labelsText": [
      "app:gravity-site",
      "controller-revision-hash:2853577482",
      "pod-template-generation:1"
    ],
    "status": K8sPodDisplayStatusEnum.FAILED,
    "statusDisplay": K8sPodDisplayStatusEnum.FAILED,
    "phaseValue": K8sPodDisplayStatusEnum.FAILED
  },
  {
    "podMap": {
      "metadata": {
        "generateName": "bandwagon-5998fb4dd7-",
        "annotations": {
          "kubernetes.io/psp": "privileged",
          "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
        },
        "selfLink": "/api/v1/namespaces/kube-system/pods/bandwagon-5998fb4dd7-npwn6",
        "resourceVersion": "498",
        "name": "bandwagon-5998fb4dd7-npwn6",
        "uid": "7691b2a6-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:31Z",
        "namespace": "kube-system",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "ReplicaSet",
            "name": "bandwagon-5998fb4dd7",
            "uid": "768d0051-0407-11e9-a200-42010a800006",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "labels": {
          "app": "bandwagon",
          "pod-template-hash": "1554960883"
        }
      },
      "spec": {
        "restartPolicy": "Always",
        "serviceAccountName": "default",
        "schedulerName": "default-scheduler",
        "terminationGracePeriodSeconds": 30,
        "nodeName": "demo-gravitational-io-node-0",
        "securityContext": {
          "runAsUser": 1000
        },
        "containers": [
          {
            "resources": {},
            "terminationMessagePath": "/dev/termination-log",
            "name": "bandwagon",
            "env": [
              {
                "name": "PATH",
                "value": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/bin"
              },
              {
                "name": "POD_IP",
                "valueFrom": {
                  "fieldRef": {
                    "apiVersion": "v1",
                    "fieldPath": "status.podIP"
                  }
                }
              }
            ],
            "ports": [
              {
                "containerPort": 8000,
                "protocol": "TCP"
              }
            ],
            "imagePullPolicy": "Always",
            "volumeMounts": [
              {
                "name": "bin",
                "mountPath": "/opt/bin"
              },
              {
                "name": "gravity",
                "mountPath": "/var/lib/gravity/local"
              },
              {
                "name": "tmp",
                "mountPath": "/tmp"
              },
              {
                "name": "default-token-bjg6w",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePolicy": "File",
            "image": "leader.telekube.local:5000/bandwagon:5.2.3"
          }
        ],
        "serviceAccount": "default",
        "volumes": [
          {
            "name": "bin",
            "hostPath": {
              "path": "/usr/bin",
              "type": ""
            }
          },
          {
            "name": "gravity",
            "hostPath": {
              "path": "/var/lib/gravity/local",
              "type": ""
            }
          },
          {
            "name": "tmp",
            "hostPath": {
              "path": "/tmp",
              "type": ""
            }
          },
          {
            "name": "default-token-bjg6w",
            "secret": {
              "secretName": "default-token-bjg6w",
              "defaultMode": 420
            }
          }
        ],
        "dnsPolicy": "ClusterFirst",
        "tolerations": [
          {
            "key": "gravitational.io/runlevel",
            "operator": "Equal",
            "value": "system"
          },
          {
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists",
            "effect": "NoSchedule"
          }
        ]
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:31Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:34Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:31Z"
          }
        ],
        "hostIP": "10.128.0.6",
        "podIP": "10.244.13.4",
        "startTime": "2018-12-20T03:29:31Z",
        "containerStatuses": [
          {
            "name": "bandwagon",
            "state": {
              "running": {
                "startedAt": "2018-12-20T03:29:34Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/bandwagon:5.2.3",
            "imageID": "docker-pullable://leader.telekube.local:5000/bandwagon@sha256:9015f6c49f3afd541c131eb32f829f91cf48cf9dfe5140e23879a2f5a65b2a06",
            "containerID": "docker://9160910e8a5a006b1e7bc6bdf87618617ec852f333e53249d34ed9b207227b72"
          }
        ],
        "qosClass": "BestEffort"
      }
    },
    "podLogUrl": "/web/site/demo.gravitational.io/logs?query=pod%3Abandwagon-5998fb4dd7-npwn6",
    "podMonitorUrl": "/web/site/demo.gravitational.io/monitor/dashboard/db/pods?var-namespace=kube-system&var-podname=bandwagon-5998fb4dd7-npwn6",
    "name": "bandwagon-5998fb4dd7-npwn6",
    "namespace": "kube-system",
    "podHostIp": "10.128.0.6",
    "podIp": "10.244.13.4",
    "containers": [
      {
        "name": "bandwagon",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Abandwagon",
        "phaseText": "running"
      }
    ],
    "containerNames": [
      "bandwagon"
    ],
    "labelsText": [
      "app:bandwagon",
      "pod-template-hash:1554960883"
    ],
    "status": K8sPodDisplayStatusEnum.PENDING,
    "statusDisplay": K8sPodDisplayStatusEnum.PENDING,
    "phaseValue": K8sPodDisplayStatusEnum.PENDING
  },
  {
    "podMap": {
      "metadata": {
        "generateName": "tiller-deploy-f6b5c5794-",
        "annotations": {
          "kubernetes.io/psp": "privileged",
          "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
        },
        "selfLink": "/api/v1/namespaces/kube-system/pods/tiller-deploy-f6b5c5794-d6txm",
        "resourceVersion": "888",
        "name": "tiller-deploy-f6b5c5794-d6txm",
        "uid": "9bc0afe1-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:30:33Z",
        "namespace": "kube-system",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "ReplicaSet",
            "name": "tiller-deploy-f6b5c5794",
            "uid": "9baf67bc-0407-11e9-a200-42010a800006",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "labels": {
          "app": "helm",
          "name": "tiller",
          "pod-template-hash": "926171350"
        }
      },
      "spec": {
        "restartPolicy": "Always",
        "serviceAccountName": "default",
        "schedulerName": "default-scheduler",
        "terminationGracePeriodSeconds": 30,
        "nodeName": "demo-gravitational-io-node-0",
        "securityContext": {
          "runAsUser": 1000
        },
        "containers": [
          {
            "resources": {},
            "readinessProbe": {
              "httpGet": {
                "path": "/readiness",
                "port": 44135,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 1,
              "timeoutSeconds": 1,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 3
            },
            "terminationMessagePath": "/dev/termination-log",
            "name": "tiller",
            "livenessProbe": {
              "httpGet": {
                "path": "/liveness",
                "port": 44135,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 1,
              "timeoutSeconds": 1,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 3
            },
            "env": [
              {
                "name": "TILLER_NAMESPACE",
                "value": "kube-system"
              }
            ],
            "securityContext": {
              "runAsUser": 1000
            },
            "ports": [
              {
                "name": "tiller",
                "containerPort": 44134,
                "protocol": "TCP"
              }
            ],
            "imagePullPolicy": "Always",
            "volumeMounts": [
              {
                "name": "default-token-bjg6w",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePolicy": "File",
            "image": "leader.telekube.local:5000/kubernetes-helm/tiller:v2.8.1"
          }
        ],
        "serviceAccount": "default",
        "volumes": [
          {
            "name": "default-token-bjg6w",
            "secret": {
              "secretName": "default-token-bjg6w",
              "defaultMode": 420
            }
          }
        ],
        "dnsPolicy": "ClusterFirst",
        "tolerations": [
          {
            "key": "gravitational.io/runlevel",
            "operator": "Equal",
            "value": "system"
          },
          {
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists",
            "effect": "NoSchedule"
          }
        ]
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:30:33Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:30:42Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:30:33Z"
          }
        ],
        "hostIP": "10.128.0.6",
        "podIP": "10.244.13.13",
        "startTime": "2018-12-20T03:30:33Z",
        "containerStatuses": [
          {
            "name": "tiller",
            "state": {
              "running": {
                "startedAt": "2018-12-20T03:30:39Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/kubernetes-helm/tiller:v2.8.1",
            "imageID": "docker-pullable://leader.telekube.local:5000/kubernetes-helm/tiller@sha256:f0af436a310c8c906b7f261fc9d5625596a9791f06ca36b88d556d23a0cecf4e",
            "containerID": "docker://d8c1fd7f1720911846ac4d0d826b681d0b1a95a4f39aa54a6a2c8b38c4949b56"
          }
        ],
        "qosClass": "BestEffort"
      }
    },
    "podLogUrl": "/web/site/demo.gravitational.io/logs?query=pod%3Atiller-deploy-f6b5c5794-d6txm",
    "podMonitorUrl": "/web/site/demo.gravitational.io/monitor/dashboard/db/pods?var-namespace=kube-system&var-podname=tiller-deploy-f6b5c5794-d6txm",
    "name": "tiller-deploy-f6b5c5794-d6txm",
    "namespace": "kube-system",
    "podHostIp": "10.128.0.6",
    "podIp": "10.244.13.13",
    "containers": [
      {
        "name": "tiller",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Atiller",
        "phaseText": "running"
      }
    ],
    "containerNames": [
      "tiller"
    ],
    "labelsText": [
      "app:helm",
      "name:tiller",
      "pod-template-hash:926171350"
    ],
    "status": "Running",
    "statusDisplay": "Running",
    "phaseValue": "Running"
  },
  {
    "podMap": {
      "metadata": {
        "generateName": "kube-dns-",
        "annotations": {
          "kubernetes.io/psp": "privileged",
          "scheduler.alpha.kubernetes.io/critical-pod": "",
          "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
        },
        "selfLink": "/api/v1/namespaces/kube-system/pods/kube-dns-xlrt2",
        "resourceVersion": "663",
        "name": "kube-dns-xlrt2",
        "uid": "71854065-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:22Z",
        "namespace": "kube-system",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "DaemonSet",
            "name": "kube-dns",
            "uid": "71816ae1-0407-11e9-a200-42010a800006",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "labels": {
          "controller-revision-hash": "1166559781",
          "k8s-app": "kube-dns",
          "pod-template-generation": "1"
        }
      },
      "spec": {
        "restartPolicy": "Always",
        "serviceAccountName": "kube-dns",
        "schedulerName": "default-scheduler",
        "terminationGracePeriodSeconds": 30,
        "nodeName": "demo-gravitational-io-node-0",
        "securityContext": {},
        "containers": [
          {
            "resources": {
              "limits": {
                "memory": "170Mi"
              },
              "requests": {
                "cpu": "100m",
                "memory": "70Mi"
              }
            },
            "readinessProbe": {
              "httpGet": {
                "path": "/readiness",
                "port": 8081,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 30,
              "timeoutSeconds": 5,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 3
            },
            "terminationMessagePath": "/dev/termination-log",
            "name": "kubedns",
            "livenessProbe": {
              "httpGet": {
                "path": "/healthcheck/kubedns",
                "port": 10054,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 60,
              "timeoutSeconds": 5,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 5
            },
            "env": [
              {
                "name": "PROMETHEUS_PORT",
                "value": "10055"
              }
            ],
            "securityContext": {
              "runAsUser": 1000
            },
            "ports": [
              {
                "name": "dns-local",
                "containerPort": 10053,
                "protocol": "UDP"
              },
              {
                "name": "dns-tcp-local",
                "containerPort": 10053,
                "protocol": "TCP"
              },
              {
                "name": "metrics",
                "containerPort": 10055,
                "protocol": "TCP"
              }
            ],
            "imagePullPolicy": "Always",
            "volumeMounts": [
              {
                "name": "kube-dns-config",
                "mountPath": "/kube-dns-config"
              },
              {
                "name": "kube-dns-token-r5v5v",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePolicy": "File",
            "image": "leader.telekube.local:5000/k8s-dns-kube-dns-amd64:1.14.10",
            "args": [
              "--domain=cluster.local",
              "--dns-port=10053",
              "--healthz-port=8081",
              "--config-dir=/kube-dns-config",
              "--v=2"
            ]
          },
          {
            "resources": {
              "requests": {
                "cpu": "150m",
                "memory": "20Mi"
              }
            },
            "terminationMessagePath": "/dev/termination-log",
            "name": "dnsmasq",
            "livenessProbe": {
              "httpGet": {
                "path": "/healthcheck/dnsmasq",
                "port": 10054,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 60,
              "timeoutSeconds": 5,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 5
            },
            "securityContext": {
              "runAsUser": 0
            },
            "ports": [
              {
                "name": "dns",
                "containerPort": 53,
                "protocol": "UDP"
              },
              {
                "name": "dns-tcp",
                "containerPort": 53,
                "protocol": "TCP"
              }
            ],
            "imagePullPolicy": "Always",
            "volumeMounts": [
              {
                "name": "kube-dns-config",
                "mountPath": "/etc/k8s/dns/dnsmasq-nanny"
              },
              {
                "name": "kube-dns-token-r5v5v",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePolicy": "File",
            "image": "leader.telekube.local:5000/k8s-dns-dnsmasq-nanny-amd64:1.14.10",
            "args": [
              "-v=2",
              "-logtostderr",
              "-configDir=/etc/k8s/dns/dnsmasq-nanny",
              "-restartDnsmasq=true",
              "--",
              "-k",
              "--cache-size=1000",
              "--no-resolv",
              "--log-facility=-",
              "--server=/cluster.local/127.0.0.1#10053",
              "--server=/in-addr.arpa/127.0.0.1#10053",
              "--server=/ip6.arpa/127.0.0.1#10053"
            ]
          },
          {
            "resources": {
              "requests": {
                "cpu": "10m",
                "memory": "100Mi"
              }
            },
            "terminationMessagePath": "/dev/termination-log",
            "name": "sidecar",
            "livenessProbe": {
              "httpGet": {
                "path": "/metrics",
                "port": 10054,
                "scheme": "HTTP"
              },
              "initialDelaySeconds": 60,
              "timeoutSeconds": 5,
              "periodSeconds": 10,
              "successThreshold": 1,
              "failureThreshold": 5
            },
            "securityContext": {
              "runAsUser": 0
            },
            "ports": [
              {
                "name": "metrics",
                "containerPort": 10054,
                "protocol": "TCP"
              }
            ],
            "imagePullPolicy": "Always",
            "volumeMounts": [
              {
                "name": "kube-dns-token-r5v5v",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePolicy": "File",
            "image": "leader.telekube.local:5000/k8s-dns-sidecar-amd64:1.14.10",
            "args": [
              "--v=2",
              "--logtostderr",
              "--probe=kubedns,127.0.0.1:10053,kubernetes.default.svc.cluster.local,5,A",
              "--probe=dnsmasq,127.0.0.1:53,kubernetes.default.svc.cluster.local,5,A"
            ]
          }
        ],
        "serviceAccount": "kube-dns",
        "volumes": [
          {
            "name": "kube-dns-config",
            "configMap": {
              "name": "kube-dns",
              "defaultMode": 420,
              "optional": true
            }
          },
          {
            "name": "kube-dns-token-r5v5v",
            "secret": {
              "secretName": "kube-dns-token-r5v5v",
              "defaultMode": 420
            }
          }
        ],
        "dnsPolicy": "Default",
        "tolerations": [
          {
            "key": "CriticalAddonsOnly",
            "operator": "Exists"
          },
          {
            "key": "gravitational.io/runlevel",
            "operator": "Equal",
            "value": "system"
          },
          {
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists",
            "effect": "NoSchedule"
          },
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute"
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute"
          },
          {
            "key": "node.kubernetes.io/disk-pressure",
            "operator": "Exists",
            "effect": "NoSchedule"
          },
          {
            "key": "node.kubernetes.io/memory-pressure",
            "operator": "Exists",
            "effect": "NoSchedule"
          }
        ]
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:22Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:30:00Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:22Z"
          }
        ],
        "hostIP": "10.128.0.6",
        "podIP": "10.244.13.3",
        "startTime": "2018-12-20T03:29:22Z",
        "containerStatuses": [
          {
            "name": "dnsmasq",
            "state": {
              "running": {
                "startedAt": "2018-12-20T03:29:28Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/k8s-dns-dnsmasq-nanny-amd64:1.14.10",
            "imageID": "docker-pullable://leader.telekube.local:5000/k8s-dns-dnsmasq-nanny-amd64@sha256:af9e38c7dd94ddf057154e7e4ea99ac02f7be0b40b5a46342dbeb70ecb1c03ec",
            "containerID": "docker://6b3b1a422cf64b55043a90e7e1e0b6742480623e2ba9dc090cddcdaf88da930c"
          },
          {
            "name": "kubedns",
            "state": {
              "running": {
                "startedAt": "2018-12-20T03:29:26Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/k8s-dns-kube-dns-amd64:1.14.10",
            "imageID": "docker-pullable://leader.telekube.local:5000/k8s-dns-kube-dns-amd64@sha256:8030f3e03b7b98fa11f36027760cbe41c2ef8e167051c8e451d1399274944984",
            "containerID": "docker://4d467c881929367f78a8a2f6fa6b08121c75bd307f015150a5b010ae90922672"
          },
          {
            "name": "sidecar",
            "state": {
              "running": {
                "startedAt": "2018-12-20T03:29:31Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/k8s-dns-sidecar-amd64:1.14.10",
            "imageID": "docker-pullable://leader.telekube.local:5000/k8s-dns-sidecar-amd64@sha256:4030e4ceb2c2aada6c949bd6036a2ce042979f5705f43d2f857c2ab348f28ad9",
            "containerID": "docker://8f90faf5e7885b1b2ce96e471c40630c2bcf85dc1d9f777e477a4cf547ccad67"
          }
        ],
        "qosClass": "Burstable"
      }
    },
    "podLogUrl": "/web/site/demo.gravitational.io/logs?query=pod%3Akube-dns-xlrt2",
    "podMonitorUrl": "/web/site/demo.gravitational.io/monitor/dashboard/db/pods?var-namespace=kube-system&var-podname=kube-dns-xlrt2",
    "name": "kube-dns-xlrt2",
    "namespace": "kube-system",
    "podHostIp": "10.128.0.6",
    "podIp": "10.244.13.3",
    "containers": [
      {
        "name": "dnsmasq",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Adnsmasq",
        "phaseText": "running"
      },
      {
        "name": "kubedns",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Akubedns",
        "phaseText": "running"
      },
      {
        "name": "sidecar",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Asidecar",
        "phaseText": "running"
      }
    ],
    "containerNames": [
      "kubedns",
      "dnsmasq",
      "sidecar"
    ],
    "labelsText": [
      "controller-revision-hash:1166559781",
      "k8s-app:kube-dns",
      "pod-template-generation:1"
    ],
    "status": K8sPodDisplayStatusEnum.TERMINATED,
    "statusDisplay": K8sPodDisplayStatusEnum.TERMINATED,
    "phaseValue": K8sPodDisplayStatusEnum.TERMINATED
  },
  {
    "podMap": {
      "metadata": {
        "generateName": "log-forwarder-",
        "annotations": {
          "kubernetes.io/psp": "privileged",
          "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
        },
        "selfLink": "/api/v1/namespaces/kube-system/pods/log-forwarder-jnd9j",
        "resourceVersion": "576",
        "name": "log-forwarder-jnd9j",
        "uid": "7abd70ee-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:38Z",
        "namespace": "kube-system",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "DaemonSet",
            "name": "log-forwarder",
            "uid": "7ab59a68-0407-11e9-a200-42010a800006",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "labels": {
          "controller-revision-hash": "3833365929",
          "name": "log-forwarder",
          "pod-template-generation": "1"
        }
      },
      "spec": {
        "restartPolicy": "Always",
        "serviceAccountName": "default",
        "schedulerName": "default-scheduler",
        "terminationGracePeriodSeconds": 30,
        "nodeName": "demo-gravitational-io-node-0",
        "securityContext": {
          "runAsUser": 0
        },
        "containers": [
          {
            "name": "log-forwarder",
            "image": "leader.telekube.local:5000/log-forwarder:5.0.2",
            "resources": {
              "limits": {
                "cpu": "500m",
                "memory": "600Mi"
              },
              "requests": {
                "cpu": "100m",
                "memory": "200Mi"
              }
            },
            "volumeMounts": [
              {
                "name": "varlog",
                "mountPath": "/var/log"
              },
              {
                "name": "extdockercontainers",
                "mountPath": "/ext/docker/containers"
              },
              {
                "name": "gravitysite",
                "mountPath": "/var/lib/gravity/site"
              },
              {
                "name": "default-token-bjg6w",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "serviceAccount": "default",
        "volumes": [
          {
            "name": "varlog",
            "hostPath": {
              "path": "/var/log",
              "type": ""
            }
          },
          {
            "name": "extdockercontainers",
            "hostPath": {
              "path": "/ext/docker/containers",
              "type": ""
            }
          },
          {
            "name": "gravitysite",
            "hostPath": {
              "path": "/var/lib/gravity/site",
              "type": ""
            }
          },
          {
            "name": "default-token-bjg6w",
            "secret": {
              "secretName": "default-token-bjg6w",
              "defaultMode": 420
            }
          }
        ],
        "dnsPolicy": "ClusterFirst",
        "tolerations": [
          {
            "key": "gravitational.io/runlevel",
            "operator": "Equal",
            "value": "system"
          },
          {
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists",
            "effect": "NoSchedule"
          },
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute"
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute"
          },
          {
            "key": "node.kubernetes.io/disk-pressure",
            "operator": "Exists",
            "effect": "NoSchedule"
          },
          {
            "key": "node.kubernetes.io/memory-pressure",
            "operator": "Exists",
            "effect": "NoSchedule"
          }
        ]
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:38Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:50Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2018-12-20T03:29:38Z"
          }
        ],
        "hostIP": "10.128.0.6",
        "podIP": "10.244.13.6",
        "startTime": "2018-12-20T03:29:38Z",
        "containerStatuses": [
          {
            "name": "log-forwarder",
            "state": {
              "running": {
                "startedAt": "2018-12-20T03:29:49Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/log-forwarder:5.0.2",
            "imageID": "docker-pullable://leader.telekube.local:5000/log-forwarder@sha256:b37964294055eee57155470ee220098b1a415485b16bdf48f6eff76272ec4336",
            "containerID": "docker://3c87b9b74c0643064127498030fc644b485e4804d58dd574e0922ab85eeb356c"
          }
        ],
        "qosClass": "Burstable"
      }
    },
    "podLogUrl": "/web/site/demo.gravitational.io/logs?query=pod%3Alog-forwarder-jnd9j",
    "podMonitorUrl": "/web/site/demo.gravitational.io/monitor/dashboard/db/pods?var-namespace=kube-system&var-podname=log-forwarder-jnd9j",
    "name": "log-forwarder-jnd9j",
    "namespace": "kube-system",
    "podHostIp": "10.128.0.6",
    "podIp": "10.244.13.6",
    "containers": [
      {
        "name": "log-forwarder",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Alog-forwarder",
        "phaseText": "running"
      }
    ],
    "containerNames": [
      "log-forwarder"
    ],
    "labelsText": [
      "name:log-forwarder",
      "controller-revision-hash:3833365929",
      "pod-template-generation:1"
    ],
    "status": K8sPodDisplayStatusEnum.UNKNOWN,
    "statusDisplay": K8sPodDisplayStatusEnum.UNKNOWN,
    "phaseValue": K8sPodDisplayStatusEnum.UNKNOWN
  },
  {
    "podMap": {
      "metadata": {
        "generateName": "log-collector-5746856987-",
        "annotations": {
          "kubernetes.io/psp": "privileged",
          "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
        },
        "selfLink": "/api/v1/namespaces/kube-system/pods/log-collector-5746856987-m8gcj",
        "resourceVersion": "6409897",
        "name": "log-collector-5746856987-m8gcj",
        "uid": "e929d762-34a1-11e9-a200-42010a800006",
        "creationTimestamp": "2019-02-19T23:56:01Z",
        "namespace": "kube-system",
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "ReplicaSet",
            "name": "log-collector-5746856987",
            "uid": "7aaf6e33-0407-11e9-a200-42010a800006",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "labels": {
          "pod-template-hash": "1302412543",
          "role": "log-collector",
          "version": "v1"
        }
      },
      "spec": {
        "restartPolicy": "Always",
        "initContainers": [
          {
            "name": "init-log-forwarders",
            "image": "leader.telekube.local:5000/log-collector:5.0.2",
            "command": [
              "/wstail",
              "-init-forwarders"
            ],
            "resources": {},
            "volumeMounts": [
              {
                "name": "logforwarders",
                "mountPath": "/etc/rsyslog.d"
              },
              {
                "name": "default-token-bjg6w",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "serviceAccountName": "default",
        "schedulerName": "default-scheduler",
        "terminationGracePeriodSeconds": 30,
        "nodeName": "demo-gravitational-io-node-0",
        "securityContext": {
          "runAsUser": 0
        },
        "containers": [
          {
            "name": "log-collector",
            "image": "leader.telekube.local:5000/log-collector:5.0.2",
            "ports": [
              {
                "name": "udp",
                "containerPort": 514,
                "protocol": "UDP"
              },
              {
                "name": "tcp",
                "containerPort": 514,
                "protocol": "TCP"
              },
              {
                "name": "web-tail",
                "containerPort": 8083,
                "protocol": "TCP"
              }
            ],
            "resources": {
              "limits": {
                "cpu": "500m",
                "memory": "600Mi"
              },
              "requests": {
                "cpu": "100m",
                "memory": "200Mi"
              }
            },
            "volumeMounts": [
              {
                "name": "varlog",
                "mountPath": "/var/log"
              },
              {
                "name": "logforwarders",
                "mountPath": "/etc/rsyslog.d"
              },
              {
                "name": "default-token-bjg6w",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "serviceAccount": "default",
        "volumes": [
          {
            "name": "logforwarders",
            "emptyDir": {}
          },
          {
            "name": "varlog",
            "hostPath": {
              "path": "/var/log",
              "type": ""
            }
          },
          {
            "name": "default-token-bjg6w",
            "secret": {
              "secretName": "default-token-bjg6w",
              "defaultMode": 420
            }
          }
        ],
        "dnsPolicy": "ClusterFirst",
        "tolerations": [
          {
            "key": "gravitational.io/runlevel",
            "operator": "Equal",
            "value": "system"
          },
          {
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists",
            "effect": "NoSchedule"
          }
        ]
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-19T23:56:03Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-19T23:56:04Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-19T23:56:01Z"
          }
        ],
        "hostIP": "10.128.0.6",
        "podIP": "10.244.13.2",
        "startTime": "2019-02-19T23:56:01Z",
        "initContainerStatuses": [
          {
            "name": "init-log-forwarders",
            "state": {
              "terminated": {
                "exitCode": 0,
                "reason": "Completed",
                "startedAt": "2019-02-19T23:56:02Z",
                "finishedAt": "2019-02-19T23:56:03Z",
                "containerID": "docker://fdf37fc13053aa66ffdafc5740187d202539e76b200e9139e748964389ae9b5f"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/log-collector:5.0.2",
            "imageID": "docker-pullable://leader.telekube.local:5000/log-collector@sha256:37833c2e430994a860ebb48b9c2ee023295b283ec4ed6f51ee198124d11de37c",
            "containerID": "docker://fdf37fc13053aa66ffdafc5740187d202539e76b200e9139e748964389ae9b5f"
          }
        ],
        "containerStatuses": [
          {
            "name": "log-collector",
            "state": {
              "running": {
                "startedAt": "2019-02-19T23:56:04Z"
              }
            },
            "lastState": {},
            "ready": true,
            "restartCount": 0,
            "image": "leader.telekube.local:5000/log-collector:5.0.2",
            "imageID": "docker-pullable://leader.telekube.local:5000/log-collector@sha256:37833c2e430994a860ebb48b9c2ee023295b283ec4ed6f51ee198124d11de37c",
            "containerID": "docker://16330617e238e853ee00fbfd9ef41e64106a763d2501aa7d681ed14380fc411e"
          }
        ],
        "qosClass": "Burstable"
      }
    },
    "podLogUrl": "/web/site/demo.gravitational.io/logs?query=pod%3Alog-collector-5746856987-m8gcj",
    "podMonitorUrl": "/web/site/demo.gravitational.io/monitor/dashboard/db/pods?var-namespace=kube-system&var-podname=log-collector-5746856987-m8gcj",
    "name": "log-collector-5746856987-m8gcj",
    "namespace": "kube-system",
    "podHostIp": "10.128.0.6",
    "podIp": "10.244.13.2",
    "containers": [
      {
        "name": "log-collector",
        "logUrl": "/web/site/demo.gravitational.io/logs?query=container%3Alog-collector",
        "phaseText": "running"
      }
    ],
    "containerNames": [
      "log-collector"
    ],
    "labelsText": [
      "pod-template-hash:1302412543",
      "role:log-collector",
      "version:v1"
    ],
    "status": "Running",
    "statusDisplay": "Running",
    "phaseValue": "Running"
  }
]