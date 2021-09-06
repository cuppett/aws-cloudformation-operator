module github.com/cuppett/cloudformation-operator

go 1.16

require (
	github.com/aws/aws-sdk-go-v2 v1.9.0
	github.com/aws/aws-sdk-go-v2/config v1.8.0
	github.com/aws/aws-sdk-go-v2/credentials v1.4.0
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.10.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.7.0
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.21.3 // indirect
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.6
)
