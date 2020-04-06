module github.com/pwillie/ssm-secrets-webhook

go 1.14

require (
	emperror.dev/errors v0.7.0
	github.com/aws/aws-sdk-go v1.30.4
	github.com/banzaicloud/bank-vaults v0.0.0-20200323100356-7fadfb8416b0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/google/go-cmp v0.4.0
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1
	github.com/prometheus/client_golang v1.5.1
	github.com/sirupsen/logrus v1.5.0
	github.com/slok/kubewebhook v0.3.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/viper v1.6.2
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v11.0.1-0.20190516230509-ae8359b20417+incompatible
	sigs.k8s.io/controller-runtime v0.4.0
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
)
