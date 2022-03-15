package operator

import (
	"context"
	swarmv1alpha1 "github.com/marcosQuesada/swarm/pkg/generated/clientset/versioned/typed/swarm/v1alpha1"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type adapter struct {
	client swarmv1alpha1.K8slabV1alpha1Interface
}

func NewAdapter(c swarmv1alpha1.K8slabV1alpha1Interface) ListWatcher {
	return &adapter{client: c}
}

func (a *adapter) List(options metav1.ListOptions) (runtime.Object, error) {
	return a.client.Swarms(api.NamespaceDefault).List(context.Background(), options)
}

func (a *adapter) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return a.client.Swarms(api.NamespaceDefault).Watch(context.Background(), options)
}
