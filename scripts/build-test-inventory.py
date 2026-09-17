#!/usr/bin/env python3
"""Generate TEST-INVENTORY.md: every automated test function in the repo,
grouped by package, tagged by tier/gate, and described from its doc comment.

Derived mechanically from the *_test.go sources so it stays accurate. Run
`make test-inventory` (or python3 scripts/build-test-inventory.py) after
adding or renaming tests. Complements TEST-PLAN.md (strategy) and TESTING.md
(operator reference) with the per-test enumeration layer."""
import os
import re

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "TEST-INVENTORY.md")

FUNC_RE = re.compile(r'^func (Test\w+)\(t \*testing\.T\)', re.M)


def classify(name: str, body: str) -> str:
    """Return the tier/gate label for a test, from its name and body."""
    if name.startswith("TestLive"):
        return "Live (client)"
    if name.startswith("TestAcc"):
        # Gate detection, most specific first.
        if "HACheck" in body:
            return "Acceptance · HA gate"
        if "DSCheck" in body or "dsPreCheck" in body or "TRUENAS_DS" in body:
            return "Acceptance · DS gate"
        if "AppsCheck" in body:
            return "Acceptance · Apps gate"
        if "DisruptiveCheck" in body:
            return "Acceptance · Tier 2 (disruptive)"
        # Self-skips: the test guards itself with an early t.Skip when a
        # required fixture/endpoint/env is absent. Includes the permanently
        # skipped create-validates-a-real-endpoint tests (app_registry,
        # cloud_backup, vmware) and fixture/env-gated ones.
        head = body[:400]
        if re.search(r't\.Skip\w*\(', head):
            return "Acceptance · conditional skip"
        return "Acceptance · Tier 1"
    return "Unit"


def doc_comment(src: str, func_start: int) -> str:
    """Extract the contiguous // comment block immediately above func_start."""
    lines = src[:func_start].rstrip("\n").split("\n")
    out = []
    for ln in reversed(lines):
        s = ln.strip()
        if s.startswith("//"):
            out.append(s[2:].strip())
        elif s == "":
            break
        else:
            break
    out.reverse()
    text = " ".join(out).strip()
    # First sentence, trimmed.
    m = re.match(r'(.+?[.。])(\s|$)', text)
    if m:
        text = m.group(1)
    return text


def pkg_of(path: str) -> str:
    rel = os.path.relpath(path, ROOT)
    d = os.path.dirname(rel)
    return d


def main() -> int:
    packages = {}  # pkgdir -> list[(name, tier, desc)]
    for dp, dns, fns in os.walk(ROOT):
        if os.path.relpath(dp, ROOT).split(os.sep)[0] in {".git", "vendor"}:
            dns[:] = []
            continue
        for fn in fns:
            if not fn.endswith("_test.go"):
                continue
            p = os.path.join(dp, fn)
            src = open(p, encoding="utf-8").read()
            for m in FUNC_RE.finditer(src):
                name = m.group(1)
                # crude body span: from func to next top-level func or EOF
                nxt = FUNC_RE.search(src, m.end())
                body = src[m.start(): nxt.start() if nxt else len(src)]
                tier = classify(name, body)
                desc = doc_comment(src, m.start())
                packages.setdefault(pkg_of(p), []).append((name, tier, desc))

    # Totals
    all_tests = [(pk, t) for pk, lst in packages.items() for t in lst]
    total = len(all_tests)
    by_tier = {}
    for _, (n, tier, d) in all_tests:
        by_tier[tier] = by_tier.get(tier, 0) + 1

    lines = []
    W = lines.append
    W("# Automated Test Inventory\n")
    W("Every automated test function in the repository, grouped by package "
      "and tagged by tier/gate. Generated from the `*_test.go` sources by "
      "`scripts/build-test-inventory.py` (`make test-inventory`) — regenerate "
      "after adding or renaming tests. See `TEST-PLAN.md` for the testing "
      "strategy and `TESTING.md` for how to run each tier.\n")
    W(f"**Total: {total} test functions** across {len(packages)} packages.\n")
    W("| Tier / gate | Count |")
    W("|---|---|")
    for tier in sorted(by_tier):
        W(f"| {tier} | {by_tier[tier]} |")
    W("")
    W("Tier legend: **Unit** needs no server (pure functions). "
      "**Acceptance · Tier 1** creates and destroys its own objects. "
      "**Tier 2 (disruptive)** mutates a singleton and restores it. "
      "**DS / HA / Apps gate** needs a directory server / HA system / app "
      "catalog and a matching env gate. **conditional skip** self-skips when "
      "a required fixture/endpoint/env is absent — including the permanently "
      "skipped tests whose `create` validates against a real remote endpoint "
      "(app_registry, cloud_backup, vmware), each with re-enable steps in the "
      "test file. **Live (client)** exercises the WebSocket client against a "
      "real box.\n")
    W("---\n")

    for pk in sorted(packages):
        tests = sorted(packages[pk], key=lambda x: (x[1], x[0]))
        name = pk if pk else "(repo root)"
        W(f"## `{name}`  ({len(tests)})\n")
        W("| Test | Tier / gate | Covers |")
        W("|---|---|---|")
        for n, tier, d in tests:
            d = d.replace("|", "\\|")
            W(f"| `{n}` | {tier} | {d} |")
        W("")

    open(OUT, "w", encoding="utf-8").write("\n".join(lines) + "\n")
    print(f"wrote {OUT}: {total} tests, {len(packages)} packages")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
