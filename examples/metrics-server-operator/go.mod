module sigs.k8s.io/kubebuilder-declarative-pattern/examples/metrics-server-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0-20200127222139-6fce7388950a
)

replace sigs.k8s.io/kubebuilder-declarative-pattern => ../../
