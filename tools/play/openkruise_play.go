package main

import (
	"context"
	"flag"
	"log"
	"math"
	"path/filepath"
	"time"

	kruiseclientset "github.com/openkruise/kruise-api/client/clientset/versioned"
	supervisor "github.com/tikv/pd/supervisor_proto"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func outsideConfig() (*restclient.Config, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	return clientcmd.BuildConfigFromFlags("", *kubeconfig)
}

// func GetResourcesDynamically(dynamic dynamic.Interface, ctx context.Context,
// 	group string, version string, resource string, namespace string) (
// 	[]unstructured.Unstructured, error) {

// 	resourceId := schema.GroupVersionResource{
// 		Group:    group,
// 		Version:  version,
// 		Resource: resource,
// 	}
// 	list, err := dynamic.Resource(resourceId).Namespace(namespace).
// 		List(ctx, metav1.ListOptions{})

// 	if err != nil {
// 		return nil, err
// 	}

// 	return list.Items, nil
// }

// func ExampleGetResourcesDynamically() {
// 	ctx := context.Background()
// 	config := ctrl.GetConfigOrDie()
// 	dynamic := dynamic.NewForConfigOrDie(config)

// 	namespace := "default"
// 	items, err := GetResourcesDynamically(dynamic, ctx,
// 		"apps.kruise.io", "v1alpha1", "clonesets", namespace)
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		for _, item := range items {
// 			log.Printf("%+v\n", item)
// 		}
// 	}
// }

func configmapCrtExample() {
	config, err := outsideConfig()
	clientset, err := kubernetes.NewForConfig(config)
	// clientset.AppsV1().
	if err != nil {
		panic(err.Error())
	}
	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-config-map",
		},
		Data: map[string]string{
			"ka": "va",
			"kb": "vb",
			"kc": "vc",
		},
	}
	// for {
	// get pods in all the namespaces by omitting namespace
	// Or specify namespace to get pods in particular namespace
	_, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}
	// .Pods("").List(context.TODO(), metav1.ListOptions{})
	// }
}

func configmapPlayGround() {
	config, err := outsideConfig()
	clientset, err := kubernetes.NewForConfig(config)
	// clientset.AppsV1().
	if err != nil {
		panic(err.Error())
	}
	configMapName := "my-config-map1234"
	configMap, err := clientset.CoreV1().ConfigMaps("tiflash-autoscale").Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		// panic(err.Error())
		configMap = &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: configMapName,
			},
			Data: map[string]string{},
		}
		// for {
		// get pods in all the namespaces by omitting namespace
		// Or specify namespace to get pods in particular namespace
		configMap, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Create(context.TODO(), configMap, metav1.CreateOptions{})
		if err != nil {
			panic(err.Error())
		}
	}
	println(configMap.String())
}

func configmapGetAndUpdateExample() {
	config, err := outsideConfig()
	clientset, err := kubernetes.NewForConfig(config)
	// clientset.AppsV1().
	if err != nil {
		panic(err.Error())
	}
	// for {
	// get pods in all the namespaces by omitting namespace
	// Or specify namespace to get pods in particular namespace
	configMap, err := clientset.CoreV1().ConfigMaps("tiflash-autoscale").Get(context.TODO(), "my-config-map", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	configMapCopy := configMap.DeepCopy()
	configMapCopy.Data["ka"] = "vaa"
	configMapCopy, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), configMapCopy, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
	configMapCopy.Data["ka"] = "vaaa"
	configMapCopy, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), configMapCopy, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
	configMapCopy.Data["ka"] = "vaaaa"
	_, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), configMapCopy, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func configmapPatchExample() {
	config, err := outsideConfig()
	clientset, err := kubernetes.NewForConfig(config)
	// clientset.AppsV1().
	if err != nil {
		panic(err.Error())
	}
	// for {
	// get pods in all the namespaces by omitting namespace
	// Or specify namespace to get pods in particular namespace
	configMap, err := clientset.CoreV1().ConfigMaps("tiflash-autoscale").Get(context.TODO(), "my-config-map", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	configMapCopy := configMap.DeepCopy()
	configMapCopy.Data["ka"] = "vaa"
	_, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), configMapCopy, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
	configMapCopy.Data["ka"] = "vaaa"
	_, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), configMapCopy, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
	configMapCopy.Data["ka"] = "vaaaa"
	_, err = clientset.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), configMapCopy, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func Int32Ptr(val int32) *int32 {
	ret := new(int32)
	*ret = int32(val)
	return &val
}

