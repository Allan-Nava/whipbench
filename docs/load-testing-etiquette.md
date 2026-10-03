# Load-testing etiquette

The rule whipbench is run under, and what a run against a managed service has to record
before its numbers are published ([WB-6](../BACKLOG.md)). The README states the rule in
one line; this page is the longer form.

This page is not legal advice, and it does not say what any provider's terms allow. Read
the terms of the plan you are on; when they are unclear about load testing, ask the
provider and keep the answer in writing.

## The rule

Run whipbench only against:

1. **a server you run** — your own MediaMTX, OvenMediaEngine, LiveKit, Janus or anything
   else, on machines you control;
2. **a managed service on your own account, within its terms** — the account is yours,
   and the provider's terms for your plan permit what the run does;
3. **anything else only with written permission** from whoever runs it.

A load test opens many sessions from one machine on a schedule and holds them. To the
service on the other end that is indistinguishable from an attack, whatever the intent.
Nothing in this repository is measured outside the rule: the live runs in
[`evals/`](../evals/) are against local servers only.

## Before a run against a managed service

- **Read the terms for your plan** and note what they say about load or performance
  testing. If they ask for notice before a test, give it.
- **Size the run inside the plan.** A plan states its own limits — concurrent sessions,
  egress, sometimes a rate of new sessions. A run past them measures the limits, not
  the service.
- **Use a stream on your own account made for the test**, and put its token in an
  environment variable named by `bearerEnv` (or `--bearer-env` on `view`), never in the
  scenario file. The report keeps the endpoint's host and nothing else, as it does for
  every run.
- **Watch it.** Ctrl-C stops a run; an interrupted run is reported with no verdict.

## What the run records

A run against a managed service goes in `evals/YYYY-MM-DD-<what>.md`, like any other run
worth keeping, with the raw reports beside it. On top of the server version and host
every evals file carries, it records:

| field | what to write |
|---|---|
| service | the provider and the product, as the provider names them |
| plan | the plan or tier the account is on, and the limits it states that bear on the run (concurrent sessions, egress), with the date they were read |
| region | the region or point of presence the endpoints belong to, as the provider reports it, and where the client machine ran |
| permission | which case of the rule the run falls under. Own account: that the account is yours, and which terms were read (their date or version). Written permission: who gave it (a role and an organisation, not a personal address), when, and what it covers — endpoints, viewer counts, the time window |
| window | when the run happened, in UTC, and how many viewers for how long |

Never record the account's id, a stream key, a token, an invoice or the permission
message itself: record that the permission exists and what it covers. If the endpoint's
host names the account, replace it in the evals file and say that it was replaced.

No managed-service number is published without these fields. [WB-23](../BACKLOG.md) is
the first run planned under this rule.
