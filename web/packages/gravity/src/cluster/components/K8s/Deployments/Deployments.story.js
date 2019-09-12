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
import { Deployments } from './Deployments'

storiesOf('Gravity/K8s', module)
  .add('Deployments', () => {
    const props = {
      namespace: 'kube-system',
      onFetch: () => $.Deferred(),
      deployments
    }

    return (
      <Deployments {...props} />
    );
  });

const deployments = [
  {
    "nodeMap": {
      "metadata": {
        "annotations": {
          "deployment.kubernetes.io/revision": "1",
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"labels\":{\"app\":\"helm\",\"name\":\"tiller\"},\"name\":\"tiller-deploy\",\"namespace\":\"kube-system\"},\"spec\":{\"replicas\":1,\"strategy\":{},\"template\":{\"metadata\":{\"annotations\":{\"seccomp.security.alpha.kubernetes.io/pod\":\"docker/default\"},\"creationTimestamp\":null,\"labels\":{\"app\":\"helm\",\"name\":\"tiller\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"TILLER_NAMESPACE\",\"value\":\"kube-system\"}],\"image\":\"leader.telekube.local:5000/kubernetes-helm/tiller:v2.8.1\",\"imagePullPolicy\":\"IfNotPresent\",\"livenessProbe\":{\"httpGet\":{\"path\":\"/liveness\",\"port\":44135},\"initialDelaySeconds\":1,\"timeoutSeconds\":1},\"name\":\"tiller\",\"ports\":[{\"containerPort\":44134,\"name\":\"tiller\",\"protocol\":\"TCP\"}],\"readinessProbe\":{\"httpGet\":{\"path\":\"/readiness\",\"port\":44135},\"initialDelaySeconds\":1,\"timeoutSeconds\":1},\"resources\":{},\"securityContext\":{\"runAsUser\":1000}}],\"securityContext\":{\"runAsUser\":1000},\"tolerations\":[{\"key\":\"gravitational.io/runlevel\",\"operator\":\"Equal\",\"value\":\"system\"},{\"effect\":\"NoSchedule\",\"key\":\"node-role.kubernetes.io/master\",\"operator\":\"Exists\"}]}}},\"status\":{}}\n"
        },
        "selfLink": "/apis/extensions/v1beta1/namespaces/kube-system/deployments/tiller-deploy",
        "resourceVersion": "891",
        "name": "tiller-deploy",
        "uid": "9baab330-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:30:33Z",
        "generation": 1,
        "namespace": "kube-system",
        "labels": {
          "app": "helm",
          "name": "tiller"
        }
      },
      "spec": {
        "replicas": 1,
        "selector": {
          "matchLabels": {
            "app": "helm",
            "name": "tiller"
          }
        },
        "template": {
          "metadata": {
            "creationTimestamp": null,
            "labels": {
              "app": "helm",
              "name": "tiller"
            },
            "annotations": {
              "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
            }
          },
          "spec": {
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
                "imagePullPolicy": "IfNotPresent",
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/kubernetes-helm/tiller:v2.8.1"
              }
            ],
            "restartPolicy": "Always",
            "terminationGracePeriodSeconds": 30,
            "dnsPolicy": "ClusterFirst",
            "securityContext": {
              "runAsUser": 1000
            },
            "schedulerName": "default-scheduler",
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
          }
        },
        "strategy": {
          "type": "RollingUpdate",
          "rollingUpdate": {
            "maxUnavailable": 1,
            "maxSurge": 1
          }
        },
        "revisionHistoryLimit": 10,
        "progressDeadlineSeconds": 600
      },
      "status": {
        "observedGeneration": 1,
        "replicas": 1,
        "updatedReplicas": 1,
        "readyReplicas": 1,
        "availableReplicas": 1,
        "conditions": [
          {
            "type": "Available",
            "status": "True",
            "lastUpdateTime": "2018-12-20T03:30:33Z",
            "lastTransitionTime": "2018-12-20T03:30:33Z",
            "reason": "MinimumReplicasAvailable",
            "message": "Deployment has minimum availability."
          },
          {
            "type": "Progressing",
            "status": "True",
            "lastUpdateTime": "2018-12-20T03:30:42Z",
            "lastTransitionTime": "2018-12-20T03:30:33Z",
            "reason": "NewReplicaSetAvailable",
            "message": "ReplicaSet \"tiller-deploy-f6b5c5794\" has successfully progressed."
          }
        ]
      }
    },
    "name": "tiller-deploy",
    "namespace": "kube-system",
    "created": "2018-12-20T03:30:33.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusCurrentReplicas": 1,
    "statusUpdatedReplicas": 1,
    "statusAvailableReplicas": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "annotations": {
          "deployment.kubernetes.io/revision": "1",
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"labels\":{\"name\":\"log-collector\",\"role\":\"log-collector\",\"version\":\"v1\"},\"name\":\"log-collector\",\"namespace\":\"kube-system\"},\"spec\":{\"replicas\":1,\"selector\":{\"matchLabels\":{\"role\":\"log-collector\",\"version\":\"v1\"}},\"strategy\":{},\"template\":{\"metadata\":{\"annotations\":{\"seccomp.security.alpha.kubernetes.io/pod\":\"docker/default\"},\"creationTimestamp\":null,\"labels\":{\"role\":\"log-collector\",\"version\":\"v1\"}},\"spec\":{\"containers\":[{\"image\":\"leader.telekube.local:5000/log-collector:5.0.2\",\"name\":\"log-collector\",\"ports\":[{\"containerPort\":514,\"name\":\"udp\",\"protocol\":\"UDP\"},{\"containerPort\":514,\"name\":\"tcp\",\"protocol\":\"TCP\"},{\"containerPort\":8083,\"name\":\"web-tail\",\"protocol\":\"TCP\"}],\"resources\":{\"limits\":{\"cpu\":\"500m\",\"memory\":\"600Mi\"},\"requests\":{\"cpu\":\"100m\",\"memory\":\"200Mi\"}},\"volumeMounts\":[{\"mountPath\":\"/var/log\",\"name\":\"varlog\"},{\"mountPath\":\"/etc/rsyslog.d\",\"name\":\"logforwarders\"}]}],\"initContainers\":[{\"command\":[\"/wstail\",\"-init-forwarders\"],\"image\":\"leader.telekube.local:5000/log-collector:5.0.2\",\"name\":\"init-log-forwarders\",\"resources\":{},\"volumeMounts\":[{\"mountPath\":\"/etc/rsyslog.d\",\"name\":\"logforwarders\"}]}],\"securityContext\":{\"runAsUser\":0},\"tolerations\":[{\"key\":\"gravitational.io/runlevel\",\"operator\":\"Equal\",\"value\":\"system\"},{\"effect\":\"NoSchedule\",\"key\":\"node-role.kubernetes.io/master\",\"operator\":\"Exists\"}],\"volumes\":[{\"emptyDir\":{},\"name\":\"logforwarders\"},{\"hostPath\":{\"path\":\"/var/log\"},\"name\":\"varlog\"}]}}},\"status\":{}}\n"
        },
        "selfLink": "/apis/extensions/v1beta1/namespaces/kube-system/deployments/log-collector",
        "resourceVersion": "6409899",
        "name": "log-collector",
        "uid": "7aaece11-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:38Z",
        "generation": 1,
        "namespace": "kube-system",
        "labels": {
          "name": "log-collector",
          "role": "log-collector",
          "version": "v1"
        }
      },
      "spec": {
        "replicas": 1,
        "selector": {
          "matchLabels": {
            "role": "log-collector",
            "version": "v1"
          }
        },
        "template": {
          "metadata": {
            "creationTimestamp": null,
            "labels": {
              "role": "log-collector",
              "version": "v1"
            },
            "annotations": {
              "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
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
                  }
                ],
                "terminationMessagePath": "/dev/termination-log",
                "terminationMessagePolicy": "File",
                "imagePullPolicy": "IfNotPresent"
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
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
                  }
                ],
                "terminationMessagePath": "/dev/termination-log",
                "terminationMessagePolicy": "File",
                "imagePullPolicy": "IfNotPresent"
              }
            ],
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
          }
        },
        "strategy": {
          "type": "RollingUpdate",
          "rollingUpdate": {
            "maxUnavailable": 1,
            "maxSurge": 1
          }
        },
        "revisionHistoryLimit": 10,
        "progressDeadlineSeconds": 600
      },
      "status": {
        "observedGeneration": 1,
        "replicas": 1,
        "updatedReplicas": 1,
        "readyReplicas": 1,
        "availableReplicas": 1,
        "conditions": [
          {
            "type": "Available",
            "status": "True",
            "lastUpdateTime": "2018-12-20T03:29:38Z",
            "lastTransitionTime": "2018-12-20T03:29:38Z",
            "reason": "MinimumReplicasAvailable",
            "message": "Deployment has minimum availability."
          },
          {
            "type": "Progressing",
            "status": "True",
            "lastUpdateTime": "2018-12-20T03:29:51Z",
            "lastTransitionTime": "2018-12-20T03:29:38Z",
            "reason": "NewReplicaSetAvailable",
            "message": "ReplicaSet \"log-collector-5746856987\" has successfully progressed."
          }
        ]
      }
    },
    "name": "log-collector",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:38.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusCurrentReplicas": 1,
    "statusUpdatedReplicas": 1,
    "statusAvailableReplicas": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "annotations": {
          "deployment.kubernetes.io/revision": "1"
        },
        "selfLink": "/apis/extensions/v1beta1/namespaces/kube-system/deployments/bandwagon",
        "resourceVersion": "502",
        "name": "bandwagon",
        "uid": "7688ec29-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:31Z",
        "generation": 1,
        "namespace": "kube-system",
        "labels": {
          "app": "bandwagon"
        }
      },
      "spec": {
        "replicas": 1,
        "selector": {
          "matchLabels": {
            "app": "bandwagon"
          }
        },
        "template": {
          "metadata": {
            "creationTimestamp": null,
            "labels": {
              "app": "bandwagon"
            },
            "annotations": {
              "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
            }
          },
          "spec": {
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
              }
            ],
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
                "imagePullPolicy": "IfNotPresent",
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
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/bandwagon:5.2.3"
              }
            ],
            "restartPolicy": "Always",
            "terminationGracePeriodSeconds": 30,
            "dnsPolicy": "ClusterFirst",
            "securityContext": {
              "runAsUser": 1000
            },
            "schedulerName": "default-scheduler",
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
          }
        },
        "strategy": {
          "type": "RollingUpdate",
          "rollingUpdate": {
            "maxUnavailable": 1,
            "maxSurge": 1
          }
        },
        "revisionHistoryLimit": 10,
        "progressDeadlineSeconds": 600
      },
      "status": {
        "observedGeneration": 1,
        "replicas": 1,
        "updatedReplicas": 1,
        "readyReplicas": 1,
        "availableReplicas": 1,
        "conditions": [
          {
            "type": "Available",
            "status": "True",
            "lastUpdateTime": "2018-12-20T03:29:31Z",
            "lastTransitionTime": "2018-12-20T03:29:31Z",
            "reason": "MinimumReplicasAvailable",
            "message": "Deployment has minimum availability."
          },
          {
            "type": "Progressing",
            "status": "True",
            "lastUpdateTime": "2018-12-20T03:29:35Z",
            "lastTransitionTime": "2018-12-20T03:29:31Z",
            "reason": "NewReplicaSetAvailable",
            "message": "ReplicaSet \"bandwagon-5998fb4dd7\" has successfully progressed."
          }
        ]
      }
    },
    "name": "bandwagon",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:31.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusCurrentReplicas": 1,
    "statusUpdatedReplicas": 1,
    "statusAvailableReplicas": 1
  }
]