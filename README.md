# cloudformation-operator

A custom resource definition for CloudFormation stacks and a Kubernetes operator for managing them.

## Key Features

1) Create, manage & lifecycle CloudFormation stacks via Kubernetes
2) Create, manage & lifecycle any AWS resources supported via CloudFormation from Kubernetes
3) Template Stack outputs available in-cluster for consumption of values and endpoints in other applications
4) Template able to be provided as inline YAML or via separate URL

## Demo

The following demonstration assumes a deployed & working operator. Please see the sections further below how that can be achieved.

### Create stack

Currently you don't have any stacks.

```console
$ kubectl get stacks
No resources found.
```

#### Template

Let's create a simple one that manages an S3 bucket:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-bucket
spec:
  template: |
    ---
    AWSTemplateFormatVersion: '2010-09-09'

    Resources:
      S3Bucket:
        Type: AWS::S3::Bucket
        Properties:
          VersioningConfiguration:
            Status: Suspended
```

The Stack resource's definition looks a lot like any other Kubernetes resource manifest.
The `spec` section describes an attribute called `template` which contains a regular CloudFormation template.

Go ahead and submit the stack definition to your cluster:

```console
$ kubectl apply -f config/samples/s3-bucket.yaml
stack "my-bucket" created
$ kubectl get stacks
NAME        AGE
my-bucket   21s
```

Open your AWS CloudFormation console and find your new stack.

![Create stack](docs/img/stack-create.png)

Once the CloudFormation stack is created check that your S3 bucket was created as well.

The operator will write back additional information about the CloudFormation Stack to your Kubernetes resource's `status` section, e.g. the `stackID`:

```console
$ kubectl get stacks my-bucket -o yaml
spec:
  template:
  ...
status:
  stackID: arn:aws:cloudformation:eu-central-1:123456789012:stack/my-bucket/327b7d3c-f27b-4b94-8d17-92a1d9da85ab
```

Voilà, you just created a CloudFormation stack by only talking to Kubernetes.

### Update stack

You can also update your stack resources: Let's change the `VersioningConfiguration` from `Suspended` to `Enabled`:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-bucket
spec:
  template: |
    ---
    AWSTemplateFormatVersion: '2010-09-09'

    Resources:
      S3Bucket:
        Type: AWS::S3::Bucket
        Properties:
          VersioningConfiguration:
            Status: Enabled
```

As with most Kubernetes resources you can update your `Stack` resource by applying a changed manifest to your Kubernetes 
cluster or by using `kubectl edit stack my-stack`.

```console
$ kubectl apply -f config/samples/s3-bucket.yaml
stack "my-bucket" configured
```

Wait until the operator discovered and executed the change, then look at your AWS CloudFormation console again and find 
your stack being updated, yay.

![Update stack](docs/img/stack-update.png)

### Parameters

However, often you'll want to extract dynamic values out of your CloudFormation stack template into so called `Parameters` 
so that your template itself doesn't change that often and, well, is really a *template*.

Let's extract the `VersioningConfiguration` into a parameter:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-bucket
spec:
  parameters:
    VersioningConfiguration: Enabled
  template: |
    ---
    AWSTemplateFormatVersion: '2010-09-09'

    Parameters:
      VersioningConfiguration:
        Type: String
        Default: none
        AllowedValues:
        - "Enabled"
        - "Suspended"

    Resources:
      S3Bucket:
        Type: AWS::S3::Bucket
        Properties:
          VersioningConfiguration:
            Status:
              Ref: VersioningConfiguration
```

and apply it to your cluster:

```console
$ kubectl apply -f config/samples/s3-bucket.yaml
stack "my-bucket" configured
```

Since we changed the template a little this will update your CloudFormation stack. 
However, since we didn't actually change anything because we injected the same `VersioningConfiguration` value as before, 
your S3 bucket shouldn't change.

Any CloudFormation parameters defined in the CloudFormation template can be specified in the `Stack` resource's 
`spec.parameters` section. 
It's a simple key/value map.

### Outputs

Furthermore, CloudFormation supports `Outputs`. 
These can be used for dynamic values that are only known after a stack has been created.
In our example, we don't define a particular S3 bucket name but instead let AWS generate one for us.

Let's change our CloudFormation template to expose the generated bucket name via an `Output`:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-bucket
spec:
  parameters:
    VersioningConfiguration: Enabled
  template: |
    ---
    AWSTemplateFormatVersion: '2010-09-09'

    Parameters:
      VersioningConfiguration:
        Type: String
        Default: none
        AllowedValues:
        - "Enabled"
        - "Suspended"

    Resources:
      S3Bucket:
        Type: AWS::S3::Bucket
        Properties:
          VersioningConfiguration:
            Status:
              Ref: VersioningConfiguration

    Outputs:
      BucketName:
        Value: !Ref 'S3Bucket'
        Description: Name of the sample Amazon S3 bucket.
```

