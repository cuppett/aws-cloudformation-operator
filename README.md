# cloudformation-operator

A custom resource definition for CloudFormation stacks and a Kubernetes operator for managing them.

## Deploy to a cluster

You need API access to a cluster running at least Kubernetes v1.19+ (OpenShift 4.6+).

Start the CloudFormation operator in your cluster by using the provided manifests:

```console
$ export AWS_ACCESS_KEY_ID=XXXXX
$ export AWS_SECRET_ACCESS_KEY=XXXXX
$ export AWS_REGION=XXXXX
$ make deploy IMG=quay.io/cuppett/cloudformation-operator:main
```

Additionally, you need to make sure that the operator Pod has enough AWS IAM permissions to create, update and delete 
CloudFormation stacks as well as permission to modify any resources that are part of the CloudFormation stacks you 
intend to deploy. 
In order to follow the example below it needs access to CloudFormation as well as S3.

The operator will require an IAM role or user credentials. 
Use the following Policy document as a guideline in order to follow the tutorial:

```yaml
MyIAMRole:
  Properties:
    ...
    Policies:
    - PolicyDocument:
        Statement:
        - {Action: 'cloudformation:*', Effect: Allow, Resource: '*'}
        - {Action: 's3:*', Effect: Allow, Resource: '*'}
        Version: '2012-10-17'
    ...
```

The operator will usually use the IAM role of the EC2 instance it's running on, so you have to add those permissions to 
that role. 
If you're using [EKS OIDC](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) or similar
method and give your Pod a dedicated IAM role then you have to add the permissions to that role.

Once running the operator should print some output but shouldn't actually do anything at this point. 
Leave it running, keep watching its logs and continue with the steps below.

## Demo

### Create stack

Currently you don't have any stacks.

```console
$ kubectl get stacks
No resources found.
```

Let's create a simple one that manages an S3 bucket:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
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

### Template URL

