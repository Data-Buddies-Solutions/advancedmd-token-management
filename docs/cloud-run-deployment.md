# Cloud Run deployment

Production deploys are automatic after the required tests and image build
succeed. Each `main` build is one sequential pipeline, so a failed test, build,
image push, configuration check, revision startup, or traffic update cannot be
reported as a successful release.

## One-time Scheduler setup

Token maintenance uses a dedicated Cloud Scheduler service account and a
Google-signed OIDC ID token. It never uses `API_SECRET`, AdvancedMD credentials,
or a raw AdvancedMD token.

Before merging the first request-based release, an operator must:

1. Enable the Cloud Scheduler API.
2. Create a dedicated service account in `acuity-health-prod` for this job. Do
   not use the Cloud Scheduler service agent or the Cloud Run runtime identity.
3. Preserve `roles/cloudscheduler.serviceAgent` on the Google-managed Cloud
   Scheduler service agent.
4. Grant the Cloud Build deploy identity `roles/cloudscheduler.admin` in the
   project and `roles/iam.serviceAccountUser` on the dedicated Scheduler
   service account. Scope each grant to the narrowest supported boundary.
5. Configure both production Cloud Build triggers with:
   - `_MAINTENANCE_OIDC_AUDIENCE`: the stable HTTPS Cloud Run service base URL,
     with no path, query, fragment, or trailing slash.
   - `_MAINTENANCE_OIDC_SERVICE_ACCOUNT`: the exact dedicated service-account
     email.
   - `_MAINTENANCE_SCHEDULE`: a UTC `0 */N * * *` expression where `N` is
     between 1 and 19. The repository default is every 12 hours, and the deploy
     script rejects an interval at or beyond the proven 20-hour stale threshold.
   - `_MAINTENANCE_JOB`: the stable Scheduler job name.

The checked-in `DO_NOT_DEPLOY` defaults fail before any Cloud Run change. This
prevents a merge from inventing an identity or deploying a maintenance route
with an incomplete authorization contract.

Cloud Run retains public transport access because the external agent does not
present Google IAM identity. `/api/*` remains protected by `API_SECRET`, while
`/ops/session/maintenance` independently validates the Google signature,
service-account issuer, audience, verified email claim, and exact Scheduler
service-account email.

## Built-in latency and request logs

Use Cloud Run's built-in `run.googleapis.com/request_latencies` distribution,
filtered to the `abita-middleware` service, for overall service p95 latency.
This repository does not add a custom, user-defined, or log-based latency
metric.

All process logs are emitted as one JSON object per line. Existing lifecycle
events retain their safe categorized message, while each completed HTTP request
uses exactly these fields:

- `request_id`: the generated request ID or the existing hashed external value
- `route_template`: the matched Chi template, or `unmatched`; never the raw URL
- `outcome_category`: a bounded application or transport outcome
- `latency_ms`: elapsed application time in milliseconds
- `session_state`: the existing safe Session lifecycle state
- `provider_failure_category`: a stable safe category, or `none`

Do not turn `request_id` or any other unique value into a metric label. Request
and response bodies, patient and appointment identifiers, names, dates of
birth, phones, credentials, tokens, provider URLs, and raw error strings do not
belong in logs.

## One-time `/live` uptime check

`monitoring/live-uptime-check.flags.yaml` defines one public HTTPS `GET /live`
check: HTTP 200, TLS validation, a 15-minute period, 10-second timeout, and the
minimum three regions. It defines no notification channel or alert policy.
Three regions every 15 minutes produce at most 8,928 regional executions in a
31-day month.

Before creating the check:

1. Confirm `gcloud` is authenticated to the intended operator account and that
   the active project is `acuity-health-prod`.
2. List every existing check with
   `gcloud monitoring uptime list-configs --project=acuity-health-prod`.
3. Verify current-month uptime-check execution usage and the active
   configurations remain below Cloud Monitoring's free monthly allowance after
   adding 8,928 executions. If cost cannot be proven, stop.
4. Read the public service host from
   `gcloud run services describe abita-middleware --project=acuity-health-prod --region=us-east4`.
   Do not guess or check in the generated host.

After those gates pass, set `SERVICE_HOST` to the hostname only, without
`https://` or a path, and run:

```bash
gcloud monitoring uptime create "abita-middleware /live" \
  --project=acuity-health-prod \
  --resource-type=uptime-url \
  --resource-labels="host=${SERVICE_HOST},project_id=acuity-health-prod" \
  --flags-file=monitoring/live-uptime-check.flags.yaml
```

List the configuration again and verify the path, period, regions, TLS
validation, and HTTP status. Do not create an alert policy or notification
channel.

## Automatic production pipeline

The `abita-middleware-main-build` trigger runs for every push to `main`:

1. Run the Go tests.
2. Build the production `Dockerfile`.
3. Push an immutable image tagged with the full Git commit SHA.
4. Resolve that tag to an immutable image digest and validate the deployment
   mode and maintenance identity configuration.
5. Deploy one build-owned revision with zero traffic and verify it references
   that exact digest.
