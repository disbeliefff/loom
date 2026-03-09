#  Loom

**Loom** is a fast, declarative CLI utility, designed to dynamically generate GitLab CI child pipelines for **monorepos** and seamlessly adapt them for **GitOps workflows** (e.g., FluxCD, ArgoCD).




---

##  GitOps Example (FluxCD)

Let's look at how to use Loom to build an automated GitOps pipeline using FluxCD.

### 1. Define your Services (`services.json`)
This registry maps your monorepo source directories (`watch_dir`) to your Docker images and FluxCD deployment paths (`kustomize`).

```json
[
  {
    "key": "AccountingAPI",
    "watch_dir": "src/Accounting/API",
    "image": "registry.gitlab.com/org/accounting-api",
    "kustomize": "apps/accounting/overlays/prod"
  },
  {
    "key": "AuthService",
    "watch_dir": "src/Auth",
    "image": "registry.gitlab.com/org/auth-service",
    "kustomize": "apps/auth/overlays/prod"
  }
]
```

### 2. Define your Strategies (`pipeline-strategies.yaml`)
Define the rules for when to trigger different pipeline templates. Loom uses a powerful expression engine (`expr`) to evaluate GitLab CI variables.

```yaml
strategies:
  # 1. Automatic build on the 'dev' branch (detects changed files)
  - name: "dev-build"
    condition: 'env("CI_COMMIT_BRANCH") == "dev"'
    selector:
      type: "git-diff"
      before_sha: '{{ .Context.BeforeSHA }}'
      current_sha: '{{ .Context.CommitSHA }}'
      watch_field: "watch_dir"
    template: ".gitlab/templates/build.tmpl"

  # 2. Production release triggered by a Git Tag (e.g., AccountingAPI/v1.0.0)
  - name: "tag-release"
    condition: 'env("CI_COMMIT_TAG") != "" && env("CI_COMMIT_REF_PROTECTED") == "true"'
    selector:
      type: "regex-tag"
      pattern: '^{{ .Service.Key }}/.*$'
    template: ".gitlab/templates/release.tmpl"

  # 3. Manual Web UI triggers (e.g., passing DEPLOY_AUTHSERVICE=true)
  - name: "manual"
    condition: 'env("CI_PIPELINE_SOURCE") in ["web", "api"] || env("MANUAL_SERVICE_DEPLOY") != ""'
    selector:
      type: "env-match"
      prefix: "DEPLOY_"
    template: ".gitlab/templates/manual.tmpl"
```

### 3. Create your Pipeline Template (`.gitlab/templates/build.tmpl`)
Loom passes the filtered services to this template. Notice how we seamlessly inject `.Service.Raw.kustomize` into the downstream CI job so it knows exactly where to commit the image tag update for FluxCD.

```gotemplate
include:
  - project: 'your-org/devops/ci-templates'
    file: 'templates/build/docker-build.yml'
  - project: 'your-org/devops/ci-templates'
    file: 'templates/deploy/gitops-promote.yml'

stages:
  - build
  - gitops-sync

{{ if not .Jobs }}
no-op:
  stage: build
  script:
    - echo "No monorepo services were modified in this commit."
{{ end }}

{{ range .Jobs }}
build:{{ .Service.SafeKey }}:
  extends: .docker-build
  stage: build
  variables:
    IMAGE_NAME: "{{ .Service.Raw.image }}"
    BUILD_CONTEXT: "{{ .Service.Raw.watch_dir }}"

# This job updates the GitOps repository for FluxCD
promote:{{ .Service.SafeKey }}:
  extends: .gitops-promote
  stage: gitops-sync
  needs: ["build:{{ .Service.SafeKey }}"]
  variables:
    IMAGE_NAME: "{{ .Service.Raw.image }}"
    IMAGE_TAG: "{{ $.Context.CommitSHA }}"
    # Direct parameter injection for FluxCD:
    KUSTOMIZE_PATH: "{{ .Service.Raw.kustomize }}" 
{{ end }}
```

### 4. Integrate with GitLab CI (`.gitlab-ci.yml`)
Use the Loom Docker image as the base for your setup job. It will generate the `child-pipeline.yml` and trigger it.

```yaml
stages:
  - setup
  - triggers

generate:
  stage: setup
  image: registry.gitlab.com/your-org/devops/loom:latest
  script:
    - loom generate --config .gitlab/pipeline-strategies.yaml --services services.json --out child-pipeline.yml
  artifacts:
    paths:
      - child-pipeline.yml

trigger:
  stage: triggers
  trigger:
    include:
      - artifact: child-pipeline.yml
        job: generate
    strategy: depend
```

---

## Usage & Commands

```bash
# Generate a pipeline (defaults to stdout if --out is not provided)
loom generate --config <path> --services <path> --out <file>

# Validate configuration syntax and template paths without generating a pipeline
loom validate --config <path> --services <path>
```

**Global Flags**:
*   `--debug` / `-d`: Enables verbose debug logging (useful for troubleshooting `git-diff` edge cases).