If your template exceeds maximum size of `51200` bytes, you can instead upload it to S3, [get presigned url](https://docs.aws.amazon.com/AmazonS3/latest/userguide/ShareObjectPreSignedURL.html#generating-presigned-url) if this bucket is private, and set it's URL in `templateUrl`:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
kind: Stack
metadata:
  name: my-stack
spec:
  templateUrl: 'https://my-bucket-name.s3.amazonaws.com/template_file.json?AWSAccessKeyId=AKIAIOSFODNN7EXAMPLE&Signature=adfj9yqihfseuifhsukf&Expires=1629952961'
```

> NOTE: Put URL in quotes to avoid templating issues

### Create options

To change stack behafiour on creation use `onFailure` that suports `DELETE`, `DO_NOTHING`, and `ROLLBACK` options:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
kind: Stack
metadata:
  name: my-stack
spec:
  template: |
    ...
  onFailure: DELETE
```

### Update stack

You can also update your stack: Let's change the `VersioningConfiguration` from `Suspended` to `Enabled`:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
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

### Tags

You may want to assign tags to your CloudFormation stacks. 
The tags added to a CloudFormation stack will be propagated to the managed resources. 
This feature may be useful in multiple cases, for example, to distinguish resources at billing report. 
Current operator provides two ways to assign tags:
- `--tag` command line argument or `AWS_TAGS` environment variable which allows setting default tags for all resources managed by the operator. The format is `--tag=foo=bar --tag=wambo=baz` on the command line or with a line break when specifying as an env var. (e.g. in zsh: `AWS_TAGS="foo=bar"$'\n'"wambo=baz"`)
- `tags` parameter at kubernetes resource spec:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
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

Resource-specific tags have precedence over the default tags. 
Thus if a tag is defined at command-line arguments and for a `Stack` resource, the value from the `Stack` resource will
be used.

If we run the operation and a `Stack` resource with the described above examples, we'll see such picture:

![Stack tags](docs/img/stack-tags.png)

### Parameters

However, often you'll want to extract dynamic values out of your CloudFormation stack template into so called `Parameters` 
so that your template itself doesn't change that often and, well, is really a *template*.

Let's extract the `VersioningConfiguration` into a parameter:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
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

Furthermore, CloudFormation supports so called `Outputs`. These can be used for dynamic values that are only known after 
a stack has been created.
In our example, we don't define a particular S3 bucket name but instead let AWS generate one for us.

Let's change our CloudFormation template to expose the generated bucket name via an `Output`:

```yaml
apiVersion: cloudformation.cuppett.com/v1beta1
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

## Command-line arguments

Argument | Environment variable | Default value | Description
---------|----------------------|---------------|------------
assume-role | | | Assume AWS role when defined. Useful for stacks in another AWS account. Specify the full ARN, e.g. `arn:aws:iam::123456789:role/cloudformation-operator`
capability | | | Enable specified capabilities for all stacks managed by the operator instance. Current parameter can be used multiple times. For example: `--capability CAPABILITY_NAMED_IAM --capability CAPABILITY_IAM`. Or with a line break when specifying as an environment variable: `AWS_CAPABILITIES=CAPABILITY_IAM$'\n'CAPABILITY_NAMED_IAM`
dry-run | | | If true, don't actually do anything.
tag ... | | | Default tags which should be applied for all stacks. The format is `--tag=foo=bar --tag=wambo=baz` on the command line or with a line break when specifying as an env var. (e.g. in zsh: `AWS_TAGS="foo=bar"$'\n'"wambo=baz"`)
namespace | WATCH_NAMESPACE | (all) | The Kubernetes namespace to watch

## Cleanup

Clean up the resources:

```console
$ make undeploy
```

## Build locally

This project uses the [operator sdk](https://github.com/operator-framework/operator-sdk).

```console
$ make
```

### Build and publish the docker image

```console
$ make docker-build docker-push IMG=quay.io/cuppett/cloudformation-operator:latest
```

### Test it

(Assuming you have already configured your KUBECONFIG or other means)

```console
$ export AWS_ACCESS_KEY_ID=XXXXX
$ export AWS_SECRET_ACCESS_KEY=XXXXX
$ export AWS_REGION=XXXXX
$ make deploy IMG=quay.io/cuppett/cloudformation-operator:latest
/home/scuppett/go/src/github.com/cuppett/cloudformation-operator/bin/controller-gen "crd:trivialVersions=true,preserveUnknownFields=false" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
cd config/manager && /home/scuppett/go/src/github.com/cuppett/cloudformation-operator/bin/kustomize edit set image controller=controller:latest
/home/scuppett/go/src/github.com/cuppett/cloudformation-operator/bin/kustomize build config/default | kubectl apply -f -
namespace/cloudformation-operator-system created
serviceaccount/cloudformation-operator-controller-manager created
role.rbac.authorization.k8s.io/cloudformation-operator-leader-election-role created
clusterrole.rbac.authorization.k8s.io/cloudformation-operator-manager-role created
clusterrole.rbac.authorization.k8s.io/cloudformation-operator-metrics-reader created
clusterrole.rbac.authorization.k8s.io/cloudformation-operator-proxy-role created
rolebinding.rbac.authorization.k8s.io/cloudformation-operator-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/cloudformation-operator-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/cloudformation-operator-proxy-rolebinding created
configmap/cloudformation-operator-manager-config created
secret/cloudformation-operator-aws-keys-bggbf5bk55 created
service/cloudformation-operator-controller-manager-metrics-service created
deployment.apps/cloudformation-operator-controller-manager created

$ kubectl logs -n cloudformation-operator-system [POD_NAME] manager
I0616 09:57:32.743358       1 request.go:655] Throttling request took 1.030675011s, request: GET:https://10.217.4.1:443/apis/authorization.k8s.io/v1?timeout=32s
2021-06-16T09:57:34.716Z	INFO	controller-runtime.metrics	metrics server is starting to listen	{"addr": "127.0.0.1:8080"}
2021-06-16T09:57:34.717Z	INFO	setup
2021-06-16T09:57:34.717Z	INFO	setup	starting manager
I0616 09:57:34.717500       1 leaderelection.go:243] attempting to acquire leader lease cloudformation-operator-system/3680e595.cuppett.com...
2021-06-16T09:57:34.717Z	INFO	controller-runtime.manager	starting metrics server	{"path": "/metrics"}
I0616 09:57:34.731016       1 leaderelection.go:253] successfully acquired lease cloudformation-operator-system/3680e595.cuppett.com
2021-06-16T09:57:34.731Z	INFO	controller-runtime.manager.controller.stack	Starting EventSource	{"reconciler group": "cloudformation.cuppett.com", "reconciler kind": "Stack", "source": "kind source: /, Kind="}
2021-06-16T09:57:34.731Z	DEBUG	controller-runtime.manager.events	Normal	{"object": {"kind":"ConfigMap","namespace":"cloudformation-operator-system","name":"3680e595.cuppett.com","uid":"f088468b-39e9-48cf-bdde-6db0b9bd4d57","apiVersion":"v1","resourceVersion":"64943"}, "reason": "LeaderElection", "message": "cloudformation-operator-controller-manager-7cb9f97dc7-4zdjm_542574a7-e0a4-48c4-b1fb-49025ffa3a1d became leader"}
2021-06-16T09:57:34.731Z	DEBUG	controller-runtime.manager.events	Normal	{"object": {"kind":"Lease","namespace":"cloudformation-operator-system","name":"3680e595.cuppett.com","uid":"5c3c65d3-7aed-4c3e-8654-3dc28a207d84","apiVersion":"coordination.k8s.io/v1","resourceVersion":"64944"}, "reason": "LeaderElection", "message": "cloudformation-operator-controller-manager-7cb9f97dc7-4zdjm_542574a7-e0a4-48c4-b1fb-49025ffa3a1d became leader"}
2021-06-16T09:57:34.833Z	INFO	controller-runtime.manager.controller.stack	Starting Controller	{"reconciler group": "cloudformation.cuppett.com", "reconciler kind": "Stack"}
2021-06-16T09:57:34.833Z	INFO	controller-runtime.manager.controller.stack	Starting workers	{"reconciler group": "cloudformation.cuppett.com", "reconciler kind": "Stack", "worker count": 1}
```
