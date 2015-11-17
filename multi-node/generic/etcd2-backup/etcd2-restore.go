package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"time"
)

var ETCDCTL_PATH, ETCD_PATH, ETCD_RESTORE_DIR, ETCD_NAME, ETCD_PEER_URLS string

func main() {

	flag.StringVar(&ETCDCTL_PATH, "etcdctl-path", "/usr/bin/etcdctl", "absolute path to etcdctl executable")
	flag.StringVar(&ETCD_PATH, "etcd-path", "/usr/bin/etcd2", "absolute path to etcd2 executable")
	flag.StringVar(&ETCD_RESTORE_DIR, "etcd-restore-dir", "/var/lib/etcd2-restore", "absolute path to etcd2 restore dir")
	flag.StringVar(&ETCD_NAME, "etcd-name", "default", "name of etcd2 node")
	flag.StringVar(&ETCD_PEER_URLS, "etcd-peer-urls", "", "advertise peer urls")

	flag.Parse()

	if ETCD_PEER_URLS == "" {
		panic("must set -etcd-peer-urls")
	}

	if finfo, err := os.Stat(ETCD_RESTORE_DIR); err != nil {
		panic(err)
	} else {
		if !finfo.IsDir() {
			panic(fmt.Errorf("%s is not a directory", ETCD_RESTORE_DIR))
		}
	}

	if !path.IsAbs(ETCDCTL_PATH) {
		panic(fmt.Sprintf("etcdctl-path %s is not absolute", ETCDCTL_PATH))
	}

	if !path.IsAbs(ETCD_PATH) {
		panic(fmt.Sprintf("etcd-path %s is not absolute", ETCD_PATH))
	}

	if err := restoreEtcd(); err != nil {
		panic(err)
	}
}

func restoreEtcd() error {

	etcdCmd := exec.Command(ETCD_PATH, "--force-new-cluster", "--data-dir", ETCD_RESTORE_DIR)

	etcdCmd.Stdout = os.Stdout
	etcdCmd.Stderr = os.Stderr

	if err := etcdCmd.Start(); err != nil {
		return fmt.Errorf("Could not start etcd2: %s", err)
	}
	defer etcdCmd.Wait()
	defer etcdCmd.Process.Kill()

	return runCommands(10, 2*time.Second)

}

var clusterHealthRegex = regexp.MustCompile(".*cluster is healthy.*")
var lineSplit = regexp.MustCompile("\n+")
var colonSplit = regexp.MustCompile("\\:")

func runCommands(maxRetry int, interval time.Duration) error {
	var retryCnt int
	for retryCnt = 1; retryCnt <= maxRetry; retryCnt++ {
		out, err := exec.Command(ETCDCTL_PATH, "cluster-health").CombinedOutput()
		if err == nil && clusterHealthRegex.Match(out) {
			break
		}
		fmt.Printf("Error: %s: %s\n", err, string(out))
		time.Sleep(interval)
	}

	if retryCnt > maxRetry {
		return fmt.Errorf("Timed out waiting for healthy cluster\n")
	}

	memberID := ""
	if out, err := exec.Command(ETCDCTL_PATH, "member", "list").CombinedOutput(); err != nil {
		return fmt.Errorf("Error calling member list: %s", err)
	} else {
		members := lineSplit.Split(string(out), 2)
		if len(members) < 1 {
			return fmt.Errorf("Could not find a cluster member from: \"%s\"", members)
		}
		parts := colonSplit.Split(members[0], 2)
		if len(parts) < 2 {
			return fmt.Errorf("Could not parse member id from: \"%s\"", members[0])
		}
		memberID = parts[0]
	}

	out, err := exec.Command(ETCDCTL_PATH, "member", "update", memberID, ETCD_PEER_URLS).CombinedOutput()

	fmt.Printf("member update result: %s\n", string(out))
	if err != nil {
		return err
	}

	return nil
}
