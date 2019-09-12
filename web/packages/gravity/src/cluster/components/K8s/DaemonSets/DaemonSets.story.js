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
import { DaemonSets } from './DaemonSets'

storiesOf('Gravity/K8s', module)
  .add('DaemonSets', () => {
    const props = {
      namespace: 'kube-system',
      onFetch: () => $.Deferred(),
      daemonSets
    }

    return (
      <DaemonSets {...props} />
    );
  });

const daemonSets = [
  {
    "nodeMap": {
      "metadata": {
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"DaemonSet\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"labels\":{\"app\":\"gravity-site\"},\"name\":\"gravity-site\",\"namespace\":\"kube-system\"},\"spec\":{\"selector\":{\"matchLabels\":{\"app\":\"gravity-site\"}},\"template\":{\"metadata\":{\"annotations\":{\"scheduler.alpha.kubernetes.io/critical-pod\":\"\",\"seccomp.security.alpha.kubernetes.io/pod\":\"docker/default\"},\"creationTimestamp\":null,\"labels\":{\"app\":\"gravity-site\"}},\"spec\":{\"containers\":[{\"command\":[\"/usr/bin/dumb-init\",\"/bin/sh\",\"/opt/start.sh\"],\"env\":[{\"name\":\"PATH\",\"value\":\"/opt/gravity:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"},{\"name\":\"POD_IP\",\"valueFrom\":{\"fieldRef\":{\"fieldPath\":\"status.podIP\"}}},{\"name\":\"GRAVITY_CONFIG\",\"valueFrom\":{\"configMapKeyRef\":{\"key\":\"gravity.yaml\",\"name\":\"gravity-hub\"}}},{\"name\":\"GRAVITY_TELEPORT_CONFIG\",\"valueFrom\":{\"configMapKeyRef\":{\"key\":\"teleport.yaml\",\"name\":\"gravity-hub\"}}}],\"image\":\"leader.telekube.local:5000/gravity-site:5.2.3\",\"livenessProbe\":{\"httpGet\":{\"path\":\"/healthz\",\"port\":3010},\"initialDelaySeconds\":600,\"timeoutSeconds\":5},\"name\":\"gravity-site\",\"ports\":[{\"containerPort\":3009,\"name\":\"web\"},{\"containerPort\":3007,\"name\":\"agents\"},{\"containerPort\":3023,\"name\":\"sshproxy\"},{\"containerPort\":3024,\"name\":\"sshtunnel\"},{\"containerPort\":3080,\"name\":\"teleport\"},{\"containerPort\":6060,\"name\":\"profile\"}],\"readinessProbe\":{\"httpGet\":{\"path\":\"/readyz\",\"port\":3010},\"initialDelaySeconds\":10,\"timeoutSeconds\":5},\"resources\":{},\"volumeMounts\":[{\"mountPath\":\"/etc/ssl/certs\",\"name\":\"certs\"},{\"mountPath\":\"/etc/docker/certs.d\",\"name\":\"docker-certs\"},{\"mountPath\":\"/var/state\",\"name\":\"var-state\"},{\"mountPath\":\"/opt/gravity-import\",\"name\":\"import\"},{\"mountPath\":\"/opt/gravity/config\",\"name\":\"config\"},{\"mountPath\":\"/opt/gravity/hub\",\"name\":\"hub-config\"},{\"mountPath\":\"/var/lib/gravity/secrets\",\"name\":\"secrets\"},{\"mountPath\":\"/var/lib/gravity/site/secrets\",\"name\":\"secrets\"},{\"mountPath\":\"/var/lib/gravity/site\",\"name\":\"site\"},{\"mountPath\":\"/tmp\",\"name\":\"tmp\"},{\"mountPath\":\"/usr/bin/kubectl\",\"name\":\"kubectl\"},{\"mountPath\":\"/etc/kubernetes\",\"name\":\"kubeconfigs\"},{\"mountPath\":\"/usr/local/share/gravity\",\"name\":\"assets\"}]}],\"hostNetwork\":true,\"nodeSelector\":{\"gravitational.io/k8s-role\":\"master\"},\"securityContext\":{\"runAsUser\":1000},\"tolerations\":[{\"key\":\"gravitational.io/runlevel\",\"operator\":\"Equal\",\"value\":\"system\"},{\"effect\":\"NoSchedule\",\"key\":\"node-role.kubernetes.io/master\",\"operator\":\"Exists\"}],\"volumes\":[{\"hostPath\":{\"path\":\"/tmp\"},\"name\":\"tmp\"},{\"hostPath\":{\"path\":\"/etc/ssl/certs\"},\"name\":\"certs\"},{\"hostPath\":{\"path\":\"/etc/docker/certs.d\"},\"name\":\"docker-certs\"},{\"hostPath\":{\"path\":\"/var/state\"},\"name\":\"var-state\"},{\"hostPath\":{\"path\":\"/var/lib/gravity/local\"},\"name\":\"import\"},{\"configMap\":{\"name\":\"gravity-site\"},\"name\":\"config\"},{\"configMap\":{\"name\":\"gravity-hub\"},\"name\":\"hub-config\"},{\"hostPath\":{\"path\":\"/var/lib/gravity/secrets\"},\"name\":\"secrets\"},{\"hostPath\":{\"path\":\"/var/lib/gravity/site\"},\"name\":\"site\"},{\"hostPath\":{\"path\":\"/usr/bin/kubectl\"},\"name\":\"kubectl\"},{\"hostPath\":{\"path\":\"/etc/kubernetes\"},\"name\":\"kubeconfigs\"},{\"emptyDir\":{},\"name\":\"assets\"}]}},\"updateStrategy\":{}},\"status\":{\"currentNumberScheduled\":0,\"desiredNumberScheduled\":0,\"numberMisscheduled\":0,\"numberReady\":0}}\n"
        },
        "selfLink": "/apis/extensions/v1beta1/namespaces/kube-system/daemonsets/gravity-site",
        "resourceVersion": "4957049",
        "name": "gravity-site",
        "uid": "a95925be-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:30:56Z",
        "generation": 1,
        "namespace": "kube-system",
        "labels": {
          "app": "gravity-site"
        }
      },
      "spec": {
        "selector": {
          "matchLabels": {
            "app": "gravity-site"
          }
        },
        "template": {
          "metadata": {
            "creationTimestamp": null,
            "labels": {
              "app": "gravity-site"
            },
            "annotations": {
              "scheduler.alpha.kubernetes.io/critical-pod": "",
              "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
            }
          },
          "spec": {
            "nodeSelector": {
              "gravitational.io/k8s-role": "master"
            },
            "restartPolicy": "Always",
            "schedulerName": "default-scheduler",
            "hostNetwork": true,
            "terminationGracePeriodSeconds": 30,
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
                "imagePullPolicy": "IfNotPresent",
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
        "updateStrategy": {
          "type": "OnDelete"
        },
        "templateGeneration": 1,
        "revisionHistoryLimit": 10
      },
      "status": {
        "currentNumberScheduled": 1,
        "numberMisscheduled": 0,
        "desiredNumberScheduled": 1,
        "numberReady": 1,
        "observedGeneration": 1,
        "updatedNumberScheduled": 1,
        "numberAvailable": 1
      }
    },
    "name": "gravity-site",
    "namespace": "kube-system",
    "created": "2018-12-20T03:30:56.000Z",
    "createdDisplay": "2 months",
    "statusCurrentNumberScheduled": 1,
    "statusNumberMisscheduled": 0,
    "statusNumberReady": 1,
    "statusDesiredNumberScheduled": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"DaemonSet\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"name\":\"log-forwarder\",\"namespace\":\"kube-system\"},\"spec\":{\"selector\":{\"matchLabels\":{\"name\":\"log-forwarder\"}},\"template\":{\"metadata\":{\"annotations\":{\"seccomp.security.alpha.kubernetes.io/pod\":\"docker/default\"},\"creationTimestamp\":null,\"labels\":{\"name\":\"log-forwarder\"}},\"spec\":{\"containers\":[{\"image\":\"leader.telekube.local:5000/log-forwarder:5.0.2\",\"name\":\"log-forwarder\",\"resources\":{\"limits\":{\"cpu\":\"500m\",\"memory\":\"600Mi\"},\"requests\":{\"cpu\":\"100m\",\"memory\":\"200Mi\"}},\"volumeMounts\":[{\"mountPath\":\"/var/log\",\"name\":\"varlog\"},{\"mountPath\":\"/ext/docker/containers\",\"name\":\"extdockercontainers\"},{\"mountPath\":\"/var/lib/gravity/site\",\"name\":\"gravitysite\"}]}],\"securityContext\":{\"runAsUser\":0},\"tolerations\":[{\"key\":\"gravitational.io/runlevel\",\"operator\":\"Equal\",\"value\":\"system\"},{\"effect\":\"NoSchedule\",\"key\":\"node-role.kubernetes.io/master\",\"operator\":\"Exists\"}],\"volumes\":[{\"hostPath\":{\"path\":\"/var/log\"},\"name\":\"varlog\"},{\"hostPath\":{\"path\":\"/ext/docker/containers\"},\"name\":\"extdockercontainers\"},{\"hostPath\":{\"path\":\"/var/lib/gravity/site\"},\"name\":\"gravitysite\"}]}},\"updateStrategy\":{}},\"status\":{\"currentNumberScheduled\":0,\"desiredNumberScheduled\":0,\"numberMisscheduled\":0,\"numberReady\":0}}\n"
        },
        "selfLink": "/apis/extensions/v1beta1/namespaces/kube-system/daemonsets/log-forwarder",
        "resourceVersion": "577",
        "name": "log-forwarder",
        "uid": "7ab59a68-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:38Z",
        "generation": 1,
        "namespace": "kube-system",
        "labels": {
          "name": "log-forwarder"
        }
      },
      "spec": {
        "selector": {
          "matchLabels": {
            "name": "log-forwarder"
          }
        },
        "template": {
          "metadata": {
            "creationTimestamp": null,
            "labels": {
              "name": "log-forwarder"
            },
            "annotations": {
              "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
            }
          },
          "spec": {
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
              }
            ],
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
                  }
                ],
                "terminationMessagePath": "/dev/termination-log",
                "terminationMessagePolicy": "File",
                "imagePullPolicy": "IfNotPresent"
              }
            ],
            "restartPolicy": "Always",
            "terminationGracePeriodSeconds": 30,
            "dnsPolicy": "ClusterFirst",
            "securityContext": {
              "runAsUser": 0
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
        "updateStrategy": {
          "type": "OnDelete"
        },
        "templateGeneration": 1,
        "revisionHistoryLimit": 10
      },
      "status": {
        "currentNumberScheduled": 1,
        "numberMisscheduled": 0,
        "desiredNumberScheduled": 1,
        "numberReady": 1,
        "observedGeneration": 1,
        "updatedNumberScheduled": 1,
        "numberAvailable": 1
      }
    },
    "name": "log-forwarder",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:38.000Z",
    "createdDisplay": "2 months",
    "statusCurrentNumberScheduled": 1,
    "statusNumberMisscheduled": 0,
    "statusNumberReady": 1,
    "statusDesiredNumberScheduled": 1
  },
  {
    "nodeMap": {
      "metadata": {
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"extensions/v1beta1\",\"kind\":\"DaemonSet\",\"metadata\":{\"annotations\":{},\"creationTimestamp\":null,\"labels\":{\"addonmanager.kubernetes.io/mode\":\"Reconcile\",\"k8s-app\":\"kube-dns\",\"kubernetes.io/cluster-service\":\"true\"},\"name\":\"kube-dns\",\"namespace\":\"kube-system\"},\"spec\":{\"selector\":{\"matchLabels\":{\"k8s-app\":\"kube-dns\"}},\"template\":{\"metadata\":{\"annotations\":{\"scheduler.alpha.kubernetes.io/critical-pod\":\"\",\"seccomp.security.alpha.kubernetes.io/pod\":\"docker/default\"},\"creationTimestamp\":null,\"labels\":{\"k8s-app\":\"kube-dns\"}},\"spec\":{\"containers\":[{\"args\":[\"--domain=cluster.local\",\"--dns-port=10053\",\"--healthz-port=8081\",\"--config-dir=/kube-dns-config\",\"--v=2\"],\"env\":[{\"name\":\"PROMETHEUS_PORT\",\"value\":\"10055\"}],\"image\":\"leader.telekube.local:5000/k8s-dns-kube-dns-amd64:1.14.10\",\"livenessProbe\":{\"failureThreshold\":5,\"httpGet\":{\"path\":\"/healthcheck/kubedns\",\"port\":10054,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":60,\"successThreshold\":1,\"timeoutSeconds\":5},\"name\":\"kubedns\",\"ports\":[{\"containerPort\":10053,\"name\":\"dns-local\",\"protocol\":\"UDP\"},{\"containerPort\":10053,\"name\":\"dns-tcp-local\",\"protocol\":\"TCP\"},{\"containerPort\":10055,\"name\":\"metrics\",\"protocol\":\"TCP\"}],\"readinessProbe\":{\"httpGet\":{\"path\":\"/readiness\",\"port\":8081,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":30,\"timeoutSeconds\":5},\"resources\":{\"limits\":{\"memory\":\"170Mi\"},\"requests\":{\"cpu\":\"100m\",\"memory\":\"70Mi\"}},\"securityContext\":{\"runAsUser\":1000},\"volumeMounts\":[{\"mountPath\":\"/kube-dns-config\",\"name\":\"kube-dns-config\"}]},{\"args\":[\"-v=2\",\"-logtostderr\",\"-configDir=/etc/k8s/dns/dnsmasq-nanny\",\"-restartDnsmasq=true\",\"--\",\"-k\",\"--cache-size=1000\",\"--no-resolv\",\"--log-facility=-\",\"--server=/cluster.local/127.0.0.1#10053\",\"--server=/in-addr.arpa/127.0.0.1#10053\",\"--server=/ip6.arpa/127.0.0.1#10053\"],\"image\":\"leader.telekube.local:5000/k8s-dns-dnsmasq-nanny-amd64:1.14.10\",\"livenessProbe\":{\"failureThreshold\":5,\"httpGet\":{\"path\":\"/healthcheck/dnsmasq\",\"port\":10054,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":60,\"successThreshold\":1,\"timeoutSeconds\":5},\"name\":\"dnsmasq\",\"ports\":[{\"containerPort\":53,\"name\":\"dns\",\"protocol\":\"UDP\"},{\"containerPort\":53,\"name\":\"dns-tcp\",\"protocol\":\"TCP\"}],\"resources\":{\"requests\":{\"cpu\":\"150m\",\"memory\":\"20Mi\"}},\"securityContext\":{\"runAsUser\":0},\"volumeMounts\":[{\"mountPath\":\"/etc/k8s/dns/dnsmasq-nanny\",\"name\":\"kube-dns-config\"}]},{\"args\":[\"--v=2\",\"--logtostderr\",\"--probe=kubedns,127.0.0.1:10053,kubernetes.default.svc.cluster.local,5,A\",\"--probe=dnsmasq,127.0.0.1:53,kubernetes.default.svc.cluster.local,5,A\"],\"image\":\"leader.telekube.local:5000/k8s-dns-sidecar-amd64:1.14.10\",\"livenessProbe\":{\"failureThreshold\":5,\"httpGet\":{\"path\":\"/metrics\",\"port\":10054,\"scheme\":\"HTTP\"},\"initialDelaySeconds\":60,\"successThreshold\":1,\"timeoutSeconds\":5},\"name\":\"sidecar\",\"ports\":[{\"containerPort\":10054,\"name\":\"metrics\",\"protocol\":\"TCP\"}],\"resources\":{\"requests\":{\"cpu\":\"10m\",\"memory\":\"100Mi\"}},\"securityContext\":{\"runAsUser\":0}}],\"dnsPolicy\":\"Default\",\"serviceAccountName\":\"kube-dns\",\"tolerations\":[{\"key\":\"CriticalAddonsOnly\",\"operator\":\"Exists\"},{\"key\":\"gravitational.io/runlevel\",\"operator\":\"Equal\",\"value\":\"system\"},{\"effect\":\"NoSchedule\",\"key\":\"node-role.kubernetes.io/master\",\"operator\":\"Exists\"}],\"volumes\":[{\"configMap\":{\"name\":\"kube-dns\",\"optional\":true},\"name\":\"kube-dns-config\"}]}},\"updateStrategy\":{}},\"status\":{\"currentNumberScheduled\":0,\"desiredNumberScheduled\":0,\"numberMisscheduled\":0,\"numberReady\":0}}\n"
        },
        "selfLink": "/apis/extensions/v1beta1/namespaces/kube-system/daemonsets/kube-dns",
        "resourceVersion": "665",
        "name": "kube-dns",
        "uid": "71816ae1-0407-11e9-a200-42010a800006",
        "creationTimestamp": "2018-12-20T03:29:22Z",
        "generation": 1,
        "namespace": "kube-system",
        "labels": {
          "addonmanager.kubernetes.io/mode": "Reconcile",
          "k8s-app": "kube-dns",
          "kubernetes.io/cluster-service": "true"
        }
      },
      "spec": {
        "selector": {
          "matchLabels": {
            "k8s-app": "kube-dns"
          }
        },
        "template": {
          "metadata": {
            "creationTimestamp": null,
            "labels": {
              "k8s-app": "kube-dns"
            },
            "annotations": {
              "scheduler.alpha.kubernetes.io/critical-pod": "",
              "seccomp.security.alpha.kubernetes.io/pod": "docker/default"
            }
          },
          "spec": {
            "restartPolicy": "Always",
            "serviceAccountName": "kube-dns",
            "schedulerName": "default-scheduler",
            "terminationGracePeriodSeconds": 30,
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
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "kube-dns-config",
                    "mountPath": "/kube-dns-config"
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
                "imagePullPolicy": "IfNotPresent",
                "volumeMounts": [
                  {
                    "name": "kube-dns-config",
                    "mountPath": "/etc/k8s/dns/dnsmasq-nanny"
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
                "imagePullPolicy": "IfNotPresent",
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
              }
            ]
          }
        },
        "updateStrategy": {
          "type": "OnDelete"
        },
        "templateGeneration": 1,
        "revisionHistoryLimit": 10
      },
      "status": {
        "currentNumberScheduled": 1,
        "numberMisscheduled": 0,
        "desiredNumberScheduled": 1,
        "numberReady": 1,
        "observedGeneration": 1,
        "updatedNumberScheduled": 1,
        "numberAvailable": 1
      }
    },
    "name": "kube-dns",
    "namespace": "kube-system",
    "created": "2018-12-20T03:29:22.000Z",
    "createdDisplay": "2 months",
    "statusCurrentNumberScheduled": 1,
    "statusNumberMisscheduled": 0,
    "statusNumberReady": 1,
    "statusDesiredNumberScheduled": 1
  }
]