func OpenkruiseTest() {
	config, err := outsideConfig()
	if err != nil {
		panic(err.Error())
	}
	namespace := "default"

	kruiseClient := kruiseclientset.NewForConfigOrDie(config)

	// CloneSetName := "test-cloneset"
	// cloneSet := v1alpha1.CloneSet{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name: CloneSetName,
	// 		Labels: map[string]string{
	// 			"app": CloneSetName,
	// 		}},
	// 	Spec: v1alpha1.CloneSetSpec{
	// 		Replicas: Int32Ptr(int32(5)),
	// 		Selector: &metav1.LabelSelector{
	// 			MatchLabels: map[string]string{
	// 				"app": CloneSetName,
	// 			}},
	// 		Template: v1.PodTemplateSpec{
	// 			ObjectMeta: metav1.ObjectMeta{
	// 				Labels: map[string]string{
	// 					"app": CloneSetName,
	// 				},
	// 			},
	// 			// pod anti affinity
	// 			Spec: v1.PodSpec{
	// 				// container
	// 				Containers: []v1.Container{
	// 					{
	// 						// ENV
	// 						Env: []v1.EnvVar{
	// 							{
	// 								Name: "POD_IP",
	// 								ValueFrom: &v1.EnvVarSource{
	// 									FieldRef: &v1.ObjectFieldSelector{
	// 										FieldPath: "status.podIP",
	// 									},
	// 								},
	// 							},
	// 						},
	// 						Name:            "supervisor",
	// 						Image:           "bestwoody/supervisor:1",
	// 						ImagePullPolicy: "Always",
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }
	// log.Println("create clonneSet")
	// _, err = kruiseClient.AppsV1alpha1().CloneSets(namespace).Create(context.TODO(), &cloneSet, metav1.CreateOptions{})
	// if err != nil {
	// 	panic(err.Error())
	// }
	cloneSetList, err := kruiseClient.AppsV1alpha1().CloneSets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	log.Printf("len of cloneSetList: %v \n", len(cloneSetList.Items))
	for _, cloneSet := range cloneSetList.Items {
		if cloneSet.Name != "test-cloneset" {
			continue
		}
		log.Printf("cloneSet: %v \n", cloneSet)

		newReplicas := new(int32)
		*newReplicas = 7
		// Modify object, such as replicas or template
		cloneSet.Spec.Replicas = newReplicas
		// cloneSet.Spec.cluster
		// cloneSet.Spec.ScaleStrategy.PodsToDelete = append(cloneSet.Spec.ScaleStrategy.PodsToDelete, "sample-data-44mqr", "sample-data-ntcvp")
		// Update
		// This might get conflict, should retry it
		ret, err := kruiseClient.AppsV1alpha1().CloneSets(namespace).Update(context.TODO(), &cloneSet, metav1.UpdateOptions{})
		if err != nil {
			panic(err.Error())
		} else {
			log.Printf("changed cloneSet: %v \n", ret)
		}

		ret, err = kruiseClient.AppsV1alpha1().CloneSets(namespace).Update(context.TODO(), &cloneSet, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("ERROR#1!!!!: %v \n", err.Error())
		} else {
			log.Printf("changed cloneSet: %v \n", ret)
		}
		log.Printf("ret cloneSet: %v \n", ret)
		ret, err = kruiseClient.AppsV1alpha1().CloneSets(namespace).Update(context.TODO(), ret, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("ERROR#2!!!!: %v \n", err.Error())
		} else {
			log.Printf("changed cloneSet: %v \n", ret)
		}

	}

}

