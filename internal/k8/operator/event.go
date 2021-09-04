package operator

import "k8s.io/apimachinery/pkg/runtime"

type EventType string

const (
	CREATED = EventType("created")
	UPDATED = EventType("updated")
	DELETED = EventType("deleted")
)

type Event interface {
	GetAction() EventType
	GetKey() string
	GetObject() runtime.Object
}

type event struct {
	key    string
	obj    runtime.Object
	action EventType
}

func (e *event) GetKey() string {
	return e.key
}

func (e *event) GetAction() EventType {
	return e.action
}

func (e *event) GetObject() runtime.Object {
	return e.obj
}

type updateEvent struct {
	key    string
	oldObj runtime.Object
	newObj runtime.Object
	action EventType
}

func (e *updateEvent) GetKey() string {
	return e.key
}

func (e *updateEvent) GetAction() EventType {
	return e.action
}

func (e *updateEvent) GetObject() runtime.Object {
	return e.newObj
}