Apply the change to our cluster and wait until the operator has successfully updated the CloudFormation stack.

```console
$ kubectl apply -f config/samples/s3-bucket.yaml
stack "my-bucket" configured
```

Every `Output` you define will be available in your Kubernetes resource's `status` section under the `outputs` field as 
a key/value map.

Let's check the name of our S3 bucket:

```console
$ kubectl get stacks my-bucket -o yaml
spec:
  template:
  ...
status:
  stackID: ...
  outputs:
    BucketName: my-bucket-s3bucket-tarusnslfnsj
```

In the template we defined an `Output` called `BucketName` that should contain the name of our bucket after stack creation. 
Looking up the corresponding value under `.status.outputs[BucketName]` reveals that our bucket was named 
`my-bucket-s3bucket-tarusnslfnsj`.

#### Automatic Output ConfigMap

Furthermore, outputs are written to a `ConfigMap` usable as environment variables, projected volumes and other types of usages within kubernetes.
The name will be of the form `{StackName}-cm`, see the example below:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: my-bucket-cm
  ownerReferences:
    - apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
      kind: Stack
      name: my-bucket
      uid: f204b58e-09fd-49df-9a65-3d740271d2eb
      controller: true
      blockOwnerDeletion: true
data:
  BucketName: my-bucket-7980b414-s3bucket-o1n9pv47imzx
```

Existing ConfigMaps with an ownerReference will be ignored

### Delete stack

The operator captures the whole lifecycle of a CloudFormation stack. 
So if you delete the resource from Kubernetes, the operator will tear down the CloudFormation stack as well. 
Let's do that now:

```console
$ kubectl delete stack my-bucket
stack "my-bucket" deleted
```

Check your CloudFormation console once more and validate that your stack as well as your S3 bucket were deleted.

![Delete stack](docs/img/stack-delete.png)

## Stack Features

There are several additional capabilities of a Stack resource not included in the demo above.

### Tags

You may want to assign tags to your CloudFormation stacks.
The tags added to a CloudFormation stack will be propagated to the managed resources.
This feature may be useful in multiple cases, for example, to distinguish resources at billing report.
Current operator provides two ways to assign tags:
- `tags` parameter on kubernetes `Config` resource spec
- `tags` parameter on kubernetes `Stack` resource spec

#### Stack Resource

Resource-specific tags have precedence over the default tags in the `Config` object.
Thus if a tag is defined at command-line arguments and for a `Stack` resource, the value from the `Stack` resource will
be used.

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-bucket
spec:
  tags:
    foo: dataFromStack
  template: |
    ---
    AWSTemplateFormatVersion: '2010-09-09'

    Resources:
      S3Bucket:
        Type: AWS::S3::Bucket
        Properties:
          VersioningConfiguration:
            Status: Enabled
```

#### Config

### Controller Config

This method of detecting/configuring can be used as a fallback to ensure a default value for all stacks is applied
or to standardize a particular value cluster-wide.

```yaml
apiVersion: services.k8s.aws.cuppett.dev/v1alpha1
kind: Config
metadata:
  name: default
  namespace: aws-cloudformation-controller-system
spec:
  tags: 
    bu: marketing
    cluster: prod1
```

> NOTE: The name of `Config` must be `default` and in the namespace of the pod.


If we run the operation and a `Stack` resource with the described above examples, we'll see such picture:

![Stack tags](docs/img/stack-tags.png)


### Template URL

If your template exceeds maximum size of `51200` bytes, you can instead upload it to S3 or a standard web server, and set its URL in `templateUrl`:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-stack
spec:
  templateUrl: 'https://my-bucket-name.s3.amazonaws.com/template_file.json'
```

> NOTE: Put URL in quotes to avoid templating issues

> NOTE: The template URL will only be re-read by CloudFormation on operator restarts, periodically (hours), and when other updates to the Stack resource are made.

### Role ARN

For indirect ownership of the operator to stack resources (described further down below), you can specify the role to be used for
creating or updating resources.

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-stack
spec:
  roleArn: 'arn:aws:iam::123456789000:role/cf-resources-allowed'
```

