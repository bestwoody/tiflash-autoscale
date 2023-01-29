package autoscale

import (
	"context"
	"encoding/json"

	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// patchStringValue specifies a patch operation for a string.
type KubePatchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func CloneSetPatchImage(cli *kruiseclientset.Clientset, ns string, clonesetName string, newImage string) error {
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

func GetReplicaOfStatefulSet(k8sCli *kubernetes.Clientset, ns string, name string) (int, int, error) {
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
