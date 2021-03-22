module sigs.k8s.io/kubebuilder-declarative-pattern/examples/guestbook-operator

go 1.13

require (
	github.com/go-logr/logr v0.3.0
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.20.1
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0
)

replace sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0 => ../../../kubebuilder-declarative-pattern