### Notification ARNs

You can receive signals via SNS for stack changes using the CloudFormation built-in notification mechanisms.
Register one or more SNS topics with your stack like this:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-stack
spec:
  notificationArns: 
  - 'arn:aws:sns:us-east-2:641875867446:alert-admin'
  - 'arn:aws:sns:us-east-2:641875867446:lambda-processor'
```
> NOTE: Found an issue with the Go SDK, following up here: https://github.com/aws/aws-sdk-go-v2/issues/1423
> 
> Workaround (to remove on Update):
> 1. Save Stack resource without NotificationARNs (or empty YAML)
> 2. Remove manually directly in AWS after successful save/update


### Capabilities

CloudFormation stacks have an ability to render based on known macros and create additional IAM principals.
To ensure this is desired/permitted when creating or updating stacks, it requires specifying the corresponding capability.
You indicate this per-stack by providing one or more of the expected capabilities and indicating this is intended.
The example below shows all the possible capabilities and those permitted by the controller. 
Please refer to the documentation for the descriptions and when to use each.

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-stack
spec:
  capabilities: 
  - CAPABILITY_IAM
  - CAPABILITY_NAMED_IAM
  - CAPABILITY_AUTO_EXPAND
```


### Create options

#### onFailure

To change stack behavior on creation use `onFailure` that suports `DELETE`, `DO_NOTHING`, and `ROLLBACK` options:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-stack
spec:
  onFailure: DELETE
  template: |
    ...
```

#### stackName

To set the stack name on creation use `stackName`:

```yaml
apiVersion: cloudformation.services.k8s.aws.cuppett.dev/v1alpha1
kind: Stack
metadata:
  name: my-stack
spec:
  stackName: well-hello-there  
  template: |
    ...
```

## Deploying to a Cluster

You need API access to a cluster running at least Kubernetes v1.19+ (OpenShift 4.6+).

### Build and publish the docker image

Use this step for building a private copy of the operator

```console
$ make docker-build docker-push IMG=quay.io/cuppett/cloudformation-operator:latest
```

### Deployment

#### Cert-Manager

This operator uses webhooks. It will require either deploying cert-manager or arranging the manifests or environment
in such a way the certificates are available as described in the kube-builder documenation.

See Also: https://book.kubebuilder.io/cronjob-tutorial/cert-manager.html

#### Permissions

The operator will require an IAM role or user credentials.
You need to make sure that the operator Pod has enough AWS IAM permissions to create, update and delete
CloudFormation stacks as well as permission to modify any resources that are part of the CloudFormation stacks you
intend to manage.

The following use cases for the operator are possible via the IAM features provided:

1) Direct ownership: The operator works against CloudFormation and management of the resources within using the credentials provided.
2) Indirect ownership: The operator works against CloudFormation, but management of the resources is done via the RoleARN provided in the Stack resource spec.roleArn
3) Assumed identity: The --assume-role command line argument is provided. The operator assumes the role using the credentials available and then that role is used in either the #1 and #2 mode above.

##### Minimal Policy for Indirect Ownership

Assuming no resources are to be modified by the operator directly, here is the minimal IAM policy required to allow the operator to function:

```yaml
Version: '2012-10-17'
Statement:
    Resource: "*"
  - Sid: CreateRead
    Effect: Allow
    Action:
      - cloudformation:CreateStack
      - cloudformation:DescribeStackInstance
      - cloudformation:DescribeStackResource
      - cloudformation:DescribeStacks
      - cloudformation:ListStackResources
    Resource: "*"
  - Sid: UpdateDelete
    Effect: Allow
    Action:
      - cloudformation:DeleteStack
      - cloudformation:UpdateStack
    Resource: "*"
    Condition:
      StringEquals:
        aws:ResourceTag/kubernetes.io/controlled-by: cloudformation.services.k8s.aws.cuppett.dev/operator