6. Create or update the authenticated Scheduler job in a paused state.
7. Move traffic directly to that one revision at 100%; no gradual split is
   used.
8. Resume the Scheduler job. A newly created job can briefly report that its
   backend parent is not ready, so the deploy script retries only that transient
   response for up to 25 seconds. Permission and configuration failures still
   stop immediately.

Schedule the merge or manual production trigger during a lower-traffic window
when practical. The direct cutover is intentional, but in-flight requests may
still finish on the prior revision.

The production revision uses:

- region `us-east4`
- 1 vCPU and 512 MiB memory
- request-based billing (`--cpu-throttling`)
- minimum 1 and maximum 1 instance
- concurrency 20 and a 60-second request timeout
- `/live` startup and `/ready` readiness probes
- Direct VPC egress through `acuity-prod` and `cloud-run-us-east4`
- public transport access with application-level route authentication
- explicit Secret Manager versions
- startup CPU boost

The runtime service account reads the pinned Secret Manager versions when the
container starts. The deploy identity does not need to print or read those
secret values.

## Deployment smoke

Run these checks after direct cutover. Keep bearer tokens and any response body
out of shell tracing, shared terminals, build logs, and issue comments.

1. Confirm the service reports exactly one revision at 100% traffic, minimum
   one instance, maximum one instance, and CPU throttling enabled.
2. Confirm `GET /live` returns `{"status":"ok"}`.
3. Confirm `GET /ready` returns `{"status":"ready"}`.
4. Invoke the Scheduler job on demand, or mint an OIDC token for the configured
   Scheduler service account and audience without printing it. Confirm
   `POST /ops/session/maintenance` returns `204 No Content`.
5. Confirm the same route returns `401` for a missing token and for the agent
   API bearer credential.
6. Resolve the provisioned synthetic, read-only test patient through
   `POST /api/patient/resolve`. Confirm HTTP 200 and the expected application
   result without copying the response body into logs. Do not substitute a real
   patient.
7. Confirm the request log contains exactly the six fields documented under
   "Built-in latency and request logs." It must not contain request/response
   bodies, patient identifiers, credentials, tokens, or AdvancedMD URLs.

If the Scheduler invocation is delayed or fails, the synthetic patient request
must still initialize or refresh the Session through the bounded request-time
fallback. A failed proactive invocation is therefore observable but does not
make background CPU correctness-critical.

## Manual redeploy

Run the `abita-middleware-production-deploy` manual trigger with
`_IMAGE_TAG=<full 40-character commit SHA>` and `_DEPLOY_MODE=request` to
redeploy a post-#128 image that the automatic pipeline already built. The
trigger rejects mutable tags and abbreviated SHAs, deploys with zero traffic,
prepares maintenance, then cuts directly to the build-owned revision at 100%.

## Rollback

For a known-good image that includes #128, run the manual trigger with its full
commit SHA and `_DEPLOY_MODE=request`.

For an image from before #128, run the same trigger with its full commit SHA and
`_DEPLOY_MODE=legacy-rollback`. That mode:

- restores instance-based billing with always-allocated CPU,
- retains minimum one and maximum one instance,
- moves traffic directly to the rollback revision at 100%, and
- pauses the Scheduler job because the legacy image has no authenticated
  maintenance route and still owns the background refresh loop.

After either rollback, verify one revision has 100% traffic, `GET /health`
passes, and a synthetic read-only patient resolve succeeds. For a legacy
rollback, also verify the Scheduler job is paused and the revision reports CPU
throttling disabled.

If Cloud Run itself is unavailable, reconnect and restart Railway, restore the
Railway URL in the agent, and verify `/health`. Never leave Railway and Cloud
Run running as long-lived AdvancedMD token owners.

## Evidence record

Do not run a deployment or rollback solely to fill this record. After one
separately approved deployment smoke and one rollback exercise, record:

- UTC start/end time, operator, full image commit SHA, and revision name
- the single revision receiving 100% traffic and its min/max/CPU settings
- `/live`, `/ready`, maintenance, and synthetic read-only patient smoke results
  without response bodies or identifiers
- the JSON request-log fields inspected and confirmation that no PHI, secret,
  raw provider URL, or raw provider error appeared
- the rollback image commit SHA and revision, direct 100% traffic result,
  health results, and Scheduler state required by the rollback mode

Keep credentials, tokens, patient data, appointment data, and response bodies
out of the evidence.

References:

- [Cloud Scheduler OIDC authentication](https://cloud.google.com/scheduler/docs/http-target-auth)
- [Cloud Monitoring uptime-check pricing](https://cloud.google.com/products/observability/pricing#uptime-checks)
- [Cloud Run built-in metrics](https://cloud.google.com/monitoring/api/metrics_gcp_p_z)
- [Cloud Run billing settings](https://cloud.google.com/run/docs/configuring/billing-settings)
- [Cloud Run traffic migration and rollback](https://cloud.google.com/run/docs/rollouts-rollbacks-traffic-migration)
