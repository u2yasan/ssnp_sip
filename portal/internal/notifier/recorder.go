package notifier

import (
	"context"
	"errors"
	"sync"
)

type Recorder struct {
	mu            sync.Mutex
	Notifications []Notification
	Err           error
}

func (r *Recorder) Send(_ context.Context, notification Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.Notifications = append(r.Notifications, notification)
	return nil
}

func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Notifications)
}

func (r *Recorder) Last() (Notification, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Notifications) == 0 {
		return Notification{}, false
	}
	return r.Notifications[len(r.Notifications)-1], true
}

func FailingRecorder(message string) *Recorder {
	return &Recorder{Err: errors.New(message)}
}