```

> NOTE: For direct ownership you will need to add additional policies for any resources the operator is intended to manipulate (e.g. S3, RDS, SQS).
> You can achieve this by associating the AWS managed policies (e.g. arn:aws:iam::aws:policy/AmazonRDSFullAccess) or by crafting
> and attaching your own. For individual services, refer to the AWS documentation those permissions required by CloudFormation to 
> lifecycle those resources.

##### Allowing RoleARN with New or Existing Stacks

For indirect ownership, you will provide the Spec.roleArn attribute on every stack in the cluster. The role provided in the Stack
resource must be able to have credentials "Passed" by CloudFormation.

See also: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-iam-servicerole.html

You can achieve this by adding a policy similar to the following to the operator's principal role:

```yaml
Version: '2012-10-17'
Statement:
- Sid: PassRole
  Effect: Allow
  Action: iam:PassRole
  Resource: arn:aws:iam::123456789000:role/cf-resources-allowed
```
The effective operator role will need this for any/all roles being used/referenced by the Stack resources managed by this operator.

In addition, the spec.roleArn must have a trust relationship with the CloudFormation service as follows:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "",
      "Effect": "Allow",
      "Principal": {
        "Service": "cloudformation.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

#### Providing credentials

The operator will use the credentials discovered by the SDK and the default credential provider chain.
If you're using [EKS OIDC](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) or similar
method and give your Pod a dedicated IAM role then you have to add the permissions to that role.

To set credentials explicitly, you can use the scaffolded in AWS environment variables in the SDK kustomize manifests:

```console
$ export AWS_ACCESS_KEY_ID=XXXXX
$ export AWS_SECRET_ACCESS_KEY=XXXXX
$ export AWS_REGION=XXXXX
```

##### Mint Mode Fallback on OpenShift

For OpenShift clusters configured in [mint mode](https://docs.openshift.com/container-platform/4.9/authentication/managing_cloud_provider_credentials/cco-mode-mint.html)
the controller will attempt to discover credentials using the SDK, but then fallback to creating a `CredentialsRequest` with the minimal
permissions identified from above.
The resulting secret will be used.

#### Using kustomize & make to deploy

Deploy and start the CloudFormation operator in your cluster by using the provided manifests and Makefile:

```console
$ make deploy IMG=quay.io/cuppett/aws-cloudformation-operator:latest
/home/scuppett/go/src/github.com/cuppett/aws-cloudformation-operator/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && /home/scuppett/go/src/github.com/cuppett/aws-cloudformation-operator/bin/kustomize edit set image controller=quay.io/cuppett/aws-cloudformation-operator:latest
/home/scuppett/go/src/github.com/cuppett/aws-cloudformation-operator/bin/kustomize build config/default | kubectl apply -f -
namespace/aws-cloudformation-operator-system created
customresourcedefinition.apiextensions.k8s.io/configs.services.k8s.aws.cuppett.dev configured
customresourcedefinition.apiextensions.k8s.io/stacks.cloudformation.services.k8s.aws.cuppett.dev configured
serviceaccount/aws-cloudformation-operator-controller-manager created
role.rbac.authorization.k8s.io/aws-cloudformation-operator-leader-election-role created
clusterrole.rbac.authorization.k8s.io/aws-cloudformation-operator-manager-role created
clusterrole.rbac.authorization.k8s.io/aws-cloudformation-operator-metrics-reader created
clusterrole.rbac.authorization.k8s.io/aws-cloudformation-operator-proxy-role created
rolebinding.rbac.authorization.k8s.io/aws-cloudformation-operator-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/aws-cloudformation-operator-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/aws-cloudformation-operator-proxy-rolebinding created
configmap/aws-cloudformation-operator-manager-config created
secret/aws-cloudformation-operator-aws-keys-9t2g4d4dg2 created
secret/aws-cloudformation-operator-controller-flags-95h8d5m65t created
service/aws-cloudformation-operator-manager-metrics-service created
service/aws-cloudformation-operator-webhook-service created
deployment.apps/aws-cloudformation-operator-controller-manager created
certificate.cert-manager.io/aws-cloudformation-operator-serving-cert created
issuer.cert-manager.io/aws-cloudformation-operator-selfsigned-issuer created
servicemonitor.monitoring.coreos.com/aws-cloudformation-operator-controller-manager-metrics-monitor created
validatingwebhookconfiguration.admissionregistration.k8s.io/aws-cloudformation-operator-validating-webhook-configuration created
```

#### Monitoring

Once running the operator should print some output but shouldn't actually do anything at this point.
Leave it running & keep watching its logs as you work with Stack resources within your cluster.

```console
$ kubectl get pods -n aws-cloudformation-operator-system
NAME          READY   STATUS      RESTARTS   AGE
[POD_NAME]    2/2     Running     0          1m

