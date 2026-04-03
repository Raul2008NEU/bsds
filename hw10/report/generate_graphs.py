#!/usr/bin/env python3
"""Generate graphs and summary tables from consistency test CSV files."""

import os
import glob
import pandas as pd
import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from collections import defaultdict

REPORT_DIR = os.path.dirname(os.path.abspath(__file__))
CSV_FILES = sorted(glob.glob(os.path.join(REPORT_DIR, "*.csv")))

# ── helpers ──────────────────────────────────────────────────────────────────

def config_from_path(path):
    """Return (config_group, full_config_label) from a CSV filename."""
    base = os.path.splitext(os.path.basename(path))[0]  # e.g. leader_w1r5_ratio10
    parts = base.rsplit("_ratio", 1)
    config_group = parts[0]          # leader_w1r5 / leaderless
    ratio        = parts[1] if len(parts) == 2 else "?"
    label        = f"{config_group} (ratio={ratio}%)"
    return config_group, ratio, label

def load_all():
    """Return a dict: label -> DataFrame, plus ordering metadata."""
    data = {}
    meta = []
    for path in CSV_FILES:
        cg, ratio, label = config_from_path(path)
        df = pd.read_csv(path, parse_dates=["timestamp"])
        df["stale"] = df["stale"].astype(str).str.lower() == "true"
        data[label] = df
        meta.append({"path": path, "config_group": cg, "ratio": int(ratio), "label": label})
    return data, meta

def percentiles(series, ps=(50, 95, 99)):
    return {p: np.percentile(series, p) for p in ps}

# ── 1 & 2.  Per-config read/write latency histograms ────────────────────────

def plot_latency_hist(df, op_type, label, out_path):
    sub = df[df["type"] == op_type]["latency_ms"].dropna()
    if sub.empty:
        return
    ps = percentiles(sub)
    fig, ax = plt.subplots(figsize=(8, 4))
    ax.hist(sub, bins=60, color="steelblue" if op_type == "read" else "darkorange",
            alpha=0.75, edgecolor="white", label=f"{op_type} latency")
    colors = {"p50": "green", "p95": "gold", "p99": "red"}
    for p, val in ps.items():
        ax.axvline(val, color=colors[f"p{p}"], linewidth=1.5, linestyle="--",
                   label=f"p{p}={val:.2f} ms")
    ax.set_xlabel("Latency (ms)")
    ax.set_ylabel("Count")
    ax.set_title(f"{op_type.capitalize()} Latency Distribution\n{label}")
    ax.legend(fontsize=8)
    plt.tight_layout()
    plt.savefig(out_path, dpi=120)
    plt.close(fig)
    print(f"  saved {os.path.basename(out_path)}")

# ── 3.  Time interval between write and next read of the same key ────────────

def plot_read_write_interval(df, label, out_path):
    """For each key, pair every write with the next read that followed it."""
    intervals = []
    for key, grp in df.groupby("key"):
        grp = grp.sort_values("timestamp")
        writes = grp[grp["type"] == "write"][["timestamp"]].copy()
        reads  = grp[grp["type"] == "read"][["timestamp"]].copy()
        if writes.empty or reads.empty:
            continue
        writes_ts = writes["timestamp"].values
        reads_ts  = reads["timestamp"].values
        # for each write, find the earliest read that comes after it
        for wt in writes_ts:
            later = reads_ts[reads_ts > wt]
            if len(later):
                delta_ms = (later[0] - wt) / np.timedelta64(1, "ms")
                intervals.append(delta_ms)

    if not intervals:
        return
    intervals = np.array(intervals)
    # clip extreme outliers for readability (keep 99th pct)
    cap = np.percentile(intervals, 99)
    clipped = intervals[intervals <= cap]

    fig, ax = plt.subplots(figsize=(8, 4))
    ax.hist(clipped, bins=60, color="mediumpurple", alpha=0.75, edgecolor="white")
    for p, color in zip((50, 95, 99), ("green", "gold", "red")):
        val = np.percentile(intervals, p)
        ax.axvline(val, color=color, linewidth=1.5, linestyle="--",
                   label=f"p{p}={val:.2f} ms")
    ax.set_xlabel("Time from write → next read of same key (ms)")
    ax.set_ylabel("Count")
    ax.set_title(f"Write→Read Interval Distribution\n{label}")
    ax.legend(fontsize=8)
    plt.tight_layout()
    plt.savefig(out_path, dpi=120)
    plt.close(fig)
    print(f"  saved {os.path.basename(out_path)}")

# ── 4.  Stale-read bar chart across all configs ──────────────────────────────