// func main2() {
// 	// ctx := context.Background()
// 	// config := ctrl.GetConfigOrDie()

// 	config, err := outsideConfig()

// 	// // creates the in-cluster config
// 	// config, err := rest.InClusterConfig()

// 	if err != nil {
// 		panic(err.Error())
// 	}
// 	tsContainer := autoscale.NewTimeSeriesContainer(5)
// 	mclientset, err := metricsv.NewForConfig(config)
// 	as_meta := autoscale.NewAutoScaleMeta(config)
// 	as_meta.AddPod4Test("web-0")
// 	as_meta.AddPod4Test("web-1")
// 	as_meta.AddPod4Test("web-2")
// 	as_meta.AddPod4Test("hello-node-7c7c59b7cb-6bsjg")
// 	as_meta.SetupTenantWithDefaultArgs4Test("t1")
// 	as_meta.SetupTenantWithDefaultArgs4Test("t2")
// 	as_meta.UpdateTenant4Test("web-0", "t1")
// 	as_meta.UpdateTenant4Test("web-1", "t1")
// 	as_meta.UpdateTenant4Test("web-2", "t1")
// 	as_meta.UpdateTenant4Test("hello-node-7c7c59b7cb-6bsjg", "t2")
// 	// var lstTs int64
// 	lstTsMap := make(map[string]int64)
// 	hasNew := false
// 	for {
// 		podMetricsList, err := mclientset.MetricsV1beta1().PodMetricses("default").List(context.TODO(), metav1.ListOptions{})
// 		if err != nil {
// 			panic(err.Error())
// 		}

// 		for _, pod := range podMetricsList.Items {
// 			// if pod.Name == "web-0" {
// 			// 	log.Printf("podmetrics: %v \n", pod)
// 			// }
// 			lstTs, ok := lstTsMap[pod.Name]
// 			if !ok || pod.Timestamp.Unix() != lstTs {
// 				tsContainer.Insert(pod.Name, pod.Timestamp.Unix(),
// 					[]float64{
// 						pod.Containers[0].Usage.Cpu().AsApproximateFloat64(),
// 						pod.Containers[0].Usage.Memory().AsApproximateFloat64(),
// 					})
// 				lstTsMap[pod.Name] = pod.Timestamp.Unix()

// 				// if pod.Name == "web-0" {
// 				snapshot := tsContainer.GetSnapshotOfTimeSeries(pod.Name)
// 				// mint, maxt := cur_serires.GetMinMaxTime()
// 				hasNew = true
// 				log.Printf("%v mint,maxt: %v ~ %v\n", pod.Name, snapshot.MinTime, snapshot.MaxTime)
// 				log.Printf("%v statistics: cpu: %v %v mem: %v %v\n", pod.Name,
// 					snapshot.AvgOfCpu,
// 					snapshot.SampleCntOfCpu,
// 					snapshot.AvgOfMem,
// 					snapshot.SampleCntOfMem,
// 				)
// 				// }
// 			}

// 		}
// 		if hasNew {
// 			hasNew = false
// 			tArr := []string{"t1", "t2"}
// 			for _, tName := range tArr {
// 				stats1, _ := as_meta.ComputeStatisticsOfTenant(tName, tsContainer, "play")
// 				log.Printf("[Tenant]%v statistics: cpu: %v %v mem: %v %v\n", tName,
// 					stats1[0].Avg(),
// 					stats1[0].Cnt(),
// 					stats1[1].Avg(),
// 					stats1[1].Cnt(),
// 				)
// 			}
// 		}
// 		// v, ok := as_meta.PodDescMap[]
// 		// log.Printf("Podmetrics: %v \n", podMetricsList)
// 	}

// 	// creates the clientset
// 	clientset, err := kubernetes.NewForConfig(config)
// 	// clientset.AppsV1().
// 	if err != nil {
// 		panic(err.Error())
// 	}
// 	for {
// 		// get pods in all the namespaces by omitting namespace
// 		// Or specify namespace to get pods in particular namespace
// 		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
// 		if err != nil {
// 			panic(err.Error())
// 		}
// 		log.Printf("There are %d pods in the cluster\n", len(pods.Items))