$ kubectl logs -n aws-cloudformation-operator-system [POD_NAME] manager
I0129 12:32:22.918001       1 request.go:665] Waited for 1.027928187s due to client-side throttling, not priority and fairness, request: GET:https://172.30.0.1:443/apis/apps.openshift.io/v1?timeout=32s
2022-01-29T12:32:25.073Z	INFO	controller-runtime.metrics	Metrics server is starting to listen	{"addr": "127.0.0.1:8080"}
2022-01-29T12:32:25.074Z	INFO	controller-runtime.builder	skip registering a mutating webhook, object does not implement admission.Defaulter or WithDefaulter wasn't called	{"GVK": "cloudformation.services.k8s.aws.cuppett.dev/v1alpha1, Kind=Stack"}
2022-01-29T12:32:25.074Z	INFO	controller-runtime.builder	Registering a validating webhook	{"GVK": "cloudformation.services.k8s.aws.cuppett.dev/v1alpha1, Kind=Stack", "path": "/validate-cloudformation-services-k8s-aws-cuppett-dev-v1alpha1-stack"}
2022-01-29T12:32:25.074Z	INFO	controller-runtime.webhook	Registering webhook	{"path": "/validate-cloudformation-services-k8s-aws-cuppett-dev-v1alpha1-stack"}
2022-01-29T12:32:25.074Z	INFO	setup	starting manager
I0129 12:32:25.075004       1 leaderelection.go:248] attempting to acquire leader lease aws-cloudformation-operator-system/3680e595.cuppett.dev...
2022-01-29T12:32:25.075Z	INFO	Starting metrics server	{"path": "/metrics"}
2022-01-29T12:32:25.075Z	INFO	controller-runtime.webhook.webhooks	Starting webhook server
2022-01-29T12:32:25.075Z	INFO	controller-runtime.certwatcher	Updated current TLS certificate
2022-01-29T12:32:25.075Z	INFO	controller-runtime.webhook	Serving webhook server	{"host": "", "port": 9443}
2022-01-29T12:32:25.075Z	INFO	controller-runtime.certwatcher	Starting certificate watcher
I0129 12:32:25.095053       1 leaderelection.go:258] successfully acquired lease aws-cloudformation-operator-system/3680e595.cuppett.dev
2022-01-29T12:32:25.095Z	INFO	controller.stack	Starting EventSource	{"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack", "source": "kind source: *v1alpha1.Stack"}
2022-01-29T12:32:25.095Z	INFO	controller.stack	Starting EventSource	{"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack", "source": "kind source: *v1.ConfigMap"}
2022-01-29T12:32:25.095Z	INFO	controller.stack	Starting Controller	{"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack"}
2022-01-29T12:32:25.095Z	DEBUG	events	Normal	{"object": {"kind":"ConfigMap","namespace":"aws-cloudformation-operator-system","name":"3680e595.cuppett.dev","uid":"f205af01-f5ec-4b11-9cc6-4badffc9d765","apiVersion":"v1","resourceVersion":"109209856"}, "reason": "LeaderElection", "message": "aws-cloudformation-operator-controller-manager-756b7b7789-5sntq_a12cf4ef-04a9-4b15-837d-e0fcbbdd1bb3 became leader"}
2022-01-29T12:32:25.095Z	INFO	controller.config	Starting EventSource	{"reconciler group": "services.k8s.aws.cuppett.dev", "reconciler kind": "Config", "source": "kind source: *v1alpha1.Config"}
2022-01-29T12:32:25.095Z	INFO	controller.config	Starting Controller	{"reconciler group": "services.k8s.aws.cuppett.dev", "reconciler kind": "Config"}
2022-01-29T12:32:25.095Z	DEBUG	events	Normal	{"object": {"kind":"Lease","namespace":"aws-cloudformation-operator-system","name":"3680e595.cuppett.dev","uid":"1249bb6f-e855-4ec8-b40c-4664f9a7fb3d","apiVersion":"coordination.k8s.io/v1","resourceVersion":"109209857"}, "reason": "LeaderElection", "message": "aws-cloudformation-operator-controller-manager-756b7b7789-5sntq_a12cf4ef-04a9-4b15-837d-e0fcbbdd1bb3 became leader"}
2022-01-29T12:32:25.897Z	INFO	controller.stack	Starting workers	{"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack", "worker count": 1}
2022-01-29T12:32:25.897Z	INFO	controller.config	Starting workers	{"reconciler group": "services.k8s.aws.cuppett.dev", "reconciler kind": "Config", "worker count": 1}
2022-01-29T12:32:25.897Z	INFO	workers.Config	Region resolved	{"region": "us-east-1"}
```

### Cleanup

Clean up the resources:

```console
$ make undeploy
```

## Build/run locally

This project uses the [operator sdk](https://github.com/operator-framework/operator-sdk).

(Assuming you have already configured your KUBECONFIG or other means)

```console
$ make run
/home/scuppett/go/src/github.com/cuppett/aws-cloudformation-operator/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
/home/scuppett/go/src/github.com/cuppett/aws-cloudformation-operator/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
go run ./main.go --no-webhook
I0129 07:22:46.246507   77287 request.go:665] Waited for 1.049375903s due to client-side throttling, not priority and fairness, request: GET:https://api.prod.openshift.cuppett.dev:6443/apis/machineconfiguration.openshift.io/v1?timeout=32s
2022-01-29T07:22:48.340-0500    INFO    controller-runtime.metrics      Metrics server is starting to listen    {"addr": ":8080"}
2022-01-29T07:22:48.341-0500    INFO    setup   starting manager
2022-01-29T07:22:48.341-0500    INFO    Starting metrics server {"path": "/metrics"}
2022-01-29T07:22:48.341-0500    INFO    controller.config       Starting EventSource    {"reconciler group": "services.k8s.aws.cuppett.dev", "reconciler kind": "Config", "source": "kind source: *v1alpha1.Config"}
2022-01-29T07:22:48.341-0500    INFO    controller.config       Starting Controller     {"reconciler group": "services.k8s.aws.cuppett.dev", "reconciler kind": "Config"}
2022-01-29T07:22:48.341-0500    INFO    controller.stack        Starting EventSource    {"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack", "source": "kind source: *v1alpha1.Stack"}
2022-01-29T07:22:48.341-0500    INFO    controller.stack        Starting EventSource    {"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack", "source": "kind source: *v1.ConfigMap"}
2022-01-29T07:22:48.341-0500    INFO    controller.stack        Starting Controller     {"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack"}
2022-01-29T07:22:49.342-0500    INFO    controller.stack        Starting workers        {"reconciler group": "cloudformation.services.k8s.aws.cuppett.dev", "reconciler kind": "Stack", "worker count": 1}
2022-01-29T07:22:49.342-0500    INFO    controller.config       Starting workers        {"reconciler group": "services.k8s.aws.cuppett.dev", "reconciler kind": "Config", "worker count": 1}
2022-01-29T07:22:49.343-0500    INFO    workers.Config  Region resolved {"region": "us-east-1"}
```

## Appendix: Cluster Configuration

### Region

The AWS SDK for go does have a default region search order. 
To assist with kubernetes deployments there are a couple additional fallbacks here.
It is also possible to define region within the cluster.

### Controller Config

This method of detecting/configuring the region can be used as a fallback.

```yaml
apiVersion: services.k8s.aws.cuppett.dev/v1alpha1
kind: Config
metadata:
  name: default
  namespace: aws-cloudformation-controller-system
