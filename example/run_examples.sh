#!/bin/bash
set -e

# Change to the root directory
cd "$(dirname "$0")/.."

# Compile loom binary
echo "Building loom binary..."
go build -o loom ./cmd/loom
echo "Done."
echo "--------------------------------------------------------"

# Ensure we have git history for the git-diff to work
# Let's create a fake commit in our repo to simulate a change
echo "Creating a simulated code change to trigger git-diff..."
echo "// modified $(date +%s)" >> example/src/Accounting/API/main.go
echo "// modified $(date +%s)" >> example/src/Billing/main.go
echo "// modified $(date +%s)" >> example/src/workers/Analytics/main.go
git add example/src/
git commit -m "Simulate changes to AccountingAPI, BillingService, and AnalyticsWorker" > /dev/null 2>&1 || true

# 1. Dev Branch (Git diff mode)
echo "[Example 1] Simulating a push to 'dev' branch (git-diff strategy)"
CI_COMMIT_BRANCH="dev" \
CI_COMMIT_SHA=$(git rev-parse HEAD) \
CI_COMMIT_BEFORE_SHA=$(git rev-parse HEAD~1 2>/dev/null || echo "") \
./loom generate --config example/pipeline-strategies.yaml --services example/services.json --out generated-child.yml
echo "Generated pipeline written to generated-child.yml:"
cat generated-child.yml
echo ""
echo "--------------------------------------------------------"

# 2. Release Tag (Regex pattern mode)
echo "[Example 2] Simulating a production release tag 'AccountingAPI/v1.0.0' (regex-tag strategy)"
CI_COMMIT_TAG="AccountingAPI/v1.0.0" \
CI_COMMIT_REF_PROTECTED="true" \
./loom generate --config example/pipeline-strategies.yaml --services example/services.json --out generated-child.yml
echo "Generated pipeline written to generated-child.yml:"
cat generated-child.yml
echo ""
echo "--------------------------------------------------------"

# 3. Helm Chart fallback (Regex pattern mode)
echo "[Example 3] Simulating a production release tag 'BillingService/v2.1.0' (regex-tag with Helm chart fallback)"
CI_COMMIT_TAG="BillingService/v2.1.0" \
CI_COMMIT_REF_PROTECTED="true" \
./loom generate --config example/pipeline-strategies.yaml --services example/services.json --out generated-child.yml
echo "Generated pipeline written to generated-child.yml:"
cat generated-child.yml
echo ""
echo "--------------------------------------------------------"

# 4. Manual Web Execution (Env Match mode)
echo "[Example 4] Simulating a manual pipeline triggered via Web UI with variables (env-match strategy)"
CI_PIPELINE_SOURCE="web" \
CI_PIPELINE_ID="123456" \
DEPLOY_ACCOUNTINGAPI="true" \
DEPLOY_ANALYTICSWORKER="true" \
./loom generate --config example/pipeline-strategies.yaml --services example/services.json --out generated-child.yml
echo "Generated pipeline written to generated-child.yml:"
cat generated-child.yml
echo ""
echo "--------------------------------------------------------"

echo "Check out example/templates/*.tmpl to see how the logic generates these YAML outputs!"

