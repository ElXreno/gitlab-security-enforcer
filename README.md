# gitlab-security-enforcer

`gitlab-security-enforcer` is a small webhook service that receives GitLab System Hook events and automatically enables project security controls when a new project is created.

## Environment Variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `GITLAB_URL` | yes | none | GitLab base URL, for example `https://gitlab.example.com` |
| `GITLAB_TOKEN` | yes | none | Personal Access Token with `api` scope |
| `HOOK_SECRET` | yes | none | Expected value of incoming `X-Gitlab-Token` header |
| `LISTEN_ADDR` | no | `:8080` | HTTP listen address |

## Run Locally

```bash
export GITLAB_URL="https://gitlab.example.com"
export GITLAB_TOKEN="your-token"
export HOOK_SECRET="your-hook-secret"
export LISTEN_ADDR=":8080"

go run ./cmd/server
```

## Kubernetes Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gitlab-security-enforcer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gitlab-security-enforcer
  template:
    metadata:
      labels:
        app: gitlab-security-enforcer
    spec:
      containers:
        - name: app
          image: gitlab-security-enforcer:latest
          ports:
            - containerPort: 8080
          env:
            - name: GITLAB_URL
              value: "https://gitlab.example.com"
            - name: GITLAB_TOKEN
              value: "replace-me"
            - name: HOOK_SECRET
              value: "replace-me"
            - name: LISTEN_ADDR
              value: ":8080"
---
apiVersion: v1
kind: Service
metadata:
  name: gitlab-security-enforcer
spec:
  selector:
    app: gitlab-security-enforcer
  ports:
    - port: 80
      targetPort: 8080
      protocol: TCP
  type: ClusterIP
```

## Register GitLab System Hook

```bash
curl -X POST "https://gitlab.example.com/api/v4/hooks" \
  -H "PRIVATE-TOKEN: <admin-token>" \
  --data-urlencode "url=https://enforcer.example.com/" \
  --data-urlencode "token=<hook-secret>" \
  --data "project_events=true"
```
