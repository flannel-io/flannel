package kubernetes

import (
	"fmt"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Client *clientset.Clientset
}

var Instance *Client

func Create(apiUrl string, kubeConfig string) error {

	Instance = &Client{}

	var cfg *rest.Config
	var err error
	// Use out of cluster config if the URL or kubeconfig have been specified. Otherwise use incluster config.
	if apiUrl != "" || kubeConfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags(apiUrl, kubeConfig)
		if err != nil {
			return fmt.Errorf("unable to create k8s config: %v", err)
		}
	} else {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("unable to initialize inclusterconfig: %v", err)
		}
	}

	Instance.Client, err = clientset.NewForConfig(cfg)

	if err != nil {
		return fmt.Errorf("unable to initialize k: %v", err)
	}

	return nil

}
