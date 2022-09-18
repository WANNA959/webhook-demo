# webhook-demo
- ref
    - https://github.com/scriptwang/admission-webhook-example/tree/master/v1
    - https://github.com/kubernetes/kubernetes/tree/v1.13.0/test/images

## 流程

```shell
# 1. 创建csr.conf，创建key+cert
# 2. 创建CertificateSigningRequest
# 3. apiserver approve csr
# 4. apiserver创建secret(webhook-demo-secret)
sh scripts/webhook_create_apiserver_certs.sh 

# 存储key-cert到本地
kubectl get secret webhook-demo-certs -n webhook-demo -o json | jq -r '.data."key.pem"' | base64 -d > /etc/webhook/certs/key.pem
kubectl get secret webhook-demo-certs -n webhook-demo -o json | jq -r '.data."cert.pem"' | base64 -d > /etc/webhook/certs/cert.pem
```

- 实现webhook server逻辑

```shell
# build webhook image & push to hub
sh scripts/build_image.sh 
```

- 部署service、deployment（webhook server）

```shell
# service是为了集群内的pod可以通过service https访问到wenhook server
kubectl apply -f resources/service.yaml 

# deployment是部署webhook server的控制器
kubectl apply -f resources/deployment.yaml 
```

- 部署mutating/validating配置

```shell
# CA_BUNDLE
CA_BUNDLE=$(kubectl config view --raw --flatten -o json | jq -r '.clusters[] | .cluster."certificate-authority-data"')
# 部署mutating配置
# 注意在配置中watch pod，但是没有处理，那么deployment的pod一直无法被创建
kubectl apply -f resources/mutating-admission-webhook.yaml 

# 部署validating配置
kubectl apply -f resources/validating-admission-webhook.yaml 

# 查看/删除wenhook configuration
kubectl get mutatingwebhookconfiguration
kubectl get validatingwebhookconfiguration
kubectl delete mutatingwebhookconfiguration mutating-webhook-demo-cfg
```

- 重新部署webhook

```shell
# bug
kubectl delete pod $(kubectl get pod -A | grep nginx | awk '{print $2}') -n webhook-demo

# delete & rebuild & re-deploy
sh scripts/rebuild_webhook.sh 
```

- 测试

```shell
# 没有mutate，只有validate，但是可以创建，因为annotation：webhook-demo.gox.com/validate: "false"使得无需validate
kubectl apply -f test/resources/nginx-deployment-without-label.yaml 

# 没有mutate，只有validate，但是可以创建，因为创建的deployment本身包含了需要validate的label
kubectl apply -f test/resources/nginx-deployment-label.yaml 

# 没有mutate，只有validate，不能创建，没有validate检查的label
# 有mutate，可以创建
kubectl apply -f test/resources/nginx-deployment.yaml 
```

## webhook逻辑

#### mutating webhook

- 检查范围
  - namespace为webhook-demo
  - 且不存在annotation："webhook-demo.gox.com/mutate"="n", "no", "false", "off"
  - 且不存在annotation："webhook-demo.gox.com/status"="mutated"
    - 表示已经mutate过

- /mutate/add-label：采用patch的方式
  - annotatin
    - 无则add
    - 有则update
  - label
    - 无则add
    - 有，只update name

#### validating webhook

- 检查范围

  - namespace为webhook-demo

  - 且不存在annotation："webhook-demo.gox.com/validate"="n", "no", "false", "off"

### debug

https://github.com/scriptwang/admission-webhook-example/tree/master/v1

- 简单调试：在k8s集群外部跑webhook server