package config

const cloudConfigWorkerTemplate = `#cloud-config
coreos:
  update:
    reboot-strategy: "off"
  flannel:
    interface: $private_ipv4
    etcd_endpoints: {{.ETCDEndpoints}}
  units:
    - name: docker.service
      drop-ins:
        - name: 40-flannel.conf
          content: |
            [Unit]
            Requires=flanneld.service
            After=flanneld.service

    - name: kubelet.service
      enable: true
      command: start
      content: |
        [Unit]
        Requires=docker.service
        After=docker.service

        [Service]
        ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
        Environment="RKT_OPTS=--insecure-options=image"
        Environment=KUBELET_ACI=quay.io/colin_hom/hyperkube
        Environment=KUBELET_VERSION={{.K8sVer}}
        ExecStart=/usr/lib/coreos/kubelet-wrapper \
        --api_servers={{.SecureAPIServers}} \
        --register-node=true \
        --allow-privileged=true \
        --config=/etc/kubernetes/manifests \
        --cluster_dns={{.DNSServiceIP}} \
        --cluster_domain=cluster.local \
        --cloud-provider=aws \
        --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml \
        --tls-cert-file=/etc/kubernetes/ssl/worker.pem \
        --tls-private-key-file=/etc/kubernetes/ssl/worker-key.pem
        StartLimitInterval=0
        Restart=always
        RestartSec=10
        [Install]
        WantedBy=multi-user.target

write_files:
  - path: /etc/kubernetes/ssl/worker.pem
    encoding: gzip+base64
    content: {{.TLSConfig.WorkerCert.String}}

  - path: /etc/kubernetes/ssl/worker-key.pem
    encoding: gzip+base64
    content: {{.TLSConfig.WorkerKey.String}}

  - path: /etc/kubernetes/ssl/ca.pem
    encoding: gzip+base64
    content: {{.TLSConfig.CACert.String}}

  - path: /etc/kubernetes/manifests/kube-proxy.yaml
    content: |
        apiVersion: v1
        kind: Pod
        metadata:
          name: kube-proxy
          namespace: kube-system
        spec:
          hostNetwork: true
          containers:
          - name: kube-proxy
            image: quay.io/colin_hom/hyperkube:{{.K8sVer}}
            command:
            - /hyperkube
            - proxy
            - --master=https://{{.ControllerIP}}:443
            - --kubeconfig=/etc/kubernetes/worker-kubeconfig.yaml
            - --proxy-mode=iptables
            securityContext:
              privileged: true
            volumeMounts:
              - mountPath: /etc/ssl/certs
                name: "ssl-certs"
              - mountPath: /etc/kubernetes/worker-kubeconfig.yaml
                name: "kubeconfig"
                readOnly: true
              - mountPath: /etc/kubernetes/ssl
                name: "etc-kube-ssl"
                readOnly: true
          volumes:
            - name: "ssl-certs"
              hostPath:
                path: "/usr/share/ca-certificates"
            - name: "kubeconfig"
              hostPath:
                path: "/etc/kubernetes/worker-kubeconfig.yaml"
            - name: "etc-kube-ssl"
              hostPath:
                path: "/etc/kubernetes/ssl"

  - path: /etc/kubernetes/worker-kubeconfig.yaml
    content: |
        apiVersion: v1
        kind: Config
        clusters:
        - name: local
          cluster:
            certificate-authority: /etc/kubernetes/ssl/ca.pem
        users:
        - name: kubelet
          user:
            client-certificate: /etc/kubernetes/ssl/worker.pem
            client-key: /etc/kubernetes/ssl/worker-key.pem
        contexts:
        - context:
            cluster: local
            user: kubelet
          name: kubelet-context
        current-context: kubelet-context
`

