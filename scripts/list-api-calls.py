#!/usr/bin/env python3
"""Enumerate every TrueNAS JSON-RPC method this provider calls.

Scans Go source for the client call entrypoints — Call / CallJob / CallRead —
and extracts the method-name string literal (the argument after ctx). Emits a
grouped Markdown inventory (API-CALLS.md) used to detect upstream API changes
that would affect the provider.

Call variant is recorded because it encodes intent:
  Call     -> write/action (create/update/delete or a plain method)
  CallJob  -> long-running job method (returns a job id, polled)
  CallRead -> read/query (query/get_instance/config/get_methods)

Usage: python3 scripts/list-api-calls.py [--check]
  (no args)  write API-CALLS.md
  --check    exit 1 if API-CALLS.md is stale (for CI)
"""
import os
import re
import sys
from collections import defaultdict

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCAN_DIRS = ["internal", "cmd"]
OUT = os.path.join(ROOT, "API-CALLS.md")

# --- TrueNAS API version coverage -------------------------------------------
# The provider speaks JSON-RPC 2.0 over the rolling /api/current WebSocket
# endpoint. BASELINE_RELEASE is the oldest release the provider targets;
# VERIFIED_RELEASES are the releases acceptance-tested against live systems.
API_ENDPOINT = "/api/current"
BASELINE_RELEASE = "25.04"
VERIFIED_RELEASES = ("25.10", "26.0")
# Method namespaces absent on releases below 26.0 (the whole namespace does
# not exist earlier; a call fails with method-not-found). Keep in sync with
# the whole-resource version floors in internal/resources/*/model.go
# ("requires TrueNAS 26.0 or later"): container, lxc, webshare, sharing.webshare.
GATED_26_0_NS = {"container", "lxc", "webshare"}


def method_since(method):
    """Oldest TrueNAS release exposing this method."""
    if method.split(".", 1)[0] in GATED_26_0_NS or method.startswith(
        "sharing.webshare."
    ):
        return "26.0"
    return BASELINE_RELEASE

# .Call / .CallJob / .CallRead ( <ctx-arg> , "method" ...
# DOTALL so the method literal may sit on the next line after the ctx arg.
CALL_RE = re.compile(
    r'\.(Call|CallJob|CallRead)\(\s*[^,]+?,\s*"([a-zA-Z0-9_.]+)"',
    re.DOTALL,
)

# A bare string literal shaped like a namespaced method: two+ dotted segments,
# lowercase/underscore. Used for the secondary sweep (methods referenced via
# consts/maps/subscriptions, not directly at a Call site).
METHODISH_RE = re.compile(r'"([a-z][a-z0-9_]*(?:\.[a-z0-9_]+){1,})"')

# Segments that look method-ish but are not API methods (paths, mime, hosts).
NOT_METHOD_HINTS = ("/", " ", "://")
# Final-segment tokens that mark a hostname, not a method (docker.io, *.lan).
HOST_TLDS = {"io", "com", "net", "org", "lan", "example", "invalid"}


def go_files():
    for d in SCAN_DIRS:
        base = os.path.join(ROOT, d)
        for dirpath, _, names in os.walk(base):
            for n in names:
                if n.endswith(".go"):
                    yield os.path.join(dirpath, n)


def rel(p):
    return os.path.relpath(p, ROOT)


