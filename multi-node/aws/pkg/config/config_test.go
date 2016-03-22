package config

import (
	"net"
	"testing"
)

const MinimalConfigYaml = `externalDNSName: test-external-dns-name
keyName: test-key-name
region: us-west-1
availabilityZone: us-west-1c
clusterName: test-cluster-name
`

var goodNetworkingConfigs []string = []string{
	``, //Tests validity of default network config values
	`
vpcCIDR: 10.4.3.0/24
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.100.101
`, `
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.6.0.0/16
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`,
}

var incorrectNetworkingConfigs []string = []string{
	`
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.5.3.0/24 #instanceCIDR not in vpcCIDR
controllerIP: 10.5.3.5
podCIDR: 10.6.0.0/16
serviceCIDR: 10.5.0.0/16
dnsServiceIP: 10.5.100.101
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.0.1 #dnsServiceIP conflicts with kubernetesServiceIP
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.4.0.0/16 #vpcCIDR overlaps with podCIDR
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.5.100.101

`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
dnsServiceIP: 172.6.100.101 #dnsServiceIP not in service CIDR
`,
}

func TestNetworkValidation(t *testing.T) {

	for _, networkConfig := range goodNetworkingConfigs {
		configBody := MinimalConfigYaml + networkConfig
		if _, err := clusterFromBytes([]byte(configBody)); err != nil {
			t.Errorf("Correct config tested invalid: %s\n%s", err, networkConfig)
		}
	}

	for _, networkConfig := range incorrectNetworkingConfigs {
		configBody := MinimalConfigYaml + networkConfig
		if _, err := clusterFromBytes([]byte(configBody)); err == nil {
			t.Errorf("Incorrect config tested valid, expected error:\n%s", networkConfig)
		}
	}

}

func TestKubernetesServiceIPInference(t *testing.T) {

	// We sill assert that after parsing the network configuration,
	// KubernetesServiceIP is the correct pre-determined value
	testConfigs := []struct {
		NetworkConfig       string
		KubernetesServiceIP string
	}{
		{
			NetworkConfig: `
serviceCIDR: 172.5.10.10/22
dnsServiceIP: 172.5.10.10
        `,
			KubernetesServiceIP: "172.5.8.1",
		},
		{
			NetworkConfig: `
serviceCIDR: 10.5.70.10/18
dnsServiceIP: 10.5.64.10
        `,
			KubernetesServiceIP: "10.5.64.1",
		},
		{
			NetworkConfig: `
serviceCIDR: 172.4.155.98/27
dnsServiceIP: 172.4.155.100
        `,
			KubernetesServiceIP: "172.4.155.97",
		},
		{
			NetworkConfig: `
serviceCIDR: 10.6.142.100/28
dnsServiceIP: 10.6.142.100
        `,
			KubernetesServiceIP: "10.6.142.97",
		},
	}

	for _, testConfig := range testConfigs {

		configBody := MinimalConfigYaml + testConfig.NetworkConfig
		cluster, err := clusterFromBytes([]byte(configBody))
		if err != nil {
			t.Errorf("Unexpected error parsing config: %v\n %s", err, configBody)
			continue
		}

		config, err := cluster.Config()
		if err != nil {
			t.Errorf("Error getting config from cluster: %v\n%v", err, cluster)
			continue
		}

		if config.KubernetesServiceIP != testConfig.KubernetesServiceIP {
			t.Errorf("KubernetesServiceIP mismatch: got %s, expected %s", config.KubernetesServiceIP, testConfig.KubernetesServiceIP)
		}
	}

}

func TestBroadcastAddress(t *testing.T) {
	cidrs := []struct {
		CIDR      string
		Broadcast string
	}{
		{
			CIDR:      "10.10.10.160/28",
			Broadcast: "10.10.10.175",
		},
		{
			CIDR:      "172.4.18.206/30",
			Broadcast: "172.4.18.207",
		},
		{
			CIDR:      "172.6.30.0/20",
			Broadcast: "172.6.31.255",
		},
	}

	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr.CIDR)
		if err != nil {
			t.Errorf("Error parsing cidr %s : %v", cidr.CIDR, err)
			continue
		}

		expectedBroadcast := net.ParseIP(cidr.Broadcast)

		broadcast, err := broadcastAddress(network)
		if err != nil {
			t.Errorf("Error getting broadcast address: %v", err)
			continue
		}

		if !broadcast.Equal(expectedBroadcast) {
			t.Errorf("Got broadcast ip %s, expected %s: cidr = %s", broadcast, expectedBroadcast, network)
		}

	}

}
