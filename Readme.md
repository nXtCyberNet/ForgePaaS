# ForgePaaS 🚀

A self-hosted, open-source **Platform-as-a-Service (PaaS)** inspired by Heroku.

ForgePaaS lets developers **build, deploy, run, and manage applications** using a CLI and API, backed by Kubernetes. The focus is **simplicity first**, scale later.

---

## 🎯 Goals

* Heroku-like developer experience
* Fully self-hosted
* Kubernetes-native runtime
* CLI and API driven deployments
* Minimal, understandable architecture

---

## 🧱 High-Level Architecture (Version 1)

CLI

↓API Server

↓
Redis (State + Message Queue)

↓
CNB Builder (Cloud Native Buildpacks)

↓
Local Docker Registry

↓
Kubernetes Controller

↓Kubernetes Pods

↓
Reverse Proxy (Nginx / Traefik)

---

## 🧩 Core Components

### 1️⃣ API Server

**One line:** Central control plane that accepts CLI requests, manages app state, and triggers build/deploy workflows via Redis.

---

### 2️⃣ Redis (Local Storage + Queue)

**One line:** In-memory store used for app metadata, state tracking, and build/deploy job queues.

---

### 3️⃣ CNB Builder (Cloud Native Buildpacks)

**One line:** Builds OCI container images from source code using Cloud Native Buildpacks.

---

### 4️⃣ Local Docker Registry

**One line:** Stores built container images locally for fast and reliable Kubernetes pulls.

---

### 5️⃣ Kubernetes Controller

**One line:** Watches deploy events and creates or updates Kubernetes resources for each app.

---

### 6️⃣ Reverse Proxy (Nginx / Traefik)

**One line:** Dynamically routes incoming traffic to the correct application pods using subdomains.

---

### 7️⃣ CLI Tool

**Role:** Developer-facing interface

**Version 1 Commands:**
forge deploy
forge status
forge apps
forge logs

**Responsibilities:**

* Deploy applications
* Show build and runtime status
* Stream application logs

---

## 🔁 Deployment Flow (Version 1)

forge deploy
↓
API receives request
↓
Redis queues build job
↓
CNB builds image
↓
Image pushed to local registry
↓
Kubernetes controller deploys image
↓
Reverse proxy routes traffic

---

## 📦 Version 1 Scope (MVP)

* Application deployment
* CNB-based image builds
* Local Docker registry
* Kubernetes-based runtime
* Dynamic routing
* CLI deploy and status
* Basic log streaming

**Not included in v1:**

* Authentication
* Multi-tenant isolation
* Autoscaling
* Billing

---

## 🔐 Version 2 (Planned)

### Security

* CLI authentication (token-based)
* API authentication middleware
* Role-based access control
* Namespace isolation per user

### Observability

* Live container log streaming via CLI
* Application metrics
* Health checks

### Platform Features

* Automatic HTTPS
* Autoscaling
* Rollbacks
* Secrets management

---

## 🛡️ Security Philosophy

* Least privilege by default
* No Docker socket exposure
* Resource limits on all containers
* Network isolation

---

## 🧠 Design Principles

* Simple over complex
* One responsibility per service
* Kubernetes as the final runtime
* Clear and observable systems
* No magic, only explicit flows

---

## 🚧 Project Status

**Version:** 0.1 (Active Development)

---

## 🤝 Contributing

This project is built for learning and real-world use.
Contributions, reviews, and ideas are welcome.

---

##
