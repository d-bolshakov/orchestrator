package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/d-bolshakov/orchestrator/manager"
	"github.com/d-bolshakov/orchestrator/task"
	"github.com/d-bolshakov/orchestrator/worker"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

func main() {
	whost := os.Getenv("ORCHESTRATOR_WORKER_HOST")
	wport, _ := strconv.Atoi(os.Getenv("ORCHESTRATOR_WORKER_PORT"))

	mhost := os.Getenv("ORCHESTRATOR_MANAGER_HOST")
	mport, _ := strconv.Atoi(os.Getenv("ORCHESTRATOR_MANAGER_PORT"))

	fmt.Println("Starting worker")
	w := worker.Worker{
		Queue: *queue.New(),
		Db:    make(map[uuid.UUID]*task.Task),
	}
	wapi := worker.Api{Address: whost, Port: wport, Worker: &w}

	go w.RunTasks()
	go w.CollectStats()
	go wapi.Start()

	fmt.Println("Starting manager")
	workers := []string{fmt.Sprintf("%s:%d", whost, wport)}
	m := manager.New(workers)
	mapi := manager.Api{Address: mhost, Port: mport, Manager: m}

	go m.ProcessTasks()
	go m.UpdateTasks()

	mapi.Start()
}
