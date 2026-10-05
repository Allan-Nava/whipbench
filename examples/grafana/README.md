# Grafana dashboard for a whipbench run

[whipbench-dashboard.json](whipbench-dashboard.json) is a live view of a run over its
`/metrics` endpoint (WB-17): viewers active and failed, packets per second, a delivery
shortfall estimate, one-way delay and packet transit at p50/p95/p99, and the client's own
CPU, goroutines and heap. It queries only series `/metrics` serves —
`go test ./internal/metrics/` fails when a panel names one that does not exist.

## 1. Serve the metrics

Give the run a metrics address, on the command line or in the scenario:

```bash
whipbench run --metrics 127.0.0.1:9464 examples/mediamtx-local-10.json
```

`/metrics` lives as long as the run does, so for a dashboard worth looking at give the
scenario a long `holdSeconds`.

## 2. Scrape it

Point a Prometheus at that address. A minimal `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: whipbench
    scrape_interval: 5s
    static_configs:
      - targets: ["127.0.0.1:9464"]
```

A Prometheus in Docker reaches a run on the host at `host.docker.internal:9464` (Docker
Desktop), and the run then has to listen on an address the container can reach. Several
runs scraped at once are told apart by `instance`, which the dashboard's Instance variable
selects.

## 3. Import the dashboard

Grafana 10 or later: **Dashboards → New → Import**, upload `whipbench-dashboard.json`, and
pick the Prometheus data source in the *Prometheus* variable at the top of the dashboard.

## Reading it

- **One-way delay and packet transit** are network plus server plus both stacks, not
  glass-to-glass. The quantiles are interpolated from the histogram's buckets, so they are
  coarser than the report's; the report is the figure to quote. One-way delay has samples
  only in a `run`, which publishes; packet transit only when the server forwards the
  abs-capture-time extension. Otherwise the panels show no data.
- **Delivery shortfall** is 1 − received / (sent × active viewers), from the live counters
  of a `run`. It is an estimate: retransmissions count as received and a viewer joining
  mid-interval skews it. The report's `lossPercent` is the measured loss.
- **Client CPU** is in cores busy: 1 is one core fully used. These series describe the
  whole whipbench process — publisher, viewers and reassembly — never the server. A client
  near its core count delays and drops packets itself, and its figures then describe it as
  much as the server; the report's `client.resources` gives the same figure over the run.
  On a platform with no CPU reading (anything but linux and darwin) the counter is declared
  with no sample and the panel shows no data, never a 0.
