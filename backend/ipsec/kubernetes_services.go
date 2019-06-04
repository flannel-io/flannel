package ipsec

import (
	kubernetes "github.com/coreos/flannel/kube"
	log "github.com/golang/glog"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func getServiceChannel() <-chan watch.Event {
	c := kubernetes.Instance.Client
	// TODO: Check, if kube-system is correct.
	watchChannel, err := c.Services("kube-system").Watch(v1.ListOptions{})
	if err != nil {
		log.Warningf("Could not watch for services changes")
		return nil
	}

	return watchChannel.ResultChan()

}

func monitorServices() {

	serviceChannel := getServiceChannel()

	for {
		// TODO: Do things, if services changes
		_, ok := <-serviceChannel

		if !ok {
			log.Info("APIServer ended our service watch, establishing a new watch.")
			serviceChannel = getServiceChannel()
		}
	}
}
