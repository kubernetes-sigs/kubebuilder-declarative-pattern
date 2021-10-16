module sigs.k8s.io/kubebuilder-declarative-pattern/examples/guestbook-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	sigs.k8s.io/controller-runtime v0.10.0
	sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0-20210922163802-cac4a6cf1977
)

//replace sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0-20210922163802-cac4a6cf1977 => ../../