def main():
    # method -> {"variants": set, "prod": set(files), "test": set(files)}
    calls = defaultdict(lambda: {"variants": set(), "prod": set(), "test": set()})
    methodish = defaultdict(set)  # method -> set(files) for secondary sweep

    for path in go_files():
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
        is_test = path.endswith("_test.go")
        r = rel(path)

        for variant, method in CALL_RE.findall(src):
            e = calls[method]
            e["variants"].add(variant)
            e["test" if is_test else "prod"].add(r)

        # Secondary sweep only over non-test source: test files are full of
        # Terraform addresses, HCL attribute paths, and fixture hostnames that
        # look method-shaped but are not API methods.
        if is_test:
            continue
        for m in METHODISH_RE.findall(src):
            if any(h in m for h in NOT_METHOD_HINTS):
                continue
            methodish[m].add(r)

    # Group by top-level namespace (first dotted segment).
    def ns(method):
        return method.split(".", 1)[0]

    by_ns = defaultdict(list)
    for method in calls:
        by_ns[ns(method)].append(method)

    # Secondary: method-ish literals never seen at a Call site (referenced
    # indirectly, e.g. the debug_api probe binary or file-transfer helpers).
    # Keep only strings anchored on a real API namespace, with no numeric
    # segment (HCL list index) and no hostname TLD tail.
    known_ns = set(by_ns)

    def is_real_indirect(m):
        segs = m.split(".")
        if segs[0] not in known_ns:
            return False
        if segs[-1] in HOST_TLDS:
            return False
        if any(s.isdigit() for s in segs):
            return False
        return True

    indirect = {
        m: fs
        for m, fs in methodish.items()
        if m not in calls and is_real_indirect(m)
    }

    total_methods = len(calls)
    total_ns = len(by_ns)
    prod_methods = sum(1 for m, e in calls.items() if e["prod"])

    lines = []
    lines.append("# TrueNAS API Call Inventory")
    lines.append("")
    lines.append(
        "Every JSON-RPC method this provider invokes, extracted from the source "
        "by `scripts/list-api-calls.py`. Regenerate with `make api-calls`. Use it "
        "to catch upstream TrueNAS API changes that affect the provider: diff this "
        "file against a new release's method set."
    )
    lines.append("")
    lines.append(
        f"**{total_methods}** distinct methods across **{total_ns}** namespaces "
        f"({prod_methods} reached from non-test code)."
    )
    lines.append("")

    gated = sorted(m for m in calls if method_since(m) == "26.0")
    lines.append("## API version covered")
    lines.append("")
    lines.append(
        f"- **Endpoint:** `{API_ENDPOINT}` — JSON-RPC 2.0 over WebSocket "
        f"(the rolling API alias; not a pinned schema version)."
    )
    lines.append(
        f"- **Baseline release:** TrueNAS **{BASELINE_RELEASE}** — the oldest "
        f"release the provider targets. Methods without a `Since` mark below "
        f"exist at this baseline."
    )
    lines.append(
        f"- **Verified against:** TrueNAS "
        + " and ".join(f"**{v}**" for v in VERIFIED_RELEASES)
        + " (live acceptance-tested)."
    )
    lines.append(
        f"- **26.0-only methods:** {len(gated)} methods require TrueNAS "
        f"**26.0** or later (`Since` = 26.0 below): the `container`, `lxc`, "
        f"`webshare`, and `sharing.webshare` namespaces do not exist earlier. "
        f"SCRAM-SHA-512 API-key auth is also 26.0+ (a login mechanism, not a "
        f"distinct method)."
    )
    lines.append("")
    lines.append(
        "To catch API changes, diff the method set below against a target "
        "release's `core.get_methods` output. A method that moves from present "
        "to absent (or changes namespace) breaks the provider on that release."
    )
    lines.append("")
    lines.append("Variant legend: `Call` = write/action · `CallJob` = job "
                 "(async, polled) · `CallRead` = read/query. `Since` = oldest "
                 "release exposing the method (blank = baseline "
                 f"{BASELINE_RELEASE}).")
    lines.append("")
    lines.append("| Method | Variant(s) | Since | Prod | Test-only |")
    lines.append("|---|---|:--:|:--:|:--:|")

    for namespace in sorted(by_ns):
        for method in sorted(by_ns[namespace]):
            e = calls[method]
            variants = " ".join(
                v for v in ("Call", "CallJob", "CallRead") if v in e["variants"]
            )
            since = method_since(method)
            since_col = since if since != BASELINE_RELEASE else ""
            prod = "yes" if e["prod"] else ""
            test_only = "yes" if (e["test"] and not e["prod"]) else ""
            lines.append(
                f"| `{method}` | {variants} | {since_col} | {prod} | {test_only} |"
            )

    lines.append("")
    lines.append("## Namespaces")
    lines.append("")
    for namespace in sorted(by_ns):
        methods = sorted(by_ns[namespace])
        lines.append(f"- **`{namespace}`** ({len(methods)}): "
                     + ", ".join(f"`{m}`" for m in methods))
    lines.append("")

    if indirect:
        lines.append("## Method-shaped literals not at a direct call site")
        lines.append("")
        lines.append(
            "Namespaced-method-looking strings referenced indirectly (constants, "
            "maps, subscription channels, probes). Review — some are real methods "
            "invoked via a wrapper, some are false positives (e.g. attribute keys)."
        )
        lines.append("")
        for m in sorted(indirect):
            files = ", ".join(f"`{f}`" for f in sorted(indirect[m]))
            lines.append(f"- `{m}` — {files}")
        lines.append("")

    content = "\n".join(lines)

    if "--check" in sys.argv:
        existing = ""
        if os.path.exists(OUT):
            with open(OUT, encoding="utf-8") as fh:
                existing = fh.read()
        if existing.rstrip("\n") != content.rstrip("\n"):
            print("API-CALLS.md is stale. Run: make api-calls", file=sys.stderr)
            sys.exit(1)
        print("API-CALLS.md up to date.")
        return

    with open(OUT, "w", encoding="utf-8") as fh:
        fh.write(content + "\n")
    print(f"Wrote {rel(OUT)}: {total_methods} methods, {total_ns} namespaces "
          f"({prod_methods} prod).")


if __name__ == "__main__":
    main()
