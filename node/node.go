package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/d-bolshakov/orchestrator/utils"
	"github.com/d-bolshakov/orchestrator/worker"
)

type Node struct {
	Name            string
	Ip              string
	Cores           int
	Memory          int64
	MemoryAllocated int64
	Disk            int64
	DiskAllocated   int64
	Role            string
	TaskCount       int
	Stats           worker.Stats
}

func (n *Node) GetStats() (*worker.Stats, error) {
	url := fmt.Sprintf("%s/stats", n.Ip)
	resp, err := utils.HTTPWithRetry(http.Get, url)
	if err != nil {
		msg := fmt.Sprintf("Unable to connect to %v. Permanent failure.", n.Ip)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("Error retrieving stats from %v: %v", n.Ip, err)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var stats worker.Stats
	err = json.Unmarshal(body, &stats)
	if err != nil {
		msg := fmt.Sprintf("error decoding message while getting stats for node %s", n.Name)
		log.Println(msg)
		return nil, errors.New(msg)
	}

	n.Memory = int64(stats.MemTotalKb())
	n.Disk = int64(stats.DiskTotal())

	n.Stats = stats
	n.TaskCount = stats.TaskCount
	return &n.Stats, nil
}

func New(name string, address string, role string) *Node {
	return &Node{
		Name: name,
		Ip:   address,
		Role: role,
	}
}
