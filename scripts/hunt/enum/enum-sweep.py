#!/usr/bin/env python3
"""Enum-value sweep: for one resource, apply EVERY enum value the API schema
lists for each enum attribute (in the API's own casing), then plan again.
  empty plan   -> value round-trips
  plan diff    -> FINDING (provider normalizes/reads back differently)
  apply error  -> noted (API rejected; version gate / validator candidate)

usage: enum-sweep.py <ip> <api-method> <variant-index|-> <tf-type> <base-hcl-file> <attr-map-json>
  ip             box whose env.<ip>.sh (in $TFHUNT_ENV_DIR, default <repo>/.hunt-env)
                 and <work>/schema/methods-<ip>.json (dump-methods.sh) are used
  api-method     e.g. pool.dataset.create
  variant-index  index into the method's oneOf/anyOf accepts variants, or - for none
  tf-type        e.g. truenas_dataset (names the run directory)
  base-hcl-file  HCL with __SUFFIX__ and __EXTRA__ placeholders (see *-base.tf here)
  attr-map-json  {"api_field": "hcl_attr", ...} - only listed fields are swept;
                 a value of ["hcl_attr", [v1, v2]] sweeps explicit values instead
                 of the schema enum (for non-enum fields with aliases)

Drives `terraform` with the CURRENT provider build via dev_overrides
(<work>/upgrade/bin/current, from upgrade/build-versions.sh). Work dir is
$TFHUNT_WORK_DIR (default <repo>/.hunt-work).

example:
  enum-sweep.py 192.0.2.70 pool.dataset.create - truenas_dataset \\
    scripts/hunt/enum/dataset-base.tf '{"compression":"compression","sync":"sync"}'
"""
import json, os, re, subprocess, sys, random

if len(sys.argv) != 7 or sys.argv[1] in ("-h", "--help"):
    print(__doc__.strip()); sys.exit(0 if len(sys.argv) > 1 and sys.argv[1] in ("-h", "--help") else 2)
ip, method, variant, tftype, basefile, attrmap_json = sys.argv[1:7]

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = subprocess.run(["git", "-C", HERE, "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()
ENV_DIR = os.environ.get("TFHUNT_ENV_DIR", os.path.join(REPO, ".hunt-env"))
WORK = os.environ.get("TFHUNT_WORK_DIR", os.path.join(REPO, ".hunt-work"))

env = {k: v for k, v in os.environ.items() if k.startswith("TRUENAS_")}
envfile = os.path.join(ENV_DIR, f"env.{ip}.sh")
if os.path.exists(envfile):
    for line in open(envfile):
        m = re.match(r"export (\w+)=['\"]?(.*?)['\"]?$", line.strip())
        if m: env[m.group(1)] = m.group(2)
for k in ("TRUENAS_ENDPOINT", "TRUENAS_API_KEY"):
    if k not in env: sys.exit(f"{k} not set and not in {envfile}")
schemafile = os.path.join(WORK, "schema", f"methods-{ip}.json")
if not os.path.exists(schemafile): sys.exit(f"{schemafile} missing: run scripts/hunt/dump-methods.sh {ip}")
provider_bin = os.path.join(WORK, "upgrade", "bin", "current")
if not os.path.exists(os.path.join(provider_bin, "terraform-provider-truenas")):
    sys.exit(f"{provider_bin}/terraform-provider-truenas missing: run scripts/hunt/upgrade/build-versions.sh")

schema = json.load(open(schemafile))[method]["accepts"][0]
node = schema if variant == "-" else (schema.get("oneOf") or schema.get("anyOf"))[int(variant)]
props = node.get("properties") or {}
attrmap = json.loads(attrmap_json)

work = os.path.join(WORK, "enum", f"run-{tftype}-{ip}")
os.makedirs(work, exist_ok=True)
open(f"{work}/dev.tfrc", "w").write(
    'provider_installation {\n  dev_overrides { "truenas/truenas" = "%s" }\n  direct {}\n}\n' % provider_bin)
tfenv = dict(os.environ, TF_CLI_CONFIG_FILE=f"{work}/dev.tfrc", TF_IN_AUTOMATION="1", TF_INPUT="0",
             TF_VAR_endpoint=env["TRUENAS_ENDPOINT"], TF_VAR_api_key=env["TRUENAS_API_KEY"],
             TF_VAR_pool=env.get("TRUENAS_TEST_POOL", "tank"))
base = open(basefile).read()

def tf(*args):
    return subprocess.run(["terraform", *args, "-no-color"], cwd=work, env=tfenv, capture_output=True, text=True)

results = []
for field, hcl in attrmap.items():
    enum = None
    if isinstance(hcl, list):
        hcl, enum = hcl[0], hcl[1]
    else:
        enum = (props.get(field) or {}).get("enum")
        if not enum:
            # some enums live one level down (anyOf with enum)
            for alt in (props.get(field) or {}).get("anyOf") or []:
                if alt.get("enum"): enum = alt["enum"]
    if not enum:
        print(f"-- {field}: no enum in schema, skipped"); continue
    for val in enum:
        if val is None: continue
        sfx = f"{random.randint(1000,9999)}"
        cfg = base.replace("__SUFFIX__", sfx).replace("__EXTRA__", f'  {hcl} = {json.dumps(val)}\n')
        open(f"{work}/main.tf", "w").write(cfg)
        a = tf("apply", "-auto-approve")
        if a.returncode != 0:
            err = " ".join(a.stderr.split())[:160]
            results.append((hcl, val, "APPLY-ERR", err)); print(f"   {hcl}={val!r}: APPLY ERROR {err}")
            tf("destroy", "-auto-approve"); continue
        p = tf("plan", "-detailed-exitcode")
        if p.returncode == 0:
            results.append((hcl, val, "ok", "")); print(f"   {hcl}={val!r}: ok")
        else:
            diff = "\n".join(l for l in p.stdout.splitlines() if re.match(r"\s+[~+-] ", l))[:400]
            results.append((hcl, val, "PLAN-DIFF", diff)); print(f"   {hcl}={val!r}: PLAN DIFF\n{diff}")
        tf("destroy", "-auto-approve")

print("\n=== SUMMARY ===")
for hcl, val, status, info in results:
    if status != "ok": print(f"{status:9s} {hcl} = {val!r}  {info.splitlines()[0] if info else ''}")
print(f"swept {len(results)} values: {sum(1 for r in results if r[2]=='ok')} ok, "
      f"{sum(1 for r in results if r[2]=='PLAN-DIFF')} plan-diff, {sum(1 for r in results if r[2]=='APPLY-ERR')} apply-err")
sys.exit(1 if any(r[2] != "ok" for r in results) else 0)
