# image-annotator-webhook

## How to test locally

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)

### Steps

1. Create a local Kubernetes cluster using Kind

  ```bash
  make cluster
  ```

2. Deploy the image-annotator-webhook to the cluster

  ```bash
  make push
  ```

3. Deploy webhook

  ```bash
  make deploy-webhook
  ```

  And wait some seconds for the webhook to be ready.

4. Deploy the manifests for testing (add any other manifests you want to test)

  ```bash
  make deploy-testing
  ```

5. Check the logs

  ```bash
  make logs-webhok
  ```

6. Confim that the webhook is working

  ```bash
  kubectl get <pod/deployment/statefulset/job/cronjob> -o yaml ...
  ```

7. Delete the cluster

  ```bash
  make delete-cluster
  ```
