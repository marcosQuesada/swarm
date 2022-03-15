package cmd

import (
	"fmt"
	"github.com/marcosQuesada/swarm/internal/k8/operator"
	"github.com/marcosQuesada/swarm/pkg/apis/swarm/v1alpha1"
	clientset "github.com/marcosQuesada/swarm/pkg/generated/clientset/versioned"
	informers "github.com/marcosQuesada/swarm/pkg/generated/informers/externalversions"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// controllerCmd represents the controller command
var controllerCmd = &cobra.Command{
	Use:   "controller",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("controller called")

		err := v1alpha1.AddToScheme(scheme.Scheme)
		if err != nil {
			log.Fatalf("unable to add type to scheme %v", err)
		}

		kubeConfigPath := os.Getenv("HOME") + "/.kube/config"

		cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			log.Fatalf("getClusterConfig: %v", err)
		}

		swarmClient, err := clientset.NewForConfig(cfg)
		if err != nil {
			log.Fatalf("clientset creation fatal: %v", err)
		}

		kubeClient, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
		}

		kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*10)
		swarmInformerFactory := informers.NewSharedInformerFactory(swarmClient, time.Minute*10)

		podInformer := kubeInformerFactory.Core().V1().Pods()
		swarmInformer := swarmInformerFactory.K8slab().V1alpha1().Swarms()
		controller := operator.NewController(kubeClient, swarmClient, podInformer, swarmInformer)

		// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh))
		// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
		kubeInformerFactory.Start(wait.NeverStop)
		swarmInformerFactory.Start(wait.NeverStop)

		stopCh := make(chan struct{})
		go func() {
			sigTerm := make(chan os.Signal, 1)
			signal.Notify(sigTerm, syscall.SIGTERM, syscall.SIGINT)
			<-sigTerm
			close(stopCh)
		}()

		if err = controller.Run(2, stopCh); err != nil {
			klog.Fatalf("Error running controller: %s", err.Error())
		}

	},
}

func init() {
	rootCmd.AddCommand(controllerCmd)
}
