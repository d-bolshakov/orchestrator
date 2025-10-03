package manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/d-bolshakov/orchestrator/node"
	"github.com/d-bolshakov/orchestrator/scheduler"
	"github.com/d-bolshakov/orchestrator/store"
	"github.com/d-bolshakov/orchestrator/task"
	"github.com/d-bolshakov/orchestrator/worker"
	"github.com/docker/go-connections/nat"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

type Manager struct {
	Pending       queue.Queue
	TaskDb        store.Store[*task.Task]
	EventDb       store.Store[*task.TaskEvent]
	Workers       []string
	WorkerNodes   []*node.Node
	WorkerTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
	LastWorker    int
	Scheduler     scheduler.Scheduler
}

func (m *Manager) AddTask(te task.TaskEvent) {
	m.Pending.Enqueue(te)
}

func (m *Manager) SelectWorker(t task.Task) (*node.Node, error) {
	candidates := m.Scheduler.SelectCandidateNodes(t, m.WorkerNodes)
	if candidates == nil {
		msg := fmt.Sprintf("No available candidates match resource request for task %v", t.ID)
		return nil, errors.New(msg)
	}
	scores := m.Scheduler.Score(t, candidates)
	selectedNode := m.Scheduler.Pick(scores, candidates)

	return selectedNode, nil
}

func (m *Manager) UpdateTasks() {
	for {
		fmt.Printf("Checking for task updates from workers\n")
		m.updateTasks()
		log.Println("Task updates completed")
		log.Println("Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (m *Manager) updateTasks() {
	for _, worker := range m.Workers {
		log.Printf("Checking worker %v for task updates", worker)

		url := fmt.Sprintf("http://%s/tasks", worker)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Error connecting to %v: %v\n", worker, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("error sending request: %v\n", err)
			continue
		}

		d := json.NewDecoder(resp.Body)
		var tasks []*task.Task
		err = d.Decode(&tasks)
		if err != nil {
			log.Printf("Error unmarshalling tasks: %s\n", err.Error())
			continue
		}

		for _, t := range tasks {
			log.Printf("Attempting to update task %v\n", t.ID)

			task, err := m.TaskDb.Get(t.ID.String())
			if err != nil {
				log.Printf("Error retrieving task %s from DB: %v\n", t.ID, err)
				continue
			}

			if task.State != t.State {
				task.State = t.State
			}

			task.StartTime = t.StartTime
			task.FinishTime = t.FinishTime
			task.ContainerID = t.ContainerID
			task.HostPorts = t.HostPorts

			m.TaskDb.Put(t.ID.String(), task)
		}
	}
}

func (m *Manager) SendWork() {
	if m.Pending.Len() == 0 {
		log.Println("No work in the queue")
		return
	}

	e := m.Pending.Dequeue()
	te := e.(task.TaskEvent)
	m.EventDb.Put(te.ID.String(), &te)
	log.Printf("Pulled %v off pending queue\n", te)

	taskWorker, ok := m.TaskWorkerMap[te.Task.ID]
	if ok {
		persistedTask, err := m.TaskDb.Get(te.Task.ID.String())
		if err != nil {
			log.Printf("Error occurred retrieving task %s from DB: %v\n", te.Task.ID, err)
			return
		}

		if te.State == task.Completed && task.ValidStateTransition(persistedTask.State, te.State) {
			m.stopTask(taskWorker, te.Task.ID.String())
			return
		}

		log.Printf("invalid request: existing task %s is in state %v and cannon transition to the completed state\n", persistedTask.ID.String(), persistedTask.State)
		return
	}

	t := te.Task
	w, err := m.SelectWorker(t)
	if err != nil {
		log.Printf("error selecting worker for task %s: %v\n", t.ID, err)
		return
	}

	m.WorkerTaskMap[w.Name] = append(m.WorkerTaskMap[w.Name], te.Task.ID)
	m.TaskWorkerMap[t.ID] = w.Name

	t.State = task.Scheduled
	m.TaskDb.Put(t.ID.String(), &t)

	data, err := json.Marshal(te)
	if err != nil {
		log.Printf("Unable to marshal task object: %v.\n", t)
		return
	}

	url := fmt.Sprintf("http://%s/tasks", w.Name)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Error connecting to %v: %v\n", w, err)
		m.Pending.Enqueue(&te)
		return
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := worker.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			fmt.Printf("Error decoding response: %s\n", err.Error())
			return
		}
		log.Printf("Response error (%d): %s", e.HTTPStatusCode, e.Message)
		return
	}

	t = task.Task{}
	err = d.Decode(&t)
	if err != nil {
		fmt.Printf("Error decoding response: %s\n", err.Error())
		return
	}
	log.Printf("%v\n", t)
}

