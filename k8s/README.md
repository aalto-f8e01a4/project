# Kubernetes

Prerequisites: Kubernetes cluster (OrbStack, minikube, etc.), helm, helmfile, helm-diff.

- To install helm-diff: `helm plugin install https://github.com/databus23/helm-diff`
- To deploy everything: `helmfile apply`
- Get Grafana admin user password: `kubectl get secret grafana-admin --namespace monitoring -o jsonpath="{.data.GF_SECURITY_ADMIN_PASSWORD}" | base64 -d`
