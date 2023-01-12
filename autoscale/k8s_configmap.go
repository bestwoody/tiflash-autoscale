package autoscale

// func (c *AutoScaleMeta) initConfigMap() {
// 	configMapName := "readnode-pod-state"
// 	var err error
// 	c.configMap, err = c.k8sCli.CoreV1().ConfigMaps("tiflash-autoscale").Get(context.TODO(), configMapName, metav1.GetOptions{})
// 	if err != nil {
// 		c.configMap = &v1.ConfigMap{
// 			TypeMeta: metav1.TypeMeta{
// 				Kind:       "ConfigMap",
// 				APIVersion: "v1",
// 			},
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: configMapName,
// 			},
// 			Data: map[string]string{},
// 		}
// 		// get pods in all the namespaces by omitting namespace
// 		// Or specify namespace to get pods in particular namespace
// 		c.configMap, err = c.k8sCli.CoreV1().ConfigMaps("tiflash-autoscale").Create(context.TODO(), c.configMap, metav1.CreateOptions{})
// 		if err != nil {
// 			panic(err.Error())
// 		}
// 	}
// 	Logger.Infof("loadConfigMap %v", c.configMap.String())
// }

// func (c *AutoScaleMeta) GetRnPodStateAndTenantFromCM(podname string) (int, string) {
// 	c.cmMutex.Lock()
// 	defer c.cmMutex.Unlock()
// 	stateStr, ok := c.configMap.Data[podname]
// 	if !ok {
// 		return CmRnPodStateUnknown, ""
// 	} else {
// 		return ConfigMapPodState(stateStr)
// 	}
// }

// func ConfigMapPodStateStr(state int, optTenant string) string {
// 	var value string
// 	switch state {
// 	case CmRnPodStateUnassigned:
// 		value = "unassigned"
// 	case CmRnPodStateUnassigning:
// 		value = "unassigning"
// 	case CmRnPodStateAssigned:
// 		value = "assigned|" + optTenant
// 	case CmRnPodStateAssigning:
// 		value = "assigning"
// 	}
// 	return value
// }

// func ConfigMapPodState(str string) (int, string) {
// 	switch str {
// 	case "unassigned":
// 		return CmRnPodStateUnassigned, ""
// 	case "unassigning":
// 		return CmRnPodStateUnassigning, ""
// 	case "assigning":
// 		return CmRnPodStateAssigning, ""
// 	default:
// 		if !strings.HasPrefix(str, "assigned|") {
// 			return -1, ""
// 		}
// 		i := strings.Index(str, "|")
// 		tenant := ""
// 		if i > -1 {
// 			tenant = str[i+1:]
// 		}
// 		return CmRnPodStateAssigned, tenant
// 	}

// }

// func (c *AutoScaleMeta) handleK8sConfigMapsApiError(err error, caller string) {
// 	configMapName := "readnode-pod-state"
// 	errStr := err.Error()
// 	Logger.Errorf("[error][%v]K8sConfigMapsApiError, err: %+v", caller, errStr)
// 	if strings.Contains(errStr, "please apply your changes to the latest version") {
// 		retConfigMap, err := c.k8sCli.CoreV1().ConfigMaps("tiflash-autoscale").Get(context.TODO(), configMapName, metav1.GetOptions{})
// 		if err != nil {
// 			Logger.Errorf("[error][%v]K8sConfigMapsApiError, failed to get latest version of configmap, err: %+v", caller, err.Error())
// 		} else {
// 			c.configMap = retConfigMap
// 		}
// 	}
// }

// func (c *AutoScaleMeta) setConfigMapState(podName string, state int, optTenant string) error {
// 	c.cmMutex.Lock()
// 	defer c.cmMutex.Unlock()
// 	var value string
// 	switch state {
// 	case CmRnPodStateUnassigned:
// 		value = "unassigned"
// 	case CmRnPodStateUnassigning:
// 		value = "unassigning"
// 	case CmRnPodStateAssigned:
// 		value = "assigned|" + optTenant
// 	case CmRnPodStateAssigning:
// 		value = "assigning"
// 	}
// 	// TODO  c.configMap.DeepCopy()
// 	// TODO err handlling
// 	if c.configMap.Data == nil {
// 		c.configMap.Data = make(map[string]string)
// 	}
// 	c.configMap.Data[podName] = value
// 	retConfigMap, err := c.k8sCli.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), c.configMap, metav1.UpdateOptions{})
// 	if err != nil {
// 		c.handleK8sConfigMapsApiError(err, "AutoScaleMeta::setConfigMapState")
// 		return err
// 	}
// 	c.configMap = retConfigMap
// 	return nil
// }

// func (c *AutoScaleMeta) setConfigMapStateBatch(kvMap map[string]string) error {
// 	c.cmMutex.Lock()
// 	defer c.cmMutex.Unlock()
// 	// TODO  c.configMap.DeepCopy()
// 	// TODO err handlling
// 	if c.configMap.Data == nil {
// 		c.configMap.Data = make(map[string]string)
// 	}
// 	for k, v := range kvMap {
// 		c.configMap.Data[k] = v
// 	}

// 	retConfigMap, err := c.k8sCli.CoreV1().ConfigMaps("tiflash-autoscale").Update(context.TODO(), c.configMap, metav1.UpdateOptions{})
// 	if err != nil {
// 		c.handleK8sConfigMapsApiError(err, "AutoScaleMeta::setConfigMapState")
// 		return err
// 	}
// 	Logger.Infof("[AutoScaleMeta]current configmap: %+v", retConfigMap.Data)
// 	c.configMap = retConfigMap
// 	return nil
// }