const cloudConfigControllerTemplate = `#cloud-config
coreos:
  update:
    reboot-strategy: "off"
  flannel:
    interface: $private_ipv4
    etcd_endpoints: {{.ETCDEndpoints}}
  etcd2:
    name: controller
    advertise-client-urls: http://$private_ipv4:2379
    initial-advertise-peer-urls: http://$private_ipv4:2380
    listen-client-urls: http://0.0.0.0:2379
    listen-peer-urls: http://0.0.0.0:2380
    initial-cluster: controller=http://$private_ipv4:2380
  units:
    - name: etcd2.service
      command: start
      drop-ins:
        - name: 80-data-dir-permissions.conf
          content: |
            [Service]
            Environment=ETCD_DATA_DIR=/var/lib/etcd2
            PermissionsStartOnly=true
            ExecStartPre=/usr/bin/chown -R etcd:etcd /var/lib/etcd2
    - name: docker.service
      drop-ins:
        - name: 40-flannel.conf
          content: |
            [Unit]
            Requires=flanneld.service
            After=flanneld.service

    - name: flanneld.service
      drop-ins:
        - name: 10-etcd.conf
          content: |
            [Service]
            ExecStartPre=/usr/bin/curl --silent -X PUT -d \
            "value={\"Network\" : \"{{.PodCIDR}}\"}" \
            http://localhost:2379/v2/keys/coreos.com/network/config?prevExist=false
    - name: kubelet.service
      command: start
      enable: true
      content: |
        [Service]
        ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
        Environment="RKT_OPTS=--insecure-options=image"
        Environment=KUBELET_VERSION={{.K8sVer}}
        Environment=KUBELET_ACI=quay.io/colin_hom/hyperkube
        ExecStart=/usr/lib/coreos/kubelet-wrapper \
        --api_servers=http://localhost:8080 \
        --register-node=false \
        --allow-privileged=true \
        --config=/etc/kubernetes/manifests \
        --cluster_dns={{.DNSServiceIP}} \
        --cluster_domain=cluster.local
        Restart=always
        RestartSec=10
        StartLimitInterval=0

        [Install]
        WantedBy=multi-user.target

    - name: install-kube-system.service
      command: start
      content: |
        [Unit]
        Requires=kubelet.service docker.service
        After=kubelet.service docker.service

        [Service]
        Type=simple
        StartLimitInterval=0
        Restart=on-failure
        ExecStartPre=/usr/bin/curl http://127.0.0.1:8080/version
        ExecStart=/opt/bin/install-kube-system

    - name: var-lib-etcd2.mount
      enable: true
      content: |
        [Unit]
        Description=etcd2 data directory ebs volume mount
        Before=etcd2.service

        [Mount]
        What=/dev/xvdf
        Where=/var/lib/etcd2
        Type=ext4

        [Install]
        RequiredBy=etcd2.service

    - name: format-etcd2-volume.service
      enable: true
      content: |
        [Unit]
        Description=etcd2 ebs volume formatting
        Before=var-lib-etcd2.mount
        After=dev-xvdf.device
        Requires=dev-xvdf.device

        [Service]
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=/opt/bin/format-etcd2-volume

        [Install]
        RequiredBy=var-lib-etcd2.mount

write_files:
  - path: /opt/bin/format-etcd2-volume
    permissions: 0700
    owner: root:root
    content: |
      #!/bin/bash -e
      if [[ "$(wipefs -n -p /dev/xvdf | grep ext4)" == "" ]];then
        mkfs.ext4 /dev/xvdf
      else
        echo "etcd volume is already formatted"
      fi

  - path: /opt/bin/install-kube-system
    permissions: 0700
    owner: root:root
    content: |
      #!/bin/bash -e
      ## TODO: Remove kube-system namespace and wait till it's actually terminated. Need this to support updates to kube-system
      /usr/bin/curl -XPOST -d @"/srv/kubernetes/manifests/kube-system.json" "http://127.0.0.1:8080/api/v1/namespaces"

      for manifest in {kube-dns,heapster,influxdb}-rc.json;do
          /usr/bin/curl -XPOST \
          -d @"/srv/kubernetes/manifests/$manifest" \
          "http://127.0.0.1:8080/api/v1/namespaces/kube-system/replicationcontrollers"
      done
      for manifest in {kube-dns,heapster,influxdb}-svc.json;do
          /usr/bin/curl -XPOST \
          -d @"/srv/kubernetes/manifests/$manifest" \
          "http://127.0.0.1:8080/api/v1/namespaces/kube-system/services"
      done

  - path: /etc/kubernetes/manifests/kube-proxy.yaml
    content: |
        apiVersion: v1
        kind: Pod
        metadata:
          name: kube-proxy
          namespace: kube-system
        spec:
          hostNetwork: true
          containers:
          - name: kube-proxy
            image: quay.io/colin_hom/hyperkube:{{.K8sVer}}
            command:
            - /hyperkube
            - proxy
            - --master=http://127.0.0.1:8080
            - --proxy-mode=iptables
            securityContext:
              privileged: true
            volumeMounts:
            - mountPath: /etc/ssl/certs
              name: ssl-certs-host
              readOnly: true
          volumes:
          - hostPath:
              path: /usr/share/ca-certificates
            name: ssl-certs-host

  - path: /etc/kubernetes/manifests/kube-apiserver.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-apiserver
        namespace: kube-system
      spec:
        hostNetwork: true
        containers:
        - name: kube-apiserver
          image: quay.io/colin_hom/hyperkube:{{.K8sVer}}
          command:
          - /hyperkube
          - apiserver
          - --bind-address=0.0.0.0
          - --etcd-servers=http://localhost:2379
          - --allow-privileged=true
          - --service-cluster-ip-range={{.ServiceCIDR}}
          - --secure-port=443
          - --advertise-address=$private_ipv4
          - --admission-control=NamespaceLifecycle,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota
          - --tls-cert-file=/etc/kubernetes/ssl/apiserver.pem
          - --tls-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
          - --client-ca-file=/etc/kubernetes/ssl/ca.pem
          - --service-account-key-file=/etc/kubernetes/ssl/apiserver-key.pem
          - --runtime-config=extensions/v1beta1/deployments=true,extensions/v1beta1/daemonsets=true
          - --cloud-provider=aws
          ports:
          - containerPort: 443
            hostPort: 443
            name: https
          - containerPort: 8080
            hostPort: 8080
            name: local
          volumeMounts:
          - mountPath: /etc/kubernetes/ssl
            name: ssl-certs-kubernetes
            readOnly: true
          - mountPath: /etc/ssl/certs
            name: ssl-certs-host
            readOnly: true
        volumes:
        - hostPath:
            path: /etc/kubernetes/ssl
          name: ssl-certs-kubernetes
        - hostPath:
            path: /usr/share/ca-certificates
          name: ssl-certs-host

  - path: /etc/kubernetes/manifests/kube-podmaster.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-podmaster
        namespace: kube-system
      spec:
        hostNetwork: true
        containers:
        - name: scheduler-elector
          image: gcr.io/google_containers/podmaster:1.1
          command:
          - /podmaster
          - --etcd-servers=http://localhost:2379
          - --key=scheduler
          - --whoami=$private_ipv4
          - --source-file=/src/manifests/kube-scheduler.yaml
          - --dest-file=/dst/manifests/kube-scheduler.yaml
          volumeMounts:
          - mountPath: /src/manifests
            name: manifest-src
            readOnly: true
          - mountPath: /dst/manifests
            name: manifest-dst
        - name: controller-manager-elector
          image: gcr.io/google_containers/podmaster:1.1
          command:
          - /podmaster
          - --etcd-servers=http://localhost:2379
          - --key=controller
          - --whoami=$private_ipv4
          - --source-file=/src/manifests/kube-controller-manager.yaml
          - --dest-file=/dst/manifests/kube-controller-manager.yaml
          terminationMessagePath: /dev/termination-log
          volumeMounts:
          - mountPath: /src/manifests
            name: manifest-src
            readOnly: true
          - mountPath: /dst/manifests
            name: manifest-dst
        volumes:
        - hostPath:
            path: /srv/kubernetes/manifests
          name: manifest-src
        - hostPath:
            path: /etc/kubernetes/manifests
          name: manifest-dst

  - path: /etc/kubernetes/manifests/kube-controller-manager.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-controller-manager
        namespace: kube-system
      spec:
        containers:
        - name: kube-controller-manager
          image: quay.io/colin_hom/hyperkube:{{.K8sVer}}
          command:
          - /hyperkube
          - controller-manager
          - --master=http://127.0.0.1:8080
          - --service-account-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
          - --root-ca-file=/etc/kubernetes/ssl/ca.pem
          - --cloud-provider=aws
          livenessProbe:
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10252
            initialDelaySeconds: 15
            timeoutSeconds: 1
          volumeMounts:
          - mountPath: /etc/kubernetes/ssl
            name: ssl-certs-kubernetes
            readOnly: true
          - mountPath: /etc/ssl/certs
            name: ssl-certs-host
            readOnly: true
        hostNetwork: true
        volumes:
        - hostPath:
            path: /etc/kubernetes/ssl
          name: ssl-certs-kubernetes
        - hostPath:
            path: /usr/share/ca-certificates
          name: ssl-certs-host

  - path: /etc/kubernetes/manifests/kube-scheduler.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-scheduler
        namespace: kube-system
      spec:
        hostNetwork: true
        containers:
        - name: kube-scheduler
          image: quay.io/colin_hom/hyperkube:{{.K8sVer}}
          command:
          - /hyperkube
          - scheduler
          - --master=http://127.0.0.1:8080
          livenessProbe:
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10251
            initialDelaySeconds: 15
            timeoutSeconds: 1

  - path: /srv/kubernetes/manifests/kube-system.json
    content: |
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "metadata": {
            "name": "kube-system"
          }
        }

  - path: /srv/kubernetes/manifests/kube-dns-rc.json
    content: |
        {
          "apiVersion": "v1",
          "kind": "ReplicationController",
          "metadata": {
            "labels": {
              "k8s-app": "kube-dns",
              "kubernetes.io/cluster-service": "true",
              "version": "v9"
            },
            "name": "kube-dns-v9",
            "namespace": "kube-system"
          },
          "spec": {
            "replicas": 1,
            "selector": {
              "k8s-app": "kube-dns",
              "version": "v9"
            },
            "template": {
              "metadata": {
                "labels": {
                  "k8s-app": "kube-dns",
                  "kubernetes.io/cluster-service": "true",
                  "version": "v9"
                }
              },
              "spec": {
                "containers": [
                  {
                    "command": [
                      "/usr/local/bin/etcd",
                      "-data-dir",
                      "/var/etcd/data",
                      "-listen-client-urls",
                      "http://127.0.0.1:2379,http://127.0.0.1:4001",
                      "-advertise-client-urls",
                      "http://127.0.0.1:2379,http://127.0.0.1:4001",
                      "-initial-cluster-token",
                      "skydns-etcd"
                    ],
                    "image": "gcr.io/google_containers/etcd:2.0.9",
                    "name": "etcd",
                    "resources": {
                      "limits": {
                        "cpu": "100m",
                        "memory": "50Mi"
                      }
                    },
                    "volumeMounts": [
                      {
                        "mountPath": "/var/etcd/data",
                        "name": "etcd-storage"
                      }
                    ]
                  },
                  {
                    "args": [
                      "-domain=cluster.local"
                    ],
                    "image": "gcr.io/google_containers/kube2sky:1.11",
                    "name": "kube2sky",
                    "resources": {
                      "limits": {
                        "cpu": "100m",
                        "memory": "50Mi"
                      }
                    }
                  },
                  {
                    "args": [
                      "-machines=http://localhost:4001",
                      "-addr=0.0.0.0:53",
                      "-domain=cluster.local."
                    ],
                    "image": "gcr.io/google_containers/skydns:2015-03-11-001",
                    "livenessProbe": {
                      "httpGet": {
                        "path": "/healthz",
                        "port": 8080,
                        "scheme": "HTTP"
                      },
                      "initialDelaySeconds": 30,
                      "timeoutSeconds": 5
                    },
                    "name": "skydns",
                    "ports": [
                      {
                        "containerPort": 53,
                        "name": "dns",
                        "protocol": "UDP"
                      },
                      {
                        "containerPort": 53,
                        "name": "dns-tcp",
                        "protocol": "TCP"
                      }
                    ],
                    "readinessProbe": {
                      "httpGet": {
                        "path": "/healthz",
                        "port": 8080,
                        "scheme": "HTTP"
                      },
                      "initialDelaySeconds": 1,
                      "timeoutSeconds": 5
                    },
                    "resources": {
                      "limits": {
                        "cpu": "100m",
                        "memory": "50Mi"
                      }
                    }
                  },
                  {
                    "args": [
                      "-cmd=nslookup kubernetes.default.svc.cluster.local localhost",
                      "-port=8080"
                    ],
                    "image": "gcr.io/google_containers/exechealthz:1.0",
                    "name": "healthz",
                    "ports": [
                      {
                        "containerPort": 8080,
                        "protocol": "TCP"
                      }
                    ],
                    "resources": {
                      "limits": {
                        "cpu": "10m",
                        "memory": "20Mi"
                      }
                    }
                  }
                ],
                "dnsPolicy": "Default",
                "volumes": [
                  {
                    "emptyDir": {},
                    "name": "etcd-storage"
                  }
                ]
              }
            }
          }
        }

  - path: /srv/kubernetes/manifests/kube-dns-svc.json
    content: |
        {
          "apiVersion": "v1",
          "kind": "Service",
          "metadata": {
            "name": "kube-dns",
            "namespace": "kube-system",
            "labels": {
              "k8s-app": "kube-dns",
              "kubernetes.io/name": "KubeDNS",
              "kubernetes.io/cluster-service": "true"
            }
          },
          "spec": {
            "clusterIP": "{{.DNSServiceIP}}",
            "ports": [
              {
                "protocol": "UDP",
                "name": "dns",
                "port": 53
              },
              {
                "protocol": "TCP",
                "name": "dns-tcp",
                "port": 53
              }
            ],
            "selector": {
              "k8s-app": "kube-dns"
            }
          }
        }

  - path: /srv/kubernetes/manifests/heapster-rc.json
    content: |
        {
          "apiVersion": "v1",
          "kind": "ReplicationController",
          "metadata": {
            "name": "heapster-v10",
            "namespace": "kube-system",
            "labels": {
              "k8s-app": "heapster",
              "version": "v10",
              "kubernetes.io/cluster-service": "true"
            }
          },
          "spec": {
            "replicas": 1,
            "selector": {
              "k8s-app": "heapster",
              "version": "v10"
            },
            "template": {
              "metadata": {
                "labels": {
                  "k8s-app": "heapster",
                  "version": "v10",
                  "kubernetes.io/cluster-service": "true"
                }
              },
              "spec": {
                "containers": [
                  {
                    "image": "gcr.io/google_containers/heapster:v0.18.2",
                    "name": "heapster",
                    "resources": {
                      "limits": {
                        "cpu": "100m",
                        "memory": "224Mi"
                      },
                      "requests": {
                        "cpu": "100m",
                        "memory": "224Mi"
                      }
                    },
                    "command": [
                      "/heapster",
                      "--source=kubernetes:''",
                      "--sink=influxdb:http://monitoring-influxdb:8086",
                      "--stats_resolution=30s",
                      "--sink_frequency=1m"
                    ]
                  }
                ]
              }
            }
          }
        }

  - path: /srv/kubernetes/manifests/heapster-svc.json
    content: |
        {
          "kind": "Service",
          "apiVersion": "v1",
          "metadata": {
            "name": "heapster",
            "namespace": "kube-system",
            "labels": {
              "kubernetes.io/cluster-service": "true",
              "kubernetes.io/name": "Heapster"
            }
          },
          "spec": {
            "ports": [
              {
                "port": 80,
                "targetPort": 8082
              }
            ],
            "selector": {
              "k8s-app": "heapster"
            }
          }
        }

  - path: /srv/kubernetes/manifests/influxdb-rc.json
    content: |
        {
          "apiVersion": "v1",
          "kind": "ReplicationController",
          "metadata": {
            "name": "monitoring-influxdb-v2",
            "namespace": "kube-system",
            "labels": {
              "k8s-app": "influxdb",
              "version": "v2",
              "kubernetes.io/cluster-service": "true"
            }
          },
          "spec": {
            "replicas": 1,
            "selector": {
              "k8s-app": "influxdb",
              "version": "v2"
            },
            "template": {
              "metadata": {
                "labels": {
                  "k8s-app": "influxdb",
                  "version": "v2",
                  "kubernetes.io/cluster-service": "true"
                }
              },
              "spec": {
                "containers": [
                  {
                    "image": "gcr.io/google_containers/heapster_influxdb:v0.4",
                    "name": "influxdb",
                    "resources": {
                      "limits": {
                        "cpu": "100m",
                        "memory": "200Mi"
                      },
                      "requests": {
                        "cpu": "100m",
                        "memory": "200Mi"
                      }
                    },
                    "ports": [
                      {
                        "containerPort": 8083,
                        "hostPort": 8083
                      },
                      {
                        "containerPort": 8086,
                        "hostPort": 8086
                      }
                    ],
                    "volumeMounts": [
                      {
                        "name": "influxdb-persistent-storage",
                        "mountPath": "/data"
                      }
                    ]
                  }
                ],
                "volumes": [
                  {
                    "name": "influxdb-persistent-storage",
                    "emptyDir": {}
                  }
                ]
              }
            }
          }
        }

  - path: /srv/kubernetes/manifests/influxdb-svc.json
    content: |
        {
          "apiVersion": "v1",
          "kind": "Service",
          "metadata": {
            "name": "monitoring-influxdb",
            "namespace": "kube-system",
            "labels": {
              "kubernetes.io/cluster-service": "true",
              "kubernetes.io/name": "InfluxDB"
            }
          },
          "spec": {
            "ports": [
              {
                "name": "http",
                "port": 8083,
                "targetPort": 8083
              },
              {
                "name": "api",
                "port": 8086,
                "targetPort": 8086
              }
            ],
            "selector": {
              "k8s-app": "influxdb"
            }
          }
        }

  - path: /etc/kubernetes/ssl/ca.pem
    encoding: gzip+base64
    content: {{.TLSConfig.CACert.String}}

  - path: /etc/kubernetes/ssl/apiserver.pem
    encoding: gzip+base64
    content: {{.TLSConfig.APIServerCert.String}}

  - path: /etc/kubernetes/ssl/apiserver-key.pem
    encoding: gzip+base64
    content: {{.TLSConfig.APIServerKey.String}}
`
