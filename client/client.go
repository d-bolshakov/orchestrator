package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/d-bolshakov/orchestrator/task"
	"github.com/d-bolshakov/orchestrator/worker"
)

type Client struct {
	address string
	role    string
}

func (c *Client) GetTasks() ([]*task.Task, error) {
	url := fmt.Sprintf("%s/tasks", c.address)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error connecting to %s %v: %v\n", c.role, c.address, err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("error sending request: %v\n", err)
		return nil, err
	}

	d := json.NewDecoder(resp.Body)
	var tasks []*task.Task
	err = d.Decode(&tasks)
	if err != nil {
		log.Printf("Error unmarshalling tasks: %s\n", err.Error())
		return nil, err
	}
	return tasks, err
}

func (c *Client) SendTask(te task.TaskEvent) (*task.Task, error) {
	data, err := json.Marshal(te)
	if err != nil {
		log.Printf("Unable to marshal task object: %v.", te)
		return nil, err
	}

	url := fmt.Sprintf("%s/tasks", c.address)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Error connecting to %v: %v", c.address, err)
		return nil, err
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := worker.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			log.Printf("Error decoding response: %s\n", err.Error())
			return nil, err
		}
		log.Printf("Response error (%d): %s\n", e.HTTPStatusCode, e.Message)
		return nil, err
	}

	newTask := task.Task{}
	err = d.Decode(&newTask)
	if err != nil {
		log.Printf("Error decoding response: %s\n", err.Error())
		return nil, err
	}
	return &newTask, nil
}

func (c *Client) StopTask(taskID string) error {
	client := &http.Client{}
	url := fmt.Sprintf("%s/tasks/%s", c.address, taskID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("error creating request to delete task %s: %v\n", taskID, err)
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error connecting to %s at %s: %v\n", c.role, c.address, err)
		return err
	}

	if resp.StatusCode != 204 {
		log.Printf("Error sending request: %v\n", err)
		return err
	}

	log.Printf("task %s has been scheduled to be stopped", taskID)
	return nil
}

func New(address string, role string) *Client {
	return &Client{
		address: toHttpUrl(address),
		role:    role,
	}
}

func toHttpUrl(addr string) string {
	if strings.HasPrefix(addr, "http://") {
		return addr
	}

	return fmt.Sprintf("http://%s", addr)
}
