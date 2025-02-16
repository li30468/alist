package task

import (
	"github.com/alist-org/alist/v3/pkg/generic_sync"
	log "github.com/sirupsen/logrus"
)

type Manager[K comparable] struct {
	workerC  chan struct{}
	curID    K
	updateID func(*K)
	tasks    generic_sync.MapOf[K, *Task[K]]
}

func (tm *Manager[K]) Submit(task *Task[K]) K {
	if tm.updateID != nil {
		task.ID = tm.curID
		tm.updateID(&task.ID)
	}
	tm.tasks.Store(task.ID, task)
	tm.do(task)
	return task.ID
}

func (tm *Manager[K]) do(task *Task[K]) {
	go func() {
		log.Debugf("task [%s] waiting for worker", task.Name)
		select {
		case <-tm.workerC:
			log.Debugf("task [%s] starting", task.Name)
			task.run()
			log.Debugf("task [%s] ended", task.Name)
		}
		// return worker
		tm.workerC <- struct{}{}
	}()
}

func (tm *Manager[K]) GetAll() []*Task[K] {
	return tm.tasks.Values()
}

func (tm *Manager[K]) Get(tid K) (*Task[K], bool) {
	return tm.tasks.Load(tid)
}

func (tm *Manager[K]) MustGet(tid K) *Task[K] {
	task, _ := tm.Get(tid)
	return task
}

func (tm *Manager[K]) Retry(tid K) error {
	t, ok := tm.Get(tid)
	if !ok {
		return ErrTaskNotFound
	}
	tm.do(t)
	return nil
}

func (tm *Manager[K]) Cancel(tid K) error {
	t, ok := tm.Get(tid)
	if !ok {
		return ErrTaskNotFound
	}
	t.Cancel()
	return nil
}

func (tm *Manager[K]) Remove(tid K) {
	tm.tasks.Delete(tid)
}

// RemoveAll removes all tasks from the manager, this maybe shouldn't be used
// because the task maybe still running.
func (tm *Manager[K]) RemoveAll() {
	tm.tasks.Clear()
}

func (tm *Manager[K]) RemoveFinished() {
	tasks := tm.GetAll()
	for _, task := range tasks {
		if task.Status == FINISHED {
			tm.Remove(task.ID)
		}
	}
}

func (tm *Manager[K]) RemoveError() {
	tasks := tm.GetAll()
	for _, task := range tasks {
		if task.Error != nil {
			tm.Remove(task.ID)
		}
	}
}

func NewTaskManager[K comparable](maxWorker int, updateID ...func(*K)) *Manager[K] {
	tm := &Manager[K]{
		tasks:   generic_sync.MapOf[K, *Task[K]]{},
		workerC: make(chan struct{}, maxWorker),
	}
	for i := 0; i < maxWorker; i++ {
		tm.workerC <- struct{}{}
	}
	if len(updateID) > 0 {
		tm.updateID = updateID[0]
	}
	return tm
}
