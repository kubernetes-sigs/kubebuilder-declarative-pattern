# kubebuilder-declarative-pattern

kubebuilder-declarative-pattern provides a set of tools for building declarative cluster operators with kubebuilder. Declarative operators provide a fast path to orchestrating Kubernetes deployments to enable domain experts to focus their component instead of re-answering questions like 'how do I get this YAML into the cluster?' or 'how do I update it?'.

## 🚧 Work in Progress 🚧

This repo is under very active development and will change rapidly over the short term. Follow the [Work in Progress](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern/issues/3) issue for an all clear when it is ready for development.

## Development

### Running Smoke Tests

Smoke tests are provided to ensure basic functinality of the framework against example operators. They should be ran as part of significant code changes. The tests require a running Kubernetes cluster to be targeted from the local machine and write access to a GCR bucket.

```bash
cd hack
IMG=<a writeable image path, eg, gcr.io/my-project/controller:latest> go run smoketest.go
```

## Documentation

- [Managing Addons with Operators (Video, KubeCon'18)](https://www.youtube.com/watch?v=LPejvfBR5_w)

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack](http://slack.k8s.io/)
- [Mailing List](https://groups.google.com/forum/#!forum/kubernetes-dev)

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

[owners]: https://git.k8s.io/community/contributors/guide/owners.md
[Creative Commons 4.0]: https://git.k8s.io/website/LICENSE