// 		// Examples for error handling:
// 		// - Use helper functions e.g. errors.IsNotFound()
// 		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
// 		_, err = clientset.CoreV1().Pods("default").Get(context.TODO(), "example-xxxxx", metav1.GetOptions{})
// 		if errors.IsNotFound(err) {
// 			log.Printf("Pod example-xxxxx not found in default namespace\n")
// 		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
// 			log.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
// 		} else if err != nil {
// 			panic(err.Error())
// 		} else {
// 			log.Printf("Found example-xxxxx pod in default namespace\n")
// 		}

// 		time.Sleep(10 * time.Second)
// 	}
// }

func MockComputeStatisticsOfTenant(ts int, coresOfPod int) float64 {
	// ts := time.Now().Unix()
	// tsInMins := ts / 60
	return (math.Sin(float64(ts)/10.0) + 1) / 2 * float64(coresOfPod)
}

func SupClient(podIP string, tenantName string) {
	conn, err := grpc.Dial(podIP+":7000", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	if conn != nil {
		defer conn.Close()
	}
	c := supervisor.NewAssignClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	r, err := c.AssignTenant(ctx, &supervisor.AssignRequest{TenantID: tenantName})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.GetTenantID())
}

func main() {
	config, err := outsideConfig()
	if err != nil {
		panic(err.Error())
	}
	namespace := "tiflash-autoscale"
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "readnode"}}
	pods, err := cli.CoreV1().Pods(namespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		panic(err)
	}
	// resVer := pods.ListMeta.ResourceVersion
	for _, pod := range pods.Items {

		playLoadBytes := `
		{
			"metadata": {
				"labels": {
					"key1" : "value1.1",
					"key2" : "value2.2"
				}
			}
		}
		`
		_, err = cli.CoreV1().Pods("tiflash-autoscale").Patch(context.TODO(), pod.Name, k8stypes.StrategicMergePatchType, []byte(playLoadBytes), metav1.PatchOptions{})
		if err != nil {
			panic(err)
		}
		//!!!TEST END
		// c.AutoScaleMeta.UpdatePod(&pod)
	}

	// data, err := cli.CoreV1().RESTClient().Get().AbsPath("/api/v1/nodes/minikube/proxy/metrics/cadvisor").DoRaw(context.TODO())
	// reader := bytes.NewReader(data)
	// // fmt.Println(string(data))
	// var parser expfmt.TextParser
	// metricFamilies, err := parser.TextToMetricFamilies(reader)
	// for _, v := range metricFamilies {

	// 	fmt.Println("###MF_NAME: " + *v.Name)
	// 	if *v.Name != "container_cpu_usage_seconds_total" {
	// 		continue
	// 	}
	// 	metrics := v.GetMetric()
	// 	for _, metric := range metrics {

	// 		labels := metric.Label
	// 		// fmt.Printf("##LABELS_LEN %v\n", (len(labels)))
	// 		extraLabelName := "tenant"
	// 		extraLabelVal := "T1"
	// 		metric.Label = append(metric.Label, &dto.LabelPair{
	// 			Name:  &extraLabelName,
	// 			Value: &extraLabelVal,
	// 		})
	// 		for _, label := range labels {
	// 			if *label.Name == "pod" && *label.Value != "" {
	// 				fmt.Println("pod: " + *label.Value)
	// 			}

	// 			// fmt.Println("##LABEL " + *label.Name + ": " + *label.Value)
	// 		}
	// 		// fmt.Println(metric.Label)
	// 	}
	// 	// fmt.Println(k + ":" + v.String())

	// 	out := bytes.NewBuffer(make([]byte, 0, 10240))
	// 	expfmt.MetricFamilyToText(out, v)
	// 	fmt.Println(out.String())
	// }

	// OpenkruiseTest()
}