func (m *Manager) ProcessTasks() {
	for {
		log.Println("Processing any tasks in the queue")
		m.SendWork()
		log.Println("Sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func (m *Manager) GetTasks() []*task.Task {
	tasks, err := m.TaskDb.List()
	if err != nil {
		log.Printf("Error getting list of tasks: %v\n", err)
		return nil
	}
	return tasks
}

func (m *Manager) checkTaskHealth(t task.Task) error {
	log.Printf("Calling health check for task %s: %s\n", t.ID, t.HealthCheck)

	w := m.TaskWorkerMap[t.ID]
	hostPort := getHostPort(t.HostPorts)
	if hostPort == nil {
		log.Println("No port known for task %s, skipping", t.ID)
		return nil
	}

	worker := strings.Split(w, ":")
	url := fmt.Sprintf("http://%s:%s%s", worker[0], *hostPort, t.HealthCheck)
	log.Printf("Calling health check for task %sL %s\n", t.ID, url)
	resp, err := http.Get(url)
	if err != nil {
		msg := fmt.Sprintf("Error connecting to health check %s", url)
		log.Println(msg)
		return errors.New(msg)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Error health check for task %s did not return 200\n", t.ID)
		log.Println(msg)
		return errors.New(msg)
	}

	log.Printf("Task %s health check response: %v\n", t.ID, resp.StatusCode)

	return nil
}

func (m *Manager) doHeathChecks() {
	for _, t := range m.GetTasks() {
		if t.State == task.Running {
			err := m.checkTaskHealth(*t)
			if err != nil {
				if t.RestartCount < 3 {
					m.restartTask(t)
				}
			}
		} else if t.State == task.Failed && t.RestartCount < 3 {
			m.restartTask(t)
		}
	}
}

func (m *Manager) DoHeathChecks() {
	for {
		log.Println("Performing task health check")
		m.doHeathChecks()
		log.Println("Task health checks completed")
		log.Println("Sleeping for 60 seconds")
		time.Sleep(60 * time.Second)
	}
}

func (m *Manager) restartTask(t *task.Task) {
	w := m.TaskWorkerMap[t.ID]
	t.State = task.Scheduled
	t.RestartCount++
	m.TaskDb.Put(t.ID.String(), t)

	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Running,
		Timestamp: time.Now(),
		Task:      *t,
	}
	data, err := json.Marshal(te)
	if err != nil {
		log.Printf("Unable to marshal task object: %v.", t)
		return
	}

	url := fmt.Sprintf("http://%s/tasks", w)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("Error connecting to %v: %v", w, err)
		m.Pending.Enqueue(t)
		return
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := worker.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			fmt.Printf("Error decoding response: %s\n", err.Error())
			return
		}
		log.Printf("Response error (%d): %s\n", e.HTTPStatusCode, e.Message)
		return
	}

	newTask := task.Task{}
	err = d.Decode(&newTask)
	if err != nil {
		fmt.Printf("Error decoding response: %s\n", err.Error())
		return
	}
	log.Printf("%#v\n", t)
}

func (m *Manager) stopTask(worker string, taskID string) {
	client := &http.Client{}
	url := fmt.Sprintf("http://%s/tasks/%s", worker, taskID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("error creating request to delete task %s: %v\n", taskID, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error connecting to worker at %s: %v\n", worker, err)
		return
	}

	if resp.StatusCode != 204 {
		log.Printf("Error sending request: %v\n", err)
		return
	}

	log.Printf("task %s has been scheduled to be stopped", taskID)
}

func New(workers []string, schedulerType string, dbType string) *Manager {
	taskDb := store.NewOfType[*task.Task](dbType, "tasks")
	eventDb := store.NewOfType[*task.TaskEvent](dbType, "task_events")
	workerTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	nodes := []*node.Node{}
	for worker := range workers {
		workerTaskMap[workers[worker]] = []uuid.UUID{}

		nAPI := fmt.Sprintf("http://%v", workers[worker])
		n := node.New(workers[worker], nAPI, "worker")
		nodes = append(nodes, n)
	}

	return &Manager{
		Pending:       *queue.New(),
		Workers:       workers,
		WorkerNodes:   nodes,
		TaskDb:        taskDb,
		EventDb:       eventDb,
		WorkerTaskMap: workerTaskMap,
		TaskWorkerMap: taskWorkerMap,
		LastWorker:    0,
		Scheduler:     scheduler.NewOfType(schedulerType),
	}
}

func getHostPort(ports nat.PortMap) *string {
	for k := range ports {
		return &ports[k][0].HostPort
	}

	return nil
}
