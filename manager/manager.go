package manager

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/d-bolshakov/orchestrator/client"
	"github.com/d-bolshakov/orchestrator/node"
	"github.com/d-bolshakov/orchestrator/scheduler"
	"github.com/d-bolshakov/orchestrator/store"
	"github.com/d-bolshakov/orchestrator/task"
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
	for _, w := range m.Workers {
		log.Printf("Checking worker %v for task updates", w)

		client := client.New(w, "worker")
		tasks, err := client.GetTasks()
		if err != nil {
			log.Printf("Error retrieving tasks from worker %s: %v\n", w, err)
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

	client := client.New(w.Ip, "worker")
	_, err = client.SendTask(te)
	if err != nil {
		log.Printf("Error sending task %s to worker %s: %v\n", t.ID, w.Ip, err)
		m.Pending.Enqueue(te)
	}
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
	client := client.New(w, "worker")
	_, err := client.SendTask(te)
	if err != nil {
		log.Printf("Error sending task %s to worker %s: %v\n", t.ID, w, err)
		m.Pending.Enqueue(te)
	}
}

func (m *Manager) stopTask(workerAddress string, taskID string) {
	client := client.New(workerAddress, "role")
	err := client.StopTask(taskID)
	if err != nil {
		log.Printf("Error stopping task %s: %v\n", taskID, err)
	}
}

func (m *Manager) CollectStats() {
	for {
		log.Println("Collecting stats")
		m.collectStats()
		log.Println("Finished collecting stats, sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (m *Manager) collectStats() {
	for _, node := range m.WorkerNodes {
		_, err := node.GetStats()
		if err != nil {
			log.Printf("Error retrieving stats for node %s: %v\n", node.Name, err)
			continue
		}
	}
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
