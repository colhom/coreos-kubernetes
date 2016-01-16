package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/coreos/coreos-kubernetes/multi-node/aws/pkg/cluster"
)

var (
	cmdUp = &cobra.Command{
		Use:   "up",
		Short: "Create a new Kubernetes cluster",
		Long:  ``,
		Run:   runCmdUp,
	}
)

func init() {
	cmdRoot.AddCommand(cmdUp)
}

func runCmdUp(cmd *cobra.Command, args []string) {
	cfgPath := filepath.Join(rootOpts.AssetDir, "cluster.yaml")

	cfg := cluster.NewDefaultConfig(VERSION)
	err := cluster.DecodeConfigFromFile(cfg, cfgPath)
	if err != nil {
		stderr("Unable to load cluster config: %v", err)
		os.Exit(1)
	}

	c := cluster.New(cfg, newAWSConfig(cfg))

	if err := c.Create(rootOpts.AssetDir); err != nil {
		stderr("Failed creating cluster: %v", err)
		os.Exit(1)
	}

	fmt.Println("Successfully created cluster")
	fmt.Println("")

	info, err := c.Info()
	if err != nil {
		stderr("Failed fetching cluster info: %v", err)
		os.Exit(1)
	}

	fmt.Printf(info.String())
}

func getCloudFormation(url string) (string, error) {
	r, err := http.Get(url)

	if err != nil {
		return "", fmt.Errorf("Failed to get template: %v", err)
	}

	if r.StatusCode != 200 {
		return "", fmt.Errorf("Failed to get template: invalid status code: %d", r.StatusCode)
	}

	tmpl, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to get template: %v", err)
	}
	r.Body.Close()

	return string(tmpl), nil
}
