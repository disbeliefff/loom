#!/bin/bash
set -e

# Change to the root directory
cd "$(dirname "$0")/.."

# Set the output file path specifically inside the example directory
OUTPUT_FILE="example/generated-child.yml"

# Clean up previous generated file if it exists
rm -f "$OUTPUT_FILE"

# Function to run loom and append both the description and the output to the generated file
run_example() {
  local desc="$1"
  shift
  
  echo "--------------------------------------------------------" >> "$OUTPUT_FILE"
  echo "$desc" >> "$OUTPUT_FILE"
  echo "--------------------------------------------------------" >> "$OUTPUT_FILE"
  
  # Run the command and append its stdout to our output file
  "$@" >> "$OUTPUT_FILE"
  
  echo "" >> "$OUTPUT_FILE"
  echo "" >> "$OUTPUT_FILE"
}

# Compile loom binary
echo "Building loom binary..."
go build -o loom ./cmd/loom
echo "Done."
echo "--------------------------------------------------------"

# Let's create a fake commit in our repo to simulate a change
echo "Creating a simulated code change to trigger git-diff..."
echo "# modified $(date +%s)" >> example/src/Accounting/API/main.py
echo "# modified $(date +%s)" >> example/src/Billing/main.py
echo "# modified $(date +%s)" >> example/src/workers/Analytics/main.py
git add example/src/
git commit -m "Simulate changes to AccountingAPI, BillingService, and AnalyticsWorker" > /dev/null 2>&1 || true

# 1. Dev Branch (Git diff mode)
echo "Running Example 1 (Dev Branch)..."
export CI_COMMIT_BRANCH="dev"
export CI_COMMIT_SHA=$(git rev-parse HEAD)
export CI_COMMIT_BEFORE_SHA=$(git rev-parse HEAD~1 2>/dev/null || echo "")
run_example "[Example 1] Simulating a push to 'dev' branch (git-diff strategy)" ./loom generate --config example/pipeline-strategies.yaml --services example/services.json

# 2. Release Tag (Regex pattern mode)
echo "Running Example 2 (Release Tag)..."
export CI_COMMIT_TAG="AccountingAPI/v1.0.0"
export CI_COMMIT_REF_PROTECTED="true"
# Unset previous branch envs to keep it clean
unset CI_COMMIT_BRANCH CI_COMMIT_SHA CI_COMMIT_BEFORE_SHA
run_example "[Example 2] Simulating a production release tag 'AccountingAPI/v1.0.0' (regex-tag strategy)" ./loom generate --config example/pipeline-strategies.yaml --services example/services.json

# 3. Helm Chart fallback (Regex pattern mode)
echo "Running Example 3 (Helm fallback)..."
export CI_COMMIT_TAG="BillingService/v2.1.0"
export CI_COMMIT_REF_PROTECTED="true"
run_example "[Example 3] Simulating a production release tag 'BillingService/v2.1.0' (regex-tag with Helm chart fallback)" ./loom generate --config example/pipeline-strategies.yaml --services example/services.json

# 4. Manual Web Execution (Env Match mode)
echo "Running Example 4 (Manual Deploy)..."
unset CI_COMMIT_TAG CI_COMMIT_REF_PROTECTED
export CI_PIPELINE_SOURCE="web"
export CI_PIPELINE_ID="123456"
export DEPLOY_ACCOUNTINGAPI="true"
export DEPLOY_ANALYTICSWORKER="true"
run_example "[Example 4] Simulating a manual pipeline triggered via Web UI with variables (env-match strategy)" ./loom generate --config example/pipeline-strategies.yaml --services example/services.json

echo "Done! Check out the complete output in '$OUTPUT_FILE'"

