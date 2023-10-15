# aws-cloud-day-2023-aws-ack-talk

## general idea
Small talk about how we can use aws ack to provision
ephemeral envs.

- use eks with IRSA (IAM Roles for ServiceAccounts)
- use ack obviously
- argo will make things much easier
- if possible throw in argo workflows + argo events
- if possible add some ticketing system to above

## Setup

### EKS
```bash
export PROJECT_ROOT=`pwd`
export AWS_REGION=us-east-1
alias tf=terraform
alias k=kubectl

cd $PROJECT_ROOT/terraform/eks
tf init
tf apply

# we will need these later
export CLUSTER_NAME=$(tf output cluster_name | tr -d \")
export ROLE_ARN=$(tf output irsa_role_arn | tr -d \")
export AWS_ID=$(tf output aws_account_id | tr -d \")
export OIDC_PROVIDER=$(tf output oidc_provider | tr -d \")

aws eks update-kubeconfig --name=$CLUSTER_NAME
```

### Argo
```bash
cd $PROJECT_ROOT/manifests/argocd/chart

helm dep up
helm install -n argocd --create-namespace ack-demo .

# give it few seconds after install
export ARGO_PASS=$(k -n argocd get secrets argocd-initial-admin-secret -o jsonpath={.data.password} | base64 -d)
k -n argocd port-forward svc/ack-demo-argocd-server 8443:443 2>&1 >/dev/null &

argocd login localhost:8443 --username admin --password $ARGO_PASS
```

### ACK
Replace the `SERVICE` variable with desired service and repeat the process for additional controllers (`s3`, `rds`, ...).
Demo app will need `sqs` and `iam`.
```bash
export SERVICE=sqs
export RELEASE_VERSION=$(curl -sL https://api.github.com/repos/aws-controllers-k8s/${SERVICE}-controller/releases/latest | jq -r '.tag_name | ltrimstr("v")')
export ACK_SYSTEM_NAMESPACE=ack-system

aws ecr-public get-login-password --region us-east-1 | helm registry login --username AWS --password-stdin public.ecr.aws

helm install --create-namespace -n $ACK_SYSTEM_NAMESPACE ack-$SERVICE-controller \
  oci://public.ecr.aws/aws-controllers-k8s/$SERVICE-chart --version=$RELEASE_VERSION \
  --set=aws.region=$AWS_REGION --set-string=serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=$ROLE_ARN
```

### Deploy app

#### Customize the app (Optional)
Optionally you can customize the app and push it to your desired registry
```bash
cd $PROJECT_ROOT/app
export REGISTRY=docker.io
export ORG=yourusername
export REPO=ack-demo

podman build -f ../manifests/containers/Dockerfile-sqs-demo -t $REGISTRY/$ORG/$REPO .
podman push 
```
Do not forget to update the deployment manifest at `manifests/apps/sqs-demo-app/deployment.yaml`

#### Add role arn to app ServiceAccount

This part is bit 'hacky', since neither helm nor kubectl have builtin ways of feeding output status field of one resource to another. We will be using our knowledge of arn structure and fill out the role name manully. Will leave this as is for now until I have better solution (nothing is as permanent as temporary suddenly comes to mind for some reason.....).
TODO:
- this can be stuffed in a workflow
- alternatively there might be some way of taking the extracting this info with k8s/aws api (still a custom solution)

```bash
cd $PROJECT_ROOT/manifests/apps/sqs-demo-app

cat <<EOF > sa.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ack-demo
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::${AWS_ID}:role/ack-demo
EOF

cat <<EOF > iam.yaml
apiVersion: iam.services.k8s.aws/v1alpha1
kind: Role
metadata:
  name: ack-demo
spec:
  name: ack-demo
  assumeRolePolicyDocument: |
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Federated": "arn:aws:iam::${AWS_ID}:oidc-provider/${OIDC_PROVIDER}"
                },
                "Action": "sts:AssumeRoleWithWebIdentity",
                "Condition": {
                    "StringEquals": {
                        "${OIDC_PROVIDER}:sub": [
                            "system:serviceaccount:sqs-demo-app:ack-demo"
                        ]
                    }
                }
            }
        ]
    }
  policies:
  - "arn:aws:iam::aws:policy/AmazonSQSFullAccess"
EOF

```
#### Deploy
```bash
cd $PROJECT_ROOT/manifests/argocd/apps
k apply -f .

argocd app list
```

### Verify
```bash
aws sqs list-queues
```

