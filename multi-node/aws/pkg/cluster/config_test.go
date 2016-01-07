package cluster

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

const (
	testVer         = "1"
	externalDNSName = "test-external-dns-name"
	keyName         = "test-key-name"
	region          = "test-region"
	clusterName     = "test-cluster-name"
)
const minimalYamlTemplate = `externalDNSName: %s
keyName: %s
region: %s
clusterName: %s
`

var minimalConfigYaml string = fmt.Sprintf(
	minimalYamlTemplate,
	externalDNSName,
	keyName,
	region,
	clusterName,
)

func correctMinimalConfig() *Config {
	config := NewDefaultConfig(testVer)
	config.ExternalDNSName = externalDNSName
	config.KeyName = keyName
	config.Region = region
	config.ClusterName = clusterName
	return config
}

func parseConfig(out *Config, loc string) error {
	d, err := ioutil.ReadFile(loc)
	if err != nil {
		return fmt.Errorf("failed reading config file: %v", err)
	}

	if err = yaml.Unmarshal(d, &out); err != nil {
		return fmt.Errorf("failed decoding config file: %v", err)
	}

	return nil
}

func TestCorrectMinimalConfig(t *testing.T) {
	cfg := correctMinimalConfig()
	if err := cfg.Valid(); err != nil {
		t.Fatalf("Could not validate: %s\n : %+v\n", err, cfg)
	}
}

func writeTestFile(contents string) (string, error) {
	tpath := filepath.Join(os.TempDir(), fmt.Sprintf("test-file-%d", rand.Int()))
	f, err := os.Create(tpath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, bytes.NewBuffer([]byte(contents))); err != nil {
		return "", err
	}

	return tpath, nil
}

func TestConfigParserDefaults(t *testing.T) {

	configPath, err := writeTestFile(minimalConfigYaml)
	if err != nil {
		t.Fatal(err)
	}

	parsedConfig := NewDefaultConfig(testVer)
	if err := parseConfig(parsedConfig, configPath); err != nil {
		t.Fatal(err)
	}

	if err := parsedConfig.Valid(); err != nil {
		t.Fatalf("could not validate: %s \n%+v\n", err, parsedConfig)
	}

	correctConfig := correctMinimalConfig()
	if *parsedConfig != *correctConfig {
		t.Fatalf("parsed config does not match:\n parsed: %+v \n correct %+v\n",
			parsedConfig,
			correctConfig,
		)
	}
}

var goodNetworkingConfigs []string = []string{
	`
vpcCIDR: 10.4.3.0/24
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
kubernetesServiceIP: 172.5.100.100
dnsServiceIP: 172.5.100.101
`, `
vpcCIDR: 10.4.0.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.6.0.0/16
serviceCIDR: 10.5.0.0/16
kubernetesServiceIP: 10.5.100.100
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
kubernetesServiceIP: 10.5.100.100
dnsServiceIP: 10.5.100.101
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
kubernetesServiceIP: 172.10.100.100 #kubernetesServiceIP not in service CIDR
dnsServiceIP: 172.5.100.101
`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 10.4.0.0/16 #vpcCIDR overlaps with podCIDR
serviceCIDR: 172.5.0.0/16
kubernetesServiceIP: 172.5.100.100
dnsServiceIP: 172.5.100.101

`, `
vpcCIDR: 10.4.3.0/16
instanceCIDR: 10.4.3.0/24
controllerIP: 10.4.3.5
podCIDR: 172.4.0.0/16
serviceCIDR: 172.5.0.0/16
kubernetesServiceIP: 172.5.100.100
dnsServiceIP: 172.6.100.101 #dnsServiceIP not in service CIDR
`,
}

func TestNetworkValidation(t *testing.T) {
	testConfig := func(networkConfig string) error {
		configPath, err := writeTestFile(minimalConfigYaml + networkConfig)
		if err != nil {
			t.Fatalf("error opening test file: %s", err)
		}

		config := NewDefaultConfig(testVer)
		if err := parseConfig(config, configPath); err != nil {
			t.Fatalf("error decoding config file: %s", err)
		}

		return config.Valid()
	}

	for _, networkConfig := range goodNetworkingConfigs {
		if err := testConfig(networkConfig); err != nil {
			t.Fatalf("Correct config tested invalid: %s\n%s", err, networkConfig)
		}
	}

	for _, networkConfig := range incorrectNetworkingConfigs {
		if err := testConfig(networkConfig); err == nil {
			t.Fatalf("Incorrect config tested valid, expected error:\n%s", networkConfig)
		}
	}

}
