#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -r -- "$test_root"' EXIT

fake_bin="$test_root/bin"
mkdir -p "$fake_bin"

cat >"$fake_bin/gcloud" <<'EOF'
#!/usr/bin/env bash

set -euo pipefail

printf '%s\n' "$*" >>"$FAKE_GCLOUD_LOG"

case "$*" in
  "artifacts docker images describe "*)
    printf '%s\n' "$EXPECTED_IMMUTABLE_IMAGE"
    ;;
  "run deploy "*)
    ;;
  "run revisions describe "*)
    printf '%s\n' "$EXPECTED_IMMUTABLE_IMAGE"
    ;;
  "scheduler jobs describe "*)
    exit 1
    ;;
  "scheduler jobs create http "* | "scheduler jobs pause "* | "run services update-traffic "*)
    ;;
  "scheduler jobs resume "*)
    resume_count_file="$FAKE_GCLOUD_STATE/resume-count"
    resume_count=0
    if [[ -f "$resume_count_file" ]]; then
      resume_count="$(<"$resume_count_file")"
    fi
    resume_count=$((resume_count + 1))
    printf '%s\n' "$resume_count" >"$resume_count_file"

    case "$FAKE_RESUME_MODE" in
      transient-once)
        if ((resume_count == 1)); then
          echo "ERROR: (gcloud.scheduler.jobs.resume) ABORTED: parent resource not in ready state" >&2
          exit 1
        fi
        ;;
      permanent)
        echo "ERROR: (gcloud.scheduler.jobs.resume) PERMISSION_DENIED: denied" >&2
        exit 1
        ;;
      *)
        echo "Unexpected FAKE_RESUME_MODE: $FAKE_RESUME_MODE" >&2
        exit 1
        ;;
    esac
    ;;
  *)
    echo "Unexpected gcloud invocation: $*" >&2
    exit 1
    ;;
esac
EOF

cat >"$fake_bin/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

chmod +x "$fake_bin/gcloud" "$fake_bin/sleep"

run_deploy() {
  local mode="$1"
  local case_root="$test_root/$mode"
  mkdir -p "$case_root"

  local image_tag="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  local immutable_image="us-east4-docker.pkg.dev/test-project/services/abita-middleware@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

  PATH="$fake_bin:$PATH" \
    FAKE_GCLOUD_LOG="$case_root/gcloud.log" \
    FAKE_GCLOUD_STATE="$case_root" \
    FAKE_RESUME_MODE="$mode" \
    EXPECTED_IMMUTABLE_IMAGE="$immutable_image" \
    PROJECT_ID="test-project" \
    REGION="us-east4" \
    REPOSITORY="services" \
    IMAGE="abita-middleware" \
    SERVICE="abita-middleware" \
    RUNTIME_SERVICE_ACCOUNT="runtime@test-project.iam.gserviceaccount.com" \
    IMAGE_TAG="$image_tag" \
    DEPLOYMENT_ID="test-build" \
    DEPLOY_MODE="request" \
    MAINTENANCE_OIDC_AUDIENCE="https://example.run.app" \
    MAINTENANCE_OIDC_SERVICE_ACCOUNT="scheduler@test-project.iam.gserviceaccount.com" \
    MAINTENANCE_SCHEDULE="0 */12 * * *" \
    MAINTENANCE_JOB="abita-middleware-session-maintenance" \
    bash "$repo_root/scripts/deploy-cloud-run.sh"
}

if ! run_deploy transient-once; then
  echo "Expected a transient Scheduler readiness failure to be retried." >&2
  exit 1
fi

transient_resume_count="$(
  grep -c '^scheduler jobs resume ' "$test_root/transient-once/gcloud.log"
)"
if [[ "$transient_resume_count" != "2" ]]; then
  echo "Expected two resume attempts, got $transient_resume_count." >&2
  exit 1
fi

if run_deploy permanent; then
  echo "Expected a permanent Scheduler resume failure to stop deployment." >&2
  exit 1
fi

permanent_resume_count="$(
  grep -c '^scheduler jobs resume ' "$test_root/permanent/gcloud.log"
)"
if [[ "$permanent_resume_count" != "1" ]]; then
  echo "Expected one permanent resume attempt, got $permanent_resume_count." >&2
  exit 1
fi

echo "deploy-cloud-run retry tests passed"
