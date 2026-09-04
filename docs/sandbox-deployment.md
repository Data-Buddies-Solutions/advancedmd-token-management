# Sandbox middleware

## Approved scope

Connect the AdvancedMD sandbox API account to a separate Cloud Run middleware
service, then route demo and staging agents to it. Verify credentials, synthetic
patient lookup, and availability only. No booking, patient creation, cancellation,
insurance update, production deployment, or production secret changes in this
setup. Agent integration is a separate PR.

One codebase, two services. Production remains unchanged. Demo and staging share
the sandbox service and its inventory, not production credentials or sessions.

## Provision and deploy

1. Verify the API credentials and application name against AdvancedMD. Admin
   website credentials are not the API credentials. Confirm that the office key
   belongs to the sandbox and that no other runtime owns this API login session.
2. Create the following secrets in the intended Google Cloud project. Never put
   values in Git, shell arguments, logs, PRs, or this document:

   | Secret | Runtime variable |
   | --- | --- |
   | `advancedmd-sandbox-username` | `ADVANCEDMD_USERNAME` |
   | `advancedmd-sandbox-password` | `ADVANCEDMD_PASSWORD` |
   | `advancedmd-sandbox-office-key` | `ADVANCEDMD_OFFICE_KEY` |
   | `advancedmd-sandbox-app-name` | `ADVANCEDMD_APP_NAME` |
   | `middleware-sandbox-api-secret` | `API_SECRET` |
   | `sandbox-booking-token-secret` | `BOOKING_TOKEN_SECRET` |

   Generate distinct random API and booking signing secrets. Grant secret access
   on these six secrets only to `abita-middleware-sandbox@PROJECT_ID.iam.gserviceaccount.com`.
   Do not grant it access to production secrets. Grant the deployer permission to
   use this identity and the selected network without expanding production IAM.
3. Create `middleware-sandbox-maintenance@PROJECT_ID.iam.gserviceaccount.com` as
   the dedicated maintenance caller identity. Do not grant it secret access. No
   Scheduler job is required initially: requests initialize/refresh the Session.
   Maintenance still requires a configured identity and the sandbox service's
   stable HTTPS base URL as its OIDC audience. For an existing sandbox service,
   read `status.url` with `gcloud run services describe abita-middleware-sandbox
   --project=PROJECT_ID --region=REGION --format='value(status.url)'`.
   For the first deployment, obtain the project number with
   `gcloud projects describe PROJECT_ID --format='value(projectNumber)'`, then use
   Google's documented deterministic URL:
   `https://abita-middleware-sandbox-PROJECT_NUMBER.REGION.run.app`.
   The service-name/project-number DNS label must be at most 63 characters; do not
   include a traffic tag. After deployment, resolve the service URL again and
   verify the configured audience is its supported stable endpoint. See
   [Cloud Run service URLs](https://docs.cloud.google.com/run/docs/triggering/https-request#deterministic).
   Never guess a hash-based hostname or reuse the production maintenance identity.
4. Build and test an immutable image from the reviewed sandbox branch. Use a
   digest, not `latest`; do not merge merely to trigger the production pipeline.
   Set the non-secret inputs below and run `bash scripts/deploy-sandbox-cloud-run.sh`:

   - `PROJECT_ID`, `REGION`
   - `SANDBOX_IMAGE`: project/region Artifact Registry `abita-middleware@sha256:...`
   - `SANDBOX_RUNTIME_SERVICE_ACCOUNT`: identity from step 2
   - `SANDBOX_NETWORK`, `SANDBOX_SUBNET`: approved existing Direct VPC network
     and subnet; preserve the provider-approved static egress path
   - `SANDBOX_MAINTENANCE_OIDC_AUDIENCE`: sandbox service HTTPS base URL
   - `SANDBOX_MAINTENANCE_OIDC_SERVICE_ACCOUNT`: identity from step 3
   - `SANDBOX_USERNAME_VERSION`, `SANDBOX_PASSWORD_VERSION`,
     `SANDBOX_OFFICE_KEY_VERSION`, `SANDBOX_APP_NAME_VERSION`,
     `SANDBOX_API_SECRET_VERSION`, `SANDBOX_BOOKING_SECRET_VERSION`: explicit
     positive integer secret versions

   The fixed target is `abita-middleware-sandbox`; the script cannot target the
   production service. It deploys `AMD_ENV=dev`, signed-token-only booking, 1 CPU,
   512 MiB, request-based billing, minimum zero and maximum one instance. It does
   not change production traffic, secrets, service accounts, or Scheduler jobs.
   Redeploy only while sandbox demos/tests are idle: revisions can briefly
   overlap, even with maximum one instance. Do not run another persistent
   middleware against the same API account.

Scale-to-zero is safe for this Session implementation: there is no background
refresh loop, and `Session.Get` performs bounded request-time login (up to 50
seconds). A cold request can exceed the agent's normal timeout. Warm with the
read-only smoke below before a scheduled demo, then start promptly; keeping an
instance warm permanently would be a separate cost decision. Existing VPC/NAT,
Secret Manager, logs, and active requests may still incur charges.

## Supported sandbox office

Use the middleware office selector `spring_hill` for sandbox calls. It resolves
to the existing dev Spring Hill facility/provider/column mappings. Confirm these
against sandbox inventory before declaring the integration ready; checked-in IDs
are not proof that inventory still exists.

Only the existing medical appointment translations are configured. Dev-mode
Crystal River production-ID placeholders and unconfigured vision translations
are rejected. Do not invent mappings or advertise unsupported specialties as
working. Production mappings are unchanged. Demo/staging agent routing must
explicitly override the middleware office selector and must fail closed if its
sandbox URL/token are missing; it must never fall back to production.

## Read-only verification gate

Keep API credentials, provider tokens, patient identifiers, and response bodies
out of logs and PRs. Use an already-provisioned, verified synthetic patient; do
not guess a real patient or create one in this setup.

1. Verify the exact deployed image digest, service account, secret names/versions,
   `AMD_ENV=dev`, network, CPU billing, and min/max instance settings. Confirm all
   service traffic goes to the intended sandbox revision.
2. `GET /live` and `GET /ready` should return 200. These prove process readiness,
   not that AdvancedMD credentials work.
3. Authenticate with the sandbox API bearer token and call
   `POST /api/patient/resolve` using
   `{"patientId":"<synthetic numeric ID>","office":"spring_hill"}`.
   Alternatively use `lastName` + `dob` for that synthetic patient. Do not combine
   `patientId` with lookup fields. Confirm the expected patient result privately.
4. Call `POST /api/scheduler/availability` with
   `{"office":"spring_hill","routing":"all_three","requestedDate":"<future YYYY-MM-DD>","dob":"<synthetic DOB>"}`.
   Confirm usable slots in the sandbox mapping, not just HTTP 200. An empty or
   provider-failed response does not prove availability works. Do not book.
5. Verify a missing API token returns 401. Connect the agent's sandbox middleware
   URL/token only after these checks succeed. Ensure production portal writes,
   notifications, and staff transfer effects are independently disabled for the
   sandbox agent route. No live call or simulation is needed for this setup gate.

Record sanitized results: image/revision, checks and outcome categories, count of
usable slots, and any unverified requirement. Do not call setup complete when
credentials work but synthetic data, mappings, or agent routing remain unverified.

Rollback is sandbox-only: stop demo/test traffic, deploy the previously verified
sandbox image digest and pinned secret versions with this script, then repeat
the read-only checks. Never fall back to the production service or credentials.
