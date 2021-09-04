package operator

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Handler interface {
	Created(ctx context.Context, obj runtime.Object)
	Updated(ctx context.Context, new runtime.Object, old runtime.Object)
	Deleted(ctx context.Context, obj runtime.Object)
}

type ListWatcher interface {
	List(options metav1.ListOptions) (runtime.Object, error)
	Watch(options metav1.ListOptions) (watch.Interface, error)
}

type controller struct {
	client   kubernetes.Interface
	informer cache.SharedIndexInformer
	queue    workqueue.RateLimitingInterface
	handler  Handler
	ready    chan struct{}
}

func Build(handler Handler, listenObj runtime.Object, watcher ListWatcher) *controller {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  watcher.List,
			WatchFunc: watcher.Watch,
		},
		listenObj,
		0, // No Resync for now
		cache.Indexers{},
	)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			log.Infof("Add obj: %s", key)
			if err != nil {
				log.Errorf("Add MetaNamespaceKeyFunc error %v", err)
				return
			}
			queue.Add(&event{
				key:    key,
				obj:    obj.(runtime.Object),
				action: CREATED,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			log.Infof("Update obj: %s", key)
			if err != nil {
				log.Errorf("Patch MetaNamespaceKeyFunc error %v", err)
				return
			}

			queue.Add(&updateEvent{
				key:    key,
				newObj: newObj.(runtime.Object),
				oldObj: oldObj.(runtime.Object),
				action: UPDATED,
			})
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			log.Infof("Delete obj: %s", key)
			if err != nil {
				log.Errorf("Delete DeletionHandlingMetaNamespaceKeyFunc error %v", err)
				return
			}
			queue.Add(&event{
				key:    key,
				obj:    obj.(runtime.Object),
				action: DELETED,
			})
		},
	})

	return &controller{
		informer: informer,
		queue:    queue,
		handler:  handler,
		ready:    make(chan struct{}),
	}
}

func (c *controller) Run(stopCh <-chan struct{}) {
	log.Info("controller.Run: initiating")

	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	go c.informer.Run(stopCh)

	// wait until sync resources
	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(errors.New("error syncing cache"))
		return
	}

	close(c.ready)

	// run the runWorker method every second with a stop channel
	wait.Until(c.runWorker, time.Second, stopCh)
}

func (c *controller) WaitUntilReady() {
	<-c.ready
}

// HasSynced allows us to satisfy the controller interface
// by wiring up the informer's HasSynced method to it
func (c *controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// runWorker executes the loop to process new items added to the queue
func (c *controller) runWorker() {
	log.Info("controller.runWorker: starting")

	for c.processNextItem() {
	}
}

func (c *controller) processNextItem() bool {
	ev, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(ev)

	e := ev.(Event)
	item, exists, err := c.informer.GetIndexer().GetByKey(e.GetKey())
	if err != nil {
		log.Errorf("controller.processNextItem: Failed processing item with key %s with error %vs", e.GetKey(), err)
		c.backoffRetry(e, err)
	}

	log.Infof("controller.processNextItem: key %s iten type %T exists %v ", e.GetKey(), item, exists)

	obj, ok := e.GetObject().(runtime.Object)
	if !ok {
		log.Errorf("object update not a pod type %T", e.GetObject())
		return true
	}

	ctx := context.Background()
	switch e.GetAction() {
	case CREATED:
		c.handler.Created(ctx, obj.DeepCopyObject())
	case UPDATED:
		u := e.(*updateEvent)
		c.handler.Updated(ctx, u.newObj.DeepCopyObject(), u.oldObj.DeepCopyObject())
	case DELETED:
		c.handler.Deleted(ctx, obj.DeepCopyObject())
	}

	c.queue.Forget(ev)

	return true
}

func (c *controller) backoffRetry(ev Event, err error) {
	if c.queue.NumRequeues(ev) >= 5 {
		log.Errorf("controller.processNextItem: key %s with error %v, no more retries", ev.GetKey(), err)
		c.queue.Forget(ev)
		utilruntime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(ev)
}
