# PLAN: Continuous GitOps Promotion for Flux HelmRelease

  ## Summary

  Add continuous GitOps promotion to Loom without moving promotion behavior into consumer repositories. Keep the current ownership split:

  - consumer repo owns services.json
  - ci-templates owns pipeline-strategies.yaml and templates
  - Loom joins them at runtime

  services.json remains a flat service catalog. pipeline-strategies.yaml gains a typed promotion block that describes how selected services are promoted to Flux-managed
  HelmRelease targets.

  ## Config Model

  services.json stays simple and service-specific:

  [
    {
      "key": "wallet-api",
      "watch_dir": "services/wallet-api",
      "image": "registry.example.com/platform/wallet-api",
      "kustomize": "clusters/production/apps/wallet-api",
      "helm_release": "wallet-api",
      "helm_namespace": "wallet"
    }
  ]

  pipeline-strategies.yaml owns promotion behavior:

  strategies:
    - name: tag-release
      condition: 'env("CI_COMMIT_TAG") != ""'
      selector:
        type: regex-tag
        pattern: '^{{ .Service.Key }}/.*$'
      template: templates/release.tmpl
      promotion:
        enabled: true
        provider: flux
        mode: direct-commit
        target: production
        checkout_path: ../gitops
        manifest_path_field: kustomize
        object_ref:
          api_version: helm.toolkit.fluxcd.io/v2
          kind: HelmRelease
          name_field: helm_release
          namespace_field: helm_namespace
        image_ref:
          repository_field: image
          repository_path: spec.values.image.repository
          tag_path: spec.values.image.tag
          event_annotation: event.toolkit.fluxcd.io/image
        rollback:
          strategy: previous-annotation

  Defaults:

  - object_ref.api_version: helm.toolkit.fluxcd.io/v2
  - object_ref.kind: HelmRelease
  - object_ref.name_field: key
  - image_ref.repository_field: image
  - image_ref.repository_path: spec.values.image.repository
  - image_ref.tag_path: spec.values.image.tag
  - image_ref.event_annotation: event.toolkit.fluxcd.io/image
  - rollback.strategy: previous-annotation

  ## Public CLI

  Existing generation remains:

  loom generate \
    --config .ci-templates/templates/monorepo/pipeline-strategies.yaml \
    --services services.json \
    --out child-pipeline.yml

  Add GitOps commands:

  loom gitops promote \
    --config .ci-templates/templates/monorepo/pipeline-strategies.yaml \
    --services services.json \
    --strategy tag-release \
    --service wallet-api \
    --tag v1.2.3

  loom gitops rollback \
    --config .ci-templates/templates/monorepo/pipeline-strategies.yaml \
    --services services.json \
    --strategy tag-release \
    --service wallet-api

  Polyrepo behavior:

  - --service is optional only when services.json contains exactly one service.
  - --strategy is required.
  - --tag is required for promote.
  - --dry-run is supported for both promote and rollback.

  ## Promotion Behavior

  - CI prepares the GitOps checkout and Git credentials before invoking Loom.
  - checkout_path points to an already cloned GitOps repo.
  - manifest_path_field points to a field in services.json; its value is relative to checkout_path.
  - Loom finds exactly one manifest under that path matching:
      - apiVersion
      - kind
      - metadata.name
      - metadata.namespace
  - Loom fails if the matched manifest contains $imagepolicy comments, because the same image fields must not be owned by both Flux Image Automation and Loom direct
    promotion.
  - Promotion updates:
      - configured repository path
      - configured tag path
      - configured event annotation
  - Promotion stores one-step rollback state in annotations:
      - loom.disbeliefff.github.io/previous-image
      - loom.disbeliefff.github.io/previous-repository
      - loom.disbeliefff.github.io/previous-tag
      - loom.disbeliefff.github.io/promoted-at
      - loom.disbeliefff.github.io/promoted-by
  - Rollback restores the previous repository/tag/event annotation and swaps the current values back into previous annotations.
  - Loom uses system git for add/commit/push so CI identity, remote URL, and credential helper are reused.
  - Loom does not manage Git credentials or patch the live cluster.

  ## Implementation Changes

  - Extend models.Strategy with optional typed Promotion.
  - Keep models.Service.Raw map[string]string unchanged.
  - Add resolver that combines a selected Strategy.Promotion with a selected Service.Raw.
  - Add gitops Cobra command group with promote and rollback.
  - Add local checkout validation and system-git wrapper.
  - Add HelmRelease finder/mutator using sigs.k8s.io/yaml.
  - Add pre-mutation scan for $imagepolicy in the matched manifest.
  - Update release templates to call loom gitops promote instead of embedding YAML patch logic.
  - Update docs/examples to show:
      - unchanged flat services.json
      - promotion block in pipeline-strategies.yaml
      - monorepo generated child pipeline promotion
      - polyrepo one-service promotion

  ## Test Plan

  - Existing generation:
      - current services.json + pipeline-strategies.yaml tests continue passing.
      - template path resolution remains relative to strategy config file.
  - Promotion config:
      - default values are applied correctly.
      - invalid missing field references fail clearly.
      - disabled or missing promotion block fails for gitops promote.
  - Service resolution:
      - --service optional with one service.
      - --service required with multiple services.
      - unknown service fails clearly.
  - Manifest mutation:
      - exactly one matching HelmRelease is required.
      - zero or multiple matches fail.
      - repository/tag/event annotation update correctly.
      - mutation fails when $imagepolicy comments are present.
  - Rollback:
      - promotion writes previous annotations.
      - rollback restores previous repository/tag/event annotation.
      - rollback fails when previous annotations are missing.
  - Git:
      - fails if checkout_path is not a Git repo.
      - dry-run does not commit or push.
      - commit contains only the changed HelmRelease file.
      - push retry runs git pull --rebase then retries git push.

  ## Assumptions

  - v1 supports only Flux HelmRelease.
  - Rollback supports one previous version through annotations.
  - Rollback to older versions is done by running a new promotion with the desired tag.
  - services.json remains consumer-owned service metadata.
  - Promotion behavior remains centralized in ci-templates.
  - Flux Image Automation may own non-production paths; Loom direct promotion owns protected promotion paths.
  - The same image fields in the same GitOps path must not be written by both Flux Image Automation and Loom promotion.
