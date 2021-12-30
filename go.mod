module github.com/cuppett/aws-cloudformation-controller

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.11.2
	github.com/aws/aws-sdk-go-v2/config v1.11.1
	github.com/aws/aws-sdk-go-v2/credentials v1.6.5
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.16.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.12.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	sigs.k8s.io/controller-runtime v0.10.3
)
