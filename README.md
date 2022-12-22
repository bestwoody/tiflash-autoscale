# TiFlash Auto Scale
## 注意事项
TiDB 版本需要存算分离的版本，部署方法： https://docs.google.com/document/d/11tNIvfdHWTzID3KuZ1lXHii7a1B6d6_XOgB7riFoNK0/edit

（binary可能已经过时，最新binary和部署方法需要咨询 @郭江涛 老师）

## 预装环境
```shell
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

helm install kruise openkruise/kruise --version 1.3.0
```

## Create Namespace
```shell
kubectl create namespace tiflash-autoscale

kubectl create clusterrolebinding add-on-cluster-admin --clusterrole=cluster-admin  --serviceaccount=tiflash-autoscale:default
```

## Node label (注意：需要事先打上)
### Autoscaler所在的node label 
建议的node规格: "t3.xlarge"，固定数目1个。
```shell
"tiflash.used-for-autoscale": "true"
```
### Compute Pod所在的node label 
建议的node规格: "m6a.2xlarge", node group需要支持auto scale，且一个Compute Pod对应一个node。
```shell
"tiflash.used-for-compute": "true"
```

## ENV 配置
配置aws region： 用于sns
```yaml
    - name: TIFLASH_AS_ENABLE_SNS # whether to enable sns: default true, if enabled, TIFLASH_AS_REGION should be set
        value: "true"
    - name: TIFLASH_AS_REGION
        value: "us-east-2"
```

## 修改TiDB Cluster配置
修改autoscale.yaml如下字段到正确的配置
```yaml
    - name: PD_ADDR
        value: "172.31.8.1:2379"
    - name: TIDB_STATUS_ADDR
        value: "172.31.7.1:4000"
```
因为当前没对接管控交互逻辑，所以当前版本是通过环境变量配置，且当前只能配一个TiDb Cluster的。

接下来会和管控讨论如何对接且支持多Cluster。

## Apply Autoscale.yaml

```shell
kubectl apply -f autoscale.yaml 
```

## Check 
```shell
kubectl describe pod autoscale-0 -n tiflash-autoscale

kubectl logs -f autoscale-0 -n tiflash-autoscale
```

### DELETE!!
```shell
kubectl delete -f autoscale.yaml && kubectl delete cloneset -n tiflash-autoscale readnode && kubectl delete configmap -n tiflash-autoscale readnode-pod-state
```

### RESET!!!!!!!!
```shell
kubectl delete -f autoscale.yaml && kubectl delete cloneset -n tiflash-autoscale readnode && kubectl delete configmap -n tiflash-autoscale readnode-pod-state && sleep 30 && kubectl apply -f autoscale.yaml 
```
