module github.com/cuppett/aws-cloudformation-operator

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.13.0
	github.com/aws/aws-sdk-go-v2/config v1.13.1
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.18.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.14.0
	github.com/go-logr/logr v1.2.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/openshift/api v0.0.0-20211028023115-7224b732cc14
	github.com/openshift/cloud-credential-operator v0.0.0-20211201043943-d642d1125fa4
	github.com/prometheus/client_golang v1.12.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.23.3
	k8s.io/apiextensions-apiserver v0.23.3
	k8s.io/apimachinery v0.23.3
	k8s.io/client-go v0.23.3
	sigs.k8s.io/controller-runtime v0.10.3
)
