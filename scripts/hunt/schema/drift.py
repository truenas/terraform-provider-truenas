#!/usr/bin/env python3
"""Version-drift report between two core.get_methods dumps (dump-methods.sh).

For every *.create / *.update method the provider calls, list the accepted
fields present in OLD but absent in NEW (dropped) and vice versa (new). Each
dropped field is grepped in the provider source: if the provider still sends
it unconditionally, applying on NEW fails with "Extra inputs are not
permitted" unless it is version-gated. New fields are coverage gaps.

usage: drift.py <old-methods.json> <new-methods.json>
  e.g. drift.py .hunt-work/schema/methods-192.0.2.49.json .hunt-work/schema/methods-192.0.2.70.json
Exit status is the number of dropped fields the provider still references (capped at 99).
"""
import json, os, re, subprocess, sys

if len(sys.argv) != 3 or sys.argv[1] in ("-h", "--help"):
    print(__doc__.strip()); sys.exit(0 if len(sys.argv) > 1 and sys.argv[1] in ("-h", "--help") else 2)
oldfile, newfile = sys.argv[1:3]
OLD = re.sub(r"^methods-|\.json$", "", os.path.basename(oldfile))
NEW = re.sub(r"^methods-|\.json$", "", os.path.basename(newfile))
old = json.load(open(oldfile))
new = json.load(open(newfile))
HERE = os.path.dirname(os.path.abspath(__file__))
repo = subprocess.run(["git", "-C", HERE, "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()

def _walk(node, defs, out, depth=0):
    """Collect property names across properties/anyOf/oneOf/allOf/$ref."""
    if not isinstance(node, dict) or depth > 6:
        return
    if "$ref" in node:
        ref = node["$ref"].split("/")[-1]
        _walk(defs.get(ref, {}), defs, out, depth + 1)
    out |= set((node.get("properties") or {}).keys())
    for key in ("anyOf", "oneOf", "allOf"):
        for alt in node.get(key) or []:
            _walk(alt, defs, out, depth + 1)

def fields(meth):
    out = set()
    for a in meth.get("accepts") or []:
        defs = (a or {}).get("$defs") or meth.get("$defs") or {}
        _walk(a, defs, out)
    return out

# methods the provider actually calls
src = subprocess.run(["grep", "-rhoE", r'"[a-z_]+(\.[a-z_]+)+\.(create|update)"', f"{repo}/internal/resources", f"{repo}/internal/client"],
                     capture_output=True, text=True).stdout
called = sorted(set(s.strip('"') for s in src.split()))

def provider_mentions(field):
    r = subprocess.run(["grep", "-rlE", rf'"{field}"', f"{repo}/internal/resources"], capture_output=True, text=True)
    return sorted(os.path.relpath(p, repo) for p in r.stdout.split())

print(f"# provider-called create/update methods: {len(called)}  (old={OLD} new={NEW})\n")
dropped_total = 0
for m in called:
    if m not in old or m not in new:
        print(f"## {m}: present on {OLD}={m in old} {NEW}={m in new}")
        continue
    fo, fn = fields(old[m]), fields(new[m])
    dropped, added = sorted(fo - fn), sorted(fn - fo)
    if not dropped and not added:
        continue
    print(f"## {m}")
    for f in dropped:
        where = provider_mentions(f)
        flag = "SENT BY PROVIDER" if where else "not in provider"
        dropped_total += 1 if where else 0
        print(f"   DROPPED on {NEW}: {f}  [{flag}] {where[:3]}")
    for f in added:
        where = provider_mentions(f)
        print(f"   NEW on {NEW}:     {f}  [{'provider has it' if where else 'COVERAGE GAP'}]")
print(f"\n# dropped-on-{NEW} fields the provider still references: {dropped_total}")
sys.exit(min(dropped_total, 99))
