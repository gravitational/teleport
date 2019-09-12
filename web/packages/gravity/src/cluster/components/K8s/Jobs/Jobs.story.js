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
import { Jobs } from './Jobs'

storiesOf('Gravity/K8s', module)
  .add('Jobs', () => {
    const props = {
      onFetch: () => $.Deferred(),
      jobs,
      namespace: 'kube-system',
    }

    return (
      <Jobs {...props} />
    );
  });


const jobs = [
  {
    "nodeMap": {
      "metadata": {
        "name": "site-app-post-install-f3201e",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/site-app-post-install-f3201e",
        "uid": "a9a641f6-0407-11e9-a200-42010a800006",
        "resourceVersion": "1029",
        "creationTimestamp": "2018-12-20T03:30:56Z",
        "labels": {
          "controller-uid": "a9a641f6-0407-11e9-a200-42010a800006",
          "job-name": "site-app-post-install-f3201e"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "a9a641f6-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "site-app-post-install",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "a9a641f6-0407-11e9-a200-42010a800006",
              "job-name": "site-app-post-install-f3201e"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/site:5.2.3"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/site:5.2.3 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 0,
              "runAsNonRoot": false,
              "fsGroup": 0
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "post-install-hook",
                "command": [
                  "/opt/bin/gravity",
                  "site",
                  "status"
                ],
                "env": [
                  {
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1"
              }
            ],
            "volumes": [
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:31:40Z",
            "lastTransitionTime": "2018-12-20T03:31:40Z"
          }
        ],
        "startTime": "2018-12-20T03:30:56Z",
        "completionTime": "2018-12-20T03:31:40Z",
        "succeeded": 1
      }
    },
    "name": "site-app-post-install-f3201e",
    "namespace": "kube-system",
    "created": "2018-12-20T03:30:56.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusSucceeded": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "name": "install-telekube-c8e0a9",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/install-telekube-c8e0a9",
        "uid": "9d6d4077-0407-11e9-a200-42010a800006",
        "resourceVersion": "927",
        "creationTimestamp": "2018-12-20T03:30:36Z",
        "labels": {
          "app": "gravity-site"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "9d6d4077-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "install-telekube",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "9d6d4077-0407-11e9-a200-42010a800006",
              "job-name": "install-telekube-c8e0a9"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/site:5.2.3"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/site:5.2.3 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "hostNetwork": true,
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 1000
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "install-telekube",
                "command": [
                  "/bin/sh",
                  "/opt/init.sh"
                ],
                "env": [
                  {
                    "name": "HOME",
                    "value": "/home"
                  },
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
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "import",
                    "mountPath": "/opt/gravity-import"
                  },
                  {
                    "name": "tmp",
                    "mountPath": "/tmp"
                  },
                  {
                    "name": "site",
                    "mountPath": "/var/lib/gravity/site"
                  },
                  {
                    "name": "secrets",
                    "mountPath": "/var/lib/gravity/secrets"
                  },
                  {
                    "name": "home",
                    "mountPath": "/home"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravity-site:5.2.3"
              }
            ],
            "volumes": [
              {
                "name": "tmp",
                "hostPath": {
                  "path": "/tmp",
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
                "name": "site",
                "hostPath": {
                  "path": "/var/lib/gravity/site",
                  "type": ""
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
                "name": "home",
                "emptyDir": {}
              },
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:30:56Z",
            "lastTransitionTime": "2018-12-20T03:30:56Z"
          }
        ],
        "startTime": "2018-12-20T03:30:36Z",
        "completionTime": "2018-12-20T03:30:56Z",
        "succeeded": 1
      }
    },
    "name": "install-telekube-c8e0a9",
    "namespace": "kube-system",
    "created": "2018-12-20T03:30:36.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusFailed": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "name": "tiller-app-bootstrap-f5d0b0",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/tiller-app-bootstrap-f5d0b0",
        "uid": "8ad042c5-0407-11e9-a200-42010a800006",
        "resourceVersion": "859",
        "creationTimestamp": "2018-12-20T03:30:05Z",
        "labels": {
          "controller-uid": "8ad042c5-0407-11e9-a200-42010a800006",
          "job-name": "tiller-app-bootstrap-f5d0b0"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "8ad042c5-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "tiller-app-bootstrap",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "8ad042c5-0407-11e9-a200-42010a800006",
              "job-name": "tiller-app-bootstrap-f5d0b0"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/tiller-app:5.2.1"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/tiller-app:5.2.1 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 0,
              "runAsNonRoot": false,
              "fsGroup": 0
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "hook",
                "command": [
                  "/usr/local/bin/kubectl",
                  "apply",
                  "-f",
                  "/var/lib/gravity/resources/resources.yaml"
                ],
                "env": [
                  {
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1"
              }
            ],
            "volumes": [
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:30:34Z",
            "lastTransitionTime": "2018-12-20T03:30:34Z"
          }
        ],
        "startTime": "2018-12-20T03:30:05Z",
        "completionTime": "2018-12-20T03:30:34Z",
        "succeeded": 1
      }
    },
    "name": "tiller-app-bootstrap-f5d0b0",
    "namespace": "kube-system",
    "created": "2018-12-20T03:30:05.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusActive": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "name": "monitoring-app-install-573f4f",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/monitoring-app-install-573f4f",
        "uid": "7c06a399-0407-11e9-a200-42010a800006",
        "resourceVersion": "729",
        "creationTimestamp": "2018-12-20T03:29:40Z",
        "labels": {
          "controller-uid": "7c06a399-0407-11e9-a200-42010a800006",
          "job-name": "monitoring-app-install-573f4f"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "7c06a399-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "monitoring-app-install",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "7c06a399-0407-11e9-a200-42010a800006",
              "job-name": "monitoring-app-install-573f4f"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/monitoring-app:5.2.2"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/monitoring-app:5.2.2 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 0,
              "runAsNonRoot": false,
              "fsGroup": 0
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "hook",
                "command": [
                  "/bin/sh",
                  "/var/lib/gravity/resources/install.sh"
                ],
                "env": [
                  {
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:stretch"
              }
            ],
            "volumes": [
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:30:03Z",
            "lastTransitionTime": "2018-12-20T03:30:03Z"
          }
        ],
        "startTime": "2018-12-20T03:29:40Z",
        "completionTime": "2018-12-20T03:30:03Z",
        "succeeded": 1
      }
    },
    "name": "monitoring-app-install-573f4f",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:40.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusSucceeded": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "name": "logging-app-bootstrap-7eaf74",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/logging-app-bootstrap-7eaf74",
        "uid": "77bbcb5c-0407-11e9-a200-42010a800006",
        "resourceVersion": "542",
        "creationTimestamp": "2018-12-20T03:29:33Z",
        "labels": {
          "controller-uid": "77bbcb5c-0407-11e9-a200-42010a800006",
          "job-name": "logging-app-bootstrap-7eaf74"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "77bbcb5c-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "logging-app-bootstrap",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "77bbcb5c-0407-11e9-a200-42010a800006",
              "job-name": "logging-app-bootstrap-7eaf74"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/logging-app:5.0.2"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/logging-app:5.0.2 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 0,
              "runAsNonRoot": false,
              "fsGroup": 0
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "hook",
                "command": [
                  "/usr/local/bin/kubectl",
                  "apply",
                  "-f",
                  "/var/lib/gravity/resources/resources.yaml"
                ],
                "env": [
                  {
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1"
              }
            ],
            "volumes": [
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:29:39Z",
            "lastTransitionTime": "2018-12-20T03:29:39Z"
          }
        ],
        "startTime": "2018-12-20T03:29:33Z",
        "completionTime": "2018-12-20T03:29:39Z",
        "succeeded": 1
      }
    },
    "name": "logging-app-bootstrap-7eaf74",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:33.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusSucceeded": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "name": "bandwagon-install-a67137",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/bandwagon-install-a67137",
        "uid": "72c763b5-0407-11e9-a200-42010a800006",
        "resourceVersion": "480",
        "creationTimestamp": "2018-12-20T03:29:24Z",
        "labels": {
          "controller-uid": "72c763b5-0407-11e9-a200-42010a800006",
          "job-name": "bandwagon-install-a67137"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "72c763b5-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "bandwagon-install",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "72c763b5-0407-11e9-a200-42010a800006",
              "job-name": "bandwagon-install-a67137"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/bandwagon:5.2.3"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/bandwagon:5.2.3 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 0,
              "runAsNonRoot": false,
              "fsGroup": 0
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "debian-tall",
                "command": [
                  "/usr/local/bin/kubectl",
                  "create",
                  "-f",
                  "/var/lib/gravity/resources/install.yaml"
                ],
                "env": [
                  {
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1"
              }
            ],
            "volumes": [
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:29:31Z",
            "lastTransitionTime": "2018-12-20T03:29:31Z"
          }
        ],
        "startTime": "2018-12-20T03:29:24Z",
        "completionTime": "2018-12-20T03:29:31Z",
        "succeeded": 1
      }
    },
    "name": "bandwagon-install-a67137",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:24.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusSucceeded": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "name": "dns-app-install-4f2ffc",
        "namespace": "kube-system",
        "selfLink": "/apis/batch/v1/namespaces/kube-system/jobs/dns-app-install-4f2ffc",
        "uid": "6ef6c747-0407-11e9-a200-42010a800006",
        "resourceVersion": "417",
        "creationTimestamp": "2018-12-20T03:29:18Z",
        "labels": {
          "controller-uid": "6ef6c747-0407-11e9-a200-42010a800006",
          "job-name": "dns-app-install-4f2ffc"
        }
      },
      "spec": {
        "parallelism": 1,
        "completions": 1,
        "activeDeadlineSeconds": 1200,
        "backoffLimit": 6,
        "selector": {
          "matchLabels": {
            "controller-uid": "6ef6c747-0407-11e9-a200-42010a800006"
          }
        },
        "template": {
          "metadata": {
            "name": "dns-app-install",
            "creationTimestamp": null,
            "labels": {
              "controller-uid": "6ef6c747-0407-11e9-a200-42010a800006",
              "job-name": "dns-app-install-4f2ffc"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "OnFailure",
            "initContainers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "init",
                "command": [
                  "/bin/sh",
                  "-c",
                  "-e"
                ],
                "env": [
                  {
                    "name": "APP_PACKAGE",
                    "value": "gravitational.io/dns-app:0.1.0"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "gravity",
                    "mountPath": "/var/lib/gravity/local"
                  },
                  {
                    "name": "state-dir",
                    "mountPath": "/tmp/state"
                  },
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1",
                "args": [
                  "\nTMPDIR=/tmp/state /opt/bin/gravity app unpack --service-uid=1000 gravitational.io/dns-app:0.1.0 /var/lib/gravity/resources\nmv /var/lib/gravity/resources/resources/* /var/lib/gravity/resources\nrm -r /var/lib/gravity/resources/resources\n"
                ]
              }
            ],
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
            "securityContext": {
              "runAsUser": 0,
              "runAsNonRoot": false,
              "fsGroup": 0
            },
            "containers": [
              {
                "resources": {},
                "terminationMessagePath": "/dev/termination-log",
                "name": "hook",
                "command": [
                  "/usr/local/bin/kubectl",
                  "apply",
                  "-f",
                  "/var/lib/gravity/resources/dns.yaml"
                ],
                "env": [
                  {
                    "name": "DEVMODE",
                    "value": "false"
                  },
                  {
                    "name": "GRAVITY_SERVICE_USER",
                    "value": "1000"
                  }
                ],
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "bin",
                    "mountPath": "/opt/bin"
                  },
                  {
                    "name": "kubectl",
                    "mountPath": "/usr/local/bin/kubectl"
                  },
                  {
                    "name": "helm",
                    "mountPath": "/usr/local/bin/helm"
                  },
                  {
                    "name": "certs",
                    "mountPath": "/etc/ssl/certs"
                  },
                  {
                    "name": "resources",
                    "mountPath": "/var/lib/gravity/resources"
                  }
                ],
                "terminationMessagePolicy": "File",
                "image": "leader.telekube.local:5000/gravitational/debian-tall:0.0.1"
              }
            ],
            "volumes": [
              {
                "name": "bin",
                "hostPath": {
                  "path": "/usr/bin",
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
                "name": "helm",
                "hostPath": {
                  "path": "/usr/bin/helm",
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
                "name": "gravity",
                "hostPath": {
                  "path": "/var/lib/gravity/local",
                  "type": ""
                }
              },
              {
                "name": "resources",
                "emptyDir": {}
              },
              {
                "name": "state-dir",
                "emptyDir": {}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "tolerations": [
              {
                "operator": "Exists",
                "effect": "NoSchedule"
              },
              {
                "operator": "Exists",
                "effect": "NoExecute"
              }
            ]
          }
        }
      },
      "status": {
        "conditions": [
          {
            "type": "Complete",
            "status": "True",
            "lastProbeTime": "2018-12-20T03:29:23Z",
            "lastTransitionTime": "2018-12-20T03:29:23Z"
          }
        ],
        "startTime": "2018-12-20T03:29:18Z",
        "completionTime": "2018-12-20T03:29:23Z",
        "succeeded": 1
      }
    },
    "name": "dns-app-install-4f2ffc",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:18.000Z",
    "createdDisplay": "2 months",
    "desired": 1,
    "statusSucceeded": 1
  }
]