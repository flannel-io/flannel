// Copyright 2015 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ipsec

import (
	"fmt"
	"github.com/bronze1man/goStrongswanVici"
	"github.com/coreos/flannel/subnet"
	log "github.com/golang/glog"
	"golang.org/x/net/context"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	charonExecutablePath = "/opt/strongswan/libexec/ipsec/charon"
)

var defaultViciUri = Uri{"unix", "/var/run/charon.vici"}

type Uri struct {
	network, address string
}

type CharonIKEDaemon struct {
	viciUri     Uri
	espProposal string
}

func NewCharonIKEDaemon(ctx context.Context, wg sync.WaitGroup, charonViciUri string,
	espProposal string) (*CharonIKEDaemon, error) {

	charon := &CharonIKEDaemon{viciUri: defaultViciUri, espProposal: espProposal}
	log.Infof("Using ESP proposal: %s", espProposal)
	if charonViciUri == "" {
		cmd, err := charon.runBundled()

		if err != nil {
			log.Errorf("Error starting bundled charon daemon: %v", err)
			return nil, err
		} else {
			log.Info("Bundled charon daemon started")
		}
		wg.Add(1)
		go func() {
			select {
			case <-ctx.Done():
				cmd.Process.Signal(syscall.SIGTERM)
				log.Infof("Stopped bundled charon daemon")
				wg.Done()
				return
			}
		}()
	} else {
		log.Infof("Using external charon at: %s", charonViciUri)
		addr := strings.Split(charonViciUri, "://")
		charon.viciUri = Uri{addr[0], addr[1]}
	}

	return charon, nil
}

func (charon *CharonIKEDaemon) getClient() (
	client *goStrongswanVici.ClientConn, err error) {
	conn, err := net.Dial(charon.viciUri.network, charon.viciUri.address)
	if err != nil {
		return
	}
	return goStrongswanVici.NewClientConn(conn), nil
}

func (charon *CharonIKEDaemon) runBundled() (cmd *exec.Cmd, err error) {
	path, err := exec.LookPath(charonExecutablePath)
	if err != nil {
		return nil, err
	}
	cmd = &exec.Cmd{
		Path: path,
		SysProcAttr: &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGTERM,
		},
	}
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	return
}

func (charon *CharonIKEDaemon) LoadSharedKey(remotePublicIP, password string) error {
	var err error
	var client *goStrongswanVici.ClientConn

	for {
		client, err = charon.getClient()
		if err == nil {
			break
		} else {
			log.Error("ClientConnection failed: ", err)
			log.Info("Retrying in 1 second ...")
			time.Sleep(1 * time.Second)
		}
	}

	defer client.Close()

	sharedKey := &goStrongswanVici.Key{
		Typ:    "IKE",
		Data:   password,
		Owners: []string{remotePublicIP},
	}

	err = client.LoadShared(sharedKey)
	if err != nil {
		return err
	}

	log.Infof("Loaded shared key for: %v", remotePublicIP)
	return nil
}

func (charon *CharonIKEDaemon) LoadConnection(localLease, remoteLease *subnet.Lease,
	reqID, encap string) error {
	var err error
	var client *goStrongswanVici.ClientConn

	for {
		client, err = charon.getClient()
		if err == nil {
			break
		} else {
			log.Info("ClientConnection failed: ", err)
			log.Infof("Retying in 1 second ...")
			time.Sleep(1 * time.Second)
		}
	}
	defer client.Close()

	childConfMap := make(map[string]goStrongswanVici.ChildSAConf)
	childSAConf := goStrongswanVici.ChildSAConf{
		Local_ts:     []string{localLease.Subnet.String()},
		Remote_ts:    []string{remoteLease.Subnet.String()},
		ESPProposals: []string{charon.espProposal},
		StartAction:  "start",
		CloseAction:  "trap",
		Mode:         "tunnel",
		ReqID:        reqID,
		//		RekeyTime:     rekeyTime,
		InstallPolicy: "no",
	}

	childSAConfName := formatChildSAConfName(localLease, remoteLease)

	childConfMap[childSAConfName] = childSAConf

	localAuthConf := goStrongswanVici.AuthConf{
		AuthMethod: "psk",
	}
	remoteAuthConf := goStrongswanVici.AuthConf{
		AuthMethod: "psk",
	}

	ikeConf := goStrongswanVici.IKEConf{
		LocalAddrs:  []string{localLease.Attrs.PublicIP.String()},
		RemoteAddrs: []string{remoteLease.Attrs.PublicIP.String()},
		Proposals:   []string{"aes256-sha256-modp4096"},
		Version:     "2",
		KeyingTries: "0", //continues to retry
		LocalAuth:   localAuthConf,
		RemoteAuth:  remoteAuthConf,
		Children:    childConfMap,
		Encap:       encap,
	}
	ikeConfMap := make(map[string]goStrongswanVici.IKEConf)

	connectionName := formatConnectionName(localLease, remoteLease)
	ikeConfMap[connectionName] = ikeConf

	err = client.LoadConn(&ikeConfMap)
	if err != nil {
		return err
	}

	log.Infof("Loaded connection: %v", connectionName)
	return nil
}

func (charon *CharonIKEDaemon) UnloadCharonConnection(localLease,
	remoteLease *subnet.Lease) error {
	client, err := charon.getClient()
	if err != nil {
		return err
	}
	defer client.Close()

	connectionName := formatConnectionName(localLease, remoteLease)
	unloadConnRequest := &goStrongswanVici.UnloadConnRequest{
		Name: connectionName,
	}

	err = client.UnloadConn(unloadConnRequest)
	if err != nil {
		return err
	}

	log.Infof("Unloaded connection: %v", connectionName)
	return nil
}

func formatConnectionName(localLease, remoteLease *subnet.Lease) string {
	return fmt.Sprintf("%s-%s-%s-%s", localLease.Attrs.PublicIP,
		localLease.Subnet, remoteLease.Subnet, remoteLease.Attrs.PublicIP)
}

func formatChildSAConfName(localLease, remoteLease *subnet.Lease) string {
	return fmt.Sprintf("%s-%s", localLease.Subnet, remoteLease.Subnet)
}