spec:
  region: us-east-1
```

> NOTE: The name of `Config` must be `default` and in the namespace of the pod.

### OpenShift Infrastructure

This method will be used if all else fails (and you are running either on OpenShift or where this type is loaded).
This is a cluster-wide CRD and only `cluster` will be looked for.

```yaml
apiVersion: config.openshift.io/v1
kind: Infrastructure
metadata:
  name: cluster
spec:
  cloudConfig:
    name: ''
  platformSpec:
    aws: {}
    type: AWS
status:
  ...
  platform: AWS
  platformStatus:
    aws:
      region: us-east-2
    type: AWS
```

## Appendix: Command-line arguments

There are a number of parameters to the controller which are not in the default manifests, but that allow further customization of it.
These may be useful for restricting permissions, adding specific tags or in support of various deployment topologies.

| Argument    | Environment variable | Default value | Description                                                                                                                                                                                                                                                                                                                              |
|-------------|----------------------|---------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| namespace   | WATCH_NAMESPACE      | (all)         | The Kubernetes namespace to watch. Can be one or more (separated by commas).                                                                                                                                                                                                                                                             |
| dry-run     |                      |               | If true, don't actually do anything.                                                                                                                                                                                                                                                                                                     |
| no-webhook  |                      |               | If true, don't listen on the webhook port (used for local dev)                                                                                                                                                                                                                                                                           |

