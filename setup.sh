#!/bin/bash
set -euo pipefail

export PATH="$PATH:/usr/local/bin:/usr/bin:/bin"

# kubectl config location (important for CI / scripts)
export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"

# Helm config locations (prevents permission issues)
export HELM_CONFIG_HOME="$HOME/.config/helm"
export HELM_CACHE_HOME="$HOME/.cache/helm"
export HELM_DATA_HOME="$HOME/.local/share/helm"

# Kind / Minikube defaults
export KIND_CLUSTER_NAME="dev-cluster"
export MINIKUBE_PROFILE="minikube"

# Curl safety
export CURL_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt


# 1. Check for Kind or Minikube
echo "Checking for Kubernetes local runners..."

if command -v kind &> /dev/null; then
    RUNNER="kind"
    echo "Found Kind!"
elif command -v minikube &> /dev/null; then
    RUNNER="minikube"
    echo "Found Minikube!"
else
    echo "Error: Neither Kind nor Minikube found."
    exit 1
fi

# 2. Install kubectl if not present
if ! command -v kubectl &> /dev/null; then
    echo "Installing kubectl..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl
fi
if ! command -v helm &> /dev/null; then
    echo "installing helm"
    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4
    chmod 700 get_helm.sh
    ./get_helm.sh
fi
# 3. Start the cluster
if [ "$RUNNER" == "kind" ]; then
    if [[ $(kind get clusters) != *"kind"* ]]; then
        kind create cluster --name dev-cluster
    fi
else
    minikube start
fi

# 4. Apply your backend manifests
echo "Checking for backend manifests in ./backend/k8s_yaml..."

if [ -d "./backend/k8s_yaml" ]; then
    echo "Applying manifests..."
    kubectl apply -f ./backend/k8s_yaml
    
    # Optional: Wait for pods to be ready
    echo "Waiting for pods to initialize..."
    kubectl get pods -n default
else
    echo "Error: Directory ./backend/k8s_yaml not found. Are you in the root directory?"
    exit 1
fi
kubectl  create namespace builder 
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm install traefik traefik/traefik
kubectl create namespace builder

