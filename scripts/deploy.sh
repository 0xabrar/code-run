#!/usr/bin/env bash
# deploy.sh -- build images and deploy CodeRunner to a local (kind) cluster or any kube context.

set -euo pipefail

NAMESPACE=${NAMESPACE:-coderunner}
KIND_CLUSTER=${KIND_CLUSTER:-kind}
IMAGES=(coderunner/dispatcher:latest coderunner/runner-go:latest coderunner/runner-python:latest coderunner/runner-node:latest)

bold() { echo -e "\033[1m$1\033[0m"; }

export DOCKER_BUILDKIT=1

echo "$(bold "1/5") Building container images"

docker build -t ${IMAGES[0]} -f Dockerfile.dispatcher .
docker build -t ${IMAGES[1]} -f Dockerfile.runner-go     .
docker build -t ${IMAGES[2]} -f Dockerfile.runner-python .
docker build -t ${IMAGES[3]} -f Dockerfile.runner-node   .

# If a Kind cluster is running, load the images so no registry push is needed.
if command -v kind >/dev/null 2>&1 && kind get clusters | grep -q "^${KIND_CLUSTER}$"; then
  echo "$(bold "2/5") Loading images into kind cluster '${KIND_CLUSTER}'"
  for img in "${IMAGES[@]}"; do
    kind load docker-image "$img" --name "$KIND_CLUSTER"
  done
else
  echo "$(bold "2/5") No kind cluster detected – assuming images are pullable by the current kube context."
fi

echo "$(bold "3/5") Creating namespace $NAMESPACE (if missing)"
if ! kubectl get ns "$NAMESPACE" >/dev/null 2>&1; then
  kubectl create ns "$NAMESPACE"
fi

echo "$(bold "4/5") Applying Kubernetes manifests"
kubectl apply -n "$NAMESPACE" -f k8s/dispatcher.yaml
kubectl apply -n "$NAMESPACE" -f k8s/runner-go.yaml
kubectl apply -n "$NAMESPACE" -f k8s/runner-python.yaml
kubectl apply -n "$NAMESPACE" -f k8s/runner-node.yaml

echo "$(bold "5/5") All set! Access the API with:\n  kubectl -n $NAMESPACE port-forward svc/coderunner-dispatcher 8080:80" 