package google

import (
	"io/ioutil"
	"net/http"
	"path"
)

func networkFromMetadata() (string, error) {
	network, err := metadataGet("/instance/network-interfaces/0/network")
	if err != nil {
		return "", err
	}
	return path.Base(network), nil
}

func projectFromMetadata() (string, error) {
	return metadataGet("/project/project-id")
}

func metadataGet(path string) (string, error) {
	req, err := http.NewRequest("GET", metadataEndpoint+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