def plot_stale_bar(data, meta, out_path):
    # group by config_group, then ratio
    grouped = defaultdict(dict)
    for m in meta:
        lbl   = m["label"]
        count = int(data[lbl]["stale"].sum())
        total = int((data[lbl]["type"] == "read").sum())
        pct   = 100.0 * count / total if total else 0
        grouped[m["config_group"]][m["ratio"]] = (count, pct)

    config_groups = sorted(grouped.keys())
    ratios        = sorted({m["ratio"] for m in meta})
    x             = np.arange(len(config_groups))
    width         = 0.18
    offsets       = np.linspace(-(len(ratios)-1)/2, (len(ratios)-1)/2, len(ratios)) * width

    fig, ax = plt.subplots(figsize=(11, 5))
    palette = plt.cm.tab10(np.linspace(0, 0.7, len(ratios)))
    for i, (ratio, offset, color) in enumerate(zip(ratios, offsets, palette)):
        counts = [grouped[cg].get(ratio, (0, 0))[0] for cg in config_groups]
        bars = ax.bar(x + offset, counts, width, label=f"ratio={ratio}%", color=color)
        for bar, cnt in zip(bars, counts):
            if cnt > 0:
                ax.text(bar.get_x() + bar.get_width()/2, bar.get_height() + 0.5,
                        str(cnt), ha="center", va="bottom", fontsize=7)

    ax.set_xticks(x)
    ax.set_xticklabels(config_groups, rotation=15, ha="right")
    ax.set_ylabel("Stale Read Count")
    ax.set_title("Stale Read Count Across All Configurations")
    ax.legend(title="Write ratio", fontsize=8)
    plt.tight_layout()
    plt.savefig(out_path, dpi=120)
    plt.close(fig)
    print(f"  saved {os.path.basename(out_path)}")

# ── summary table ─────────────────────────────────────────────────────────────

def print_summary(data, meta):
    rows = []
    for m in sorted(meta, key=lambda x: (x["config_group"], x["ratio"])):
        df  = data[m["label"]]
        rd  = df[df["type"] == "read"]["latency_ms"].dropna()
        wr  = df[df["type"] == "write"]["latency_ms"].dropna()
        stale_cnt   = int(df["stale"].sum())
        total_reads = int((df["type"] == "read").sum())
        rows.append({
            "config":       m["config_group"],
            "ratio":        m["ratio"],
            "stale_reads":  stale_cnt,
            "stale_%":      f"{100*stale_cnt/total_reads:.2f}" if total_reads else "—",
            "r_avg":        f"{rd.mean():.2f}" if not rd.empty else "—",
            "r_p50":        f"{np.percentile(rd,50):.2f}" if not rd.empty else "—",
            "r_p95":        f"{np.percentile(rd,95):.2f}" if not rd.empty else "—",
            "r_p99":        f"{np.percentile(rd,99):.2f}" if not rd.empty else "—",
            "w_avg":        f"{wr.mean():.2f}" if not wr.empty else "—",
            "w_p50":        f"{np.percentile(wr,50):.2f}" if not wr.empty else "—",
            "w_p95":        f"{np.percentile(wr,95):.2f}" if not wr.empty else "—",
            "w_p99":        f"{np.percentile(wr,99):.2f}" if not wr.empty else "—",
        })

    # column widths
    cols = list(rows[0].keys())
    widths = {c: max(len(c), max(len(str(r[c])) for r in rows)) for c in cols}
    sep  = "+" + "+".join("-" * (widths[c]+2) for c in cols) + "+"
    hdr  = "|" + "|".join(f" {c.center(widths[c])} " for c in cols) + "|"
    print("\n" + sep)
    print(hdr)
    print(sep)
    for r in rows:
        print("|" + "|".join(f" {str(r[c]).center(widths[c])} " for c in cols) + "|")
    print(sep + "\n")

# ── main ──────────────────────────────────────────────────────────────────────

def main():
    print(f"Loading {len(CSV_FILES)} CSV files …")
    data, meta = load_all()

    print("\nGenerating per-config read/write latency histograms …")
    for m in meta:
        df    = data[m["label"]]
        slug  = f"{m['config_group']}_ratio{m['ratio']}"
        plot_latency_hist(df, "read",  m["label"],
                          os.path.join(REPORT_DIR, f"{slug}_read_latency.png"))
        plot_latency_hist(df, "write", m["label"],
                          os.path.join(REPORT_DIR, f"{slug}_write_latency.png"))

    print("\nGenerating write→read interval histograms …")
    for m in meta:
        df   = data[m["label"]]
        slug = f"{m['config_group']}_ratio{m['ratio']}"
        plot_read_write_interval(df, m["label"],
                                 os.path.join(REPORT_DIR, f"{slug}_wr_interval.png"))

    print("\nGenerating stale-read comparison bar chart …")
    plot_stale_bar(data, meta, os.path.join(REPORT_DIR, "stale_reads_comparison.png"))

    print("\nSummary table (all latencies in ms):")
    print_summary(data, meta)

if __name__ == "__main__":
    main()
