package autoscale

import (
	"context"
	"encoding/json"
	kruiseclientfake "github.com/openkruise/kruise-api/client/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	k8sclientfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	metricclientfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

func initK8sEnv(Namespace string) (config *restclient.Config, K8sCli kubernetes.Interface, MetricsCli metricsv.Interface, Cli kruiseclientset.Interface) {
	if OptionRunMode == RunModeLocal {
		config = &rest.Config{
			// Set the necessary fields for an in-cluster config
		}
		K8sCli = k8sclientfake.NewSimpleClientset()

		MetricsCli = metricclientfake.NewSimpleClientset()

		Cli = kruiseclientfake.NewSimpleClientset()
		return config, K8sCli, MetricsCli, Cli
	}
	config, err := getK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	MetricsCli, err = metricsv.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	K8sCli, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	Cli = kruiseclientset.NewForConfigOrDie(config)

	// create NameSpace if not exsist
	_, err = K8sCli.CoreV1().Namespaces().Get(context.TODO(), Namespace, metav1.GetOptions{})
	if err != nil {
		_, err = K8sCli.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: Namespace,
				Labels: map[string]string{
					"ns": Namespace,
				}}}, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}
	}
	return config, K8sCli, MetricsCli, Cli
}

// patchStringValue specifies a patch operation for a string.
type KubePatchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func CloneSetPatchImage(cli kruiseclientset.Interface, ns string, clonesetName string, newImage string) error {
	payload := []KubePatchStringValue{{
		Op:    "replace",
		Path:  "/spec/template/spec/containers/0/image",
		Value: newImage,
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = cli.AppsV1alpha1().CloneSets(ns).Patch(context.TODO(), clonesetName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	return err
}

func GetReplicaOfStatefulSet(k8sCli kubernetes.Interface, ns string, name string) (int, int, error) {
	ret, err := k8sCli.AppsV1().StatefulSets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return 0, 0, err
	} else {
		specReplica := 1
		if ret.Spec.Replicas != nil {
			specReplica = int(*ret.Spec.Replicas)
		}
		return specReplica, int(ret.Status.Replicas), nil
	}
}
