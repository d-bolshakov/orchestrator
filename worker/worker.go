package worker

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/d-bolshakov/orchestrator/store"
	"github.com/d-bolshakov/orchestrator/task"
	"github.com/golang-collections/collections/queue"
)

type Worker struct {
	Name      string
	Queue     queue.Queue
	Db        store.Store[*task.Task]
	TaskCount int

	Stats *Stats
}

func (w *Worker) CollectStats() {
	for {
		log.Println("Collecting stats")
		w.Stats = GetStats()
		w.Stats.TaskCount = w.TaskCount
		time.Sleep(time.Second * 15)
	}
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
			}
		} else {
			log.Printf("No tasks to process currently.\n")
		}
		log.Println("Sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func (w *Worker) runTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("No tasks in the queue")
		return task.DockerResult{
			Error: nil,
		}
	}

	taskQueued := t.(task.Task)

	taskPersisted, err := w.Db.Get(taskQueued.ID.String())
	if err != nil {
		// TODO: differentiate between the previously non-existent task and error which occurred during the retrieval
		taskPersisted = &taskQueued
		err := w.Db.Put(taskQueued.ID.String(), &taskQueued)
		if err != nil {
			msg := fmt.Errorf("error storing task %s: %v", taskQueued.ID.String(), err)
			log.Println(msg)
			return task.DockerResult{
				Error: msg,
			}
		}
	}

	var result task.DockerResult
	// TODO: refactor this shit
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(taskQueued)

		case task.Completed:
			result = w.StopTask(taskQueued)

		default:
			result.Error = errors.New("we should not get here")
		}
	} else {
		err := fmt.Errorf("invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		result.Error = err
	}
	return result
}

func (w *Worker) StartTask(t task.Task) task.DockerResult {
	t.StartTime = time.Now().UTC()
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	result := d.Run()
	if result.Error != nil {
		log.Printf("Error running task %s: %v\n", t.ID, result.Error)
		t.State = task.Failed
		w.Db.Put(t.ID.String(), &t)
		return result
	}

	t.ContainerID = result.ContainerId
	t.State = task.Running
	w.Db.Put(t.ID.String(), &t)

	return result
}

func (w *Worker) StopTask(t task.Task) task.DockerResult {
	config := task.NewConfig(&t)
	d := task.NewDocker(config)

	stopResult := d.Stop(t.ContainerID)
	if stopResult.Error != nil {
		log.Printf("Error stopping container %s: %v\n", t.ID, stopResult.Error)
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db.Put(t.ID.String(), &t)
	log.Printf("Stopped and removed container %s for task %s", t.ContainerID, t.ID)

	return stopResult
}

func (w *Worker) GetTasks() []*task.Task {
	tasks, err := w.Db.List()
	if err != nil {
		log.Printf("Error getting list of tasks: %v\n", err)
		return nil
	}
	return tasks
}

func (w *Worker) InspectTask(t task.Task) task.DockerInspectResponse {
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	return d.Inspect(t.ContainerID)
}

func (w *Worker) updateTasks() error {
	persistedTasks, err := w.Db.List()
	if err != nil {
		return err
	}

	for id, t := range persistedTasks {
		if t.State == task.Running {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				fmt.Printf("ERROR: %v\n", resp.Error)
				continue
			}

			if resp.Container == nil {
				log.Printf("No container for running task %d\n", id)
				t.State = task.Failed
				w.Db.Put(t.ID.String(), t)
				continue
			}

			if resp.Container.State.Status == "exited" {
				log.Printf("Container for task %s in non-running state %s", t.ID, resp.Container.State.Status)
				t.State = task.Failed
				w.Db.Put(t.ID.String(), t)
				continue
			}

			t.HostPorts = resp.Container.NetworkSettings.NetworkSettingsBase.Ports
			w.Db.Put(t.ID.String(), t)
		}
	}

	return nil
}

func (w *Worker) UpdateTasks() {
	for {
		log.Println("Checking status of tasks")

		err := w.updateTasks()
		if err != nil {
			log.Printf("Error updating tasks: %v\n", err)
		}
		log.Println("Task updates completed")

		log.Println("Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func New(dbType string) *Worker {
	db := store.NewOfType[*task.Task](dbType)
	return &Worker{
		Db:    db,
		Queue: *queue.New(),
	}
}
