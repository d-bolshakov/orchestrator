package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/d-bolshakov/orchestrator/node"
)

type ManagerClient struct {
	Client
}

func (mc *ManagerClient) GetNodes() ([]*node.Node, error) {
	url := fmt.Sprintf("http://%s/nodes", mc.address)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error connecting to %s: %v", mc.address, err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error retrieving the node list from %s: %v", mc.address, err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var nodes []*node.Node
	json.Unmarshal(body, &nodes)
	return nodes, nil
}

func NewManagerClient(address string) *ManagerClient {
	return &ManagerClient{
		Client: Client{
			address: address,
			role:    "manager",
		},
	}
}
