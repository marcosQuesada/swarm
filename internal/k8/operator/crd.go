package operator

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"net"
	"reflect"
	"sync"

	v1alpha "github.com/marcosQuesada/swarm/pkg/apis/swarm/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

type Pool interface {
	Add(idx int, id string, add net.IP) error
}

type handler struct {
	lastState map[string]*v1alpha.Swarm
	mutex     sync.RWMutex
	pool      Pool
}

func NewHandler(p Pool) Handler {
	return &handler{
		lastState: make(map[string]*v1alpha.Swarm),
		pool:      p,
	}
}

func (h *handler) Created(_ context.Context, obj runtime.Object) {
	sw := obj.(*v1alpha.Swarm)
	log.Infof("Created CRD %s", sw.Name)

	h.mutex.Lock()
	defer h.mutex.Unlock()
	if _, ok := h.lastState[sw.Name]; ok {
		log.Errorf("Swarm creation error, %s already exists on registry", sw.Name)
		return
	}

	for _, peer := range sw.Spec.Peers {
		err := h.pool.Add(peer.Index, peer.ID, net.ParseIP(peer.Address))
		if err != nil {
			log.Errorf("error adding raft node, %v peer %v", err, peer)
		}
	}

	h.lastState[sw.Name] = sw
}

func (h *handler) Updated(_ context.Context, new runtime.Object, old runtime.Object) {
	oldObj := old.(*v1alpha.Swarm)
	newObj := new.(*v1alpha.Swarm)
	log.Infof("Updated CRD %s", newObj.Name)

	// @TODO: DIG ON IT!
	report := func(raw reflect.Type) bool {
		log.Infof("Raw DIff type %s", raw.String())
		return true
	}
	opt := cmp.Exporter(report)
	diff := cmp.Diff(oldObj, newObj, opt)
	fmt.Printf("%s\n", diff)

	h.mutex.Lock()
	defer h.mutex.Unlock()

	if oldObj.Spec.Size != newObj.Spec.Size && oldObj.Spec.Size < newObj.Spec.Size { // @TODO: HAPPY PATH!
		for _, peer := range newObj.Spec.Peers {
			_ = h.pool.Add(peer.Index, peer.ID, net.ParseIP(peer.Address))
		}
	}

	h.lastState[newObj.Name] = newObj
}

func (h *handler) Deleted(_ context.Context, obj runtime.Object) {
	cl := obj.(*v1alpha.Swarm)
	log.Infof("Deleting CRD %s", cl.Name)

	h.mutex.Lock()
	defer h.mutex.Unlock()
	delete(h.lastState, cl.Name)
}

func (h *handler) Get(_ context.Context, swarmName string) (*v1alpha.Swarm, error) {
	log.Infof("Get CRD %s", swarmName)

	h.mutex.RLock()
	defer h.mutex.RUnlock()
	v, ok := h.lastState[swarmName]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found in registry", swarmName)
	}

	return v, nil
}
