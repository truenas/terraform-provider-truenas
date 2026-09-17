#!/usr/bin/env python3
"""Assemble the generated provider docs (docs/) into one self-contained,
standalone HTML page: a grouped sidebar plus a rendered content pane, no
external requests. Output goes to docs-preview/index.html — serve it with
scripts/serve-docs-preview.sh (or open the file directly).

Requires the `markdown` Python package (pip install markdown)."""
import os, re, html
import markdown

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "docs-preview", "index.html")

md = markdown.Markdown(extensions=["tables", "fenced_code", "sane_lists"])

def read(path):
    return open(path, encoding="utf-8").read()

def split_front(s):
    m = re.match(r'^---\n(.*?)\n---\n(.*)$', s, re.S)
    if not m:
        return {}, s
    fm = {}
    for line in m.group(1).splitlines():
        mm = re.match(r'(\w+):\s*"?(.*?)"?\s*$', line)
        if mm:
            fm[mm.group(1)] = mm.group(2)
    return fm, m.group(2)

def title_of(fm, base, kind):
    t = fm.get("page_title", "")
    # "truenas_dataset Resource - truenas" -> "truenas_dataset"
    t = re.sub(r'\s+(Resource|Data Source)\s*-\s*truenas.*$', '', t).strip()
    return t or ("truenas_" + base)

def render(body):
    md.reset()
    return md.convert(body)

pages = {}   # id -> {"title","html","kind","base","subcat"}
def load(kind_dir, kind):
    d = os.path.join(ROOT, "docs", kind_dir)
    if not os.path.isdir(d):
        return
    for fn in sorted(os.listdir(d)):
        if not fn.endswith(".md"):
            continue
        base = fn[:-3]
        fm, body = split_front(read(os.path.join(d, fn)))
        pid = ("r_" if kind == "Resource" else "d_") + base
        pages[pid] = {
            "title": title_of(fm, base, kind),
            "html": render(body),
            "kind": kind,
            "base": base,
            "subcat": fm.get("subcategory", "") or "Other",
        }

load("resources", "Resource")
load("data-sources", "Data Source")

# Provider overview (index.md)
ifm, ibody = split_front(read(os.path.join(ROOT, "docs", "index.md")))
overview_html = render(ibody)

# Best-practices guide
guide_html = ""
gpath = os.path.join(ROOT, "docs", "guides", "best-practices.md")
if os.path.exists(gpath):
    _, gbody = split_front(read(gpath))
    guide_html = render(gbody)

# Group for the sidebar: subcat -> {resources:[(pid,base)], datasources:[...]}
groups = {}
for pid, p in pages.items():
    g = groups.setdefault(p["subcat"], {"Resource": [], "Data Source": []})
    g[p["kind"]].append((pid, p["base"]))
for g in groups.values():
    for k in g:
        g[k].sort(key=lambda x: x[1])

order = sorted(groups.keys())

n_res = sum(1 for p in pages.values() if p["kind"] == "Resource")
n_ds = sum(1 for p in pages.values() if p["kind"] == "Data Source")

# ---- build sidebar nav ----
nav = []
for cat in order:
    g = groups[cat]
    nav.append(f'<section class="grp" data-grp>')
    nav.append(f'<h3 class="grp-h">{html.escape(cat)}</h3>')
    for pid, base in g["Resource"]:
        nav.append(
            f'<a class="nav-item" data-name="{html.escape(base)}" href="#{pid}">'
            f'<span class="mono">truenas_{html.escape(base)}</span></a>'
        )
    for pid, base in g["Data Source"]:
        nav.append(
            f'<a class="nav-item ds" data-name="{html.escape(base)}" href="#{pid}">'
            f'<span class="mono">truenas_{html.escape(base)}</span>'
            f'<span class="tag">data</span></a>'
        )
    nav.append("</section>")
nav_html = "\n".join(nav)

# ---- build content articles ----
arts = []
arts.append(
    '<article class="doc" id="overview" data-doc>'
    f'<div class="kicker">Provider overview</div>{overview_html}</article>'
)
if guide_html:
    arts.append(
        '<article class="doc" id="guide_best_practices" data-doc hidden>'
        f'<div class="kicker">Guide</div>{guide_html}</article>'
    )
for pid, p in pages.items():
    kind_label = p["kind"]
    arts.append(
        f'<article class="doc" id="{pid}" data-doc hidden>'
        f'<div class="kicker">{kind_label}</div>{p["html"]}</article>'
    )
arts_html = "\n".join(arts)

CSS = r"""
:root{
  --ground:#f7f9fa; --surface:#ffffff; --ink:#141a1f; --muted:#5c6873;
  --border:#e2e8ec; --accent:#1f6f8b; --accent-ink:#0f4b60; --code-bg:#f2f5f7;
  --chip:#eef3f5; --shadow:0 1px 2px rgba(20,26,31,.05),0 8px 24px rgba(20,26,31,.04);
}
@media (prefers-color-scheme:dark){
  :root{
    --ground:#0d1115; --surface:#141a20; --ink:#d7dee4; --muted:#8593a0;
    --border:#222c34; --accent:#4bb3c9; --accent-ink:#8fd6e5; --code-bg:#0f151b;
    --chip:#1a222a; --shadow:0 1px 2px rgba(0,0,0,.4),0 12px 30px rgba(0,0,0,.35);
  }
}
:root[data-theme="light"]{
  --ground:#f7f9fa; --surface:#ffffff; --ink:#141a1f; --muted:#5c6873;
  --border:#e2e8ec; --accent:#1f6f8b; --accent-ink:#0f4b60; --code-bg:#f2f5f7;
  --chip:#eef3f5; --shadow:0 1px 2px rgba(20,26,31,.05),0 8px 24px rgba(20,26,31,.04);
}
:root[data-theme="dark"]{
  --ground:#0d1115; --surface:#141a20; --ink:#d7dee4; --muted:#8593a0;
  --border:#222c34; --accent:#4bb3c9; --accent-ink:#8fd6e5; --code-bg:#0f151b;
  --chip:#1a222a; --shadow:0 1px 2px rgba(0,0,0,.4),0 12px 30px rgba(0,0,0,.35);
}
*{box-sizing:border-box}
body{margin:0;background:var(--ground);color:var(--ink);
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
  font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased;}
.mono,code,pre,kbd{font-family:ui-monospace,SFMono-Regular,"SF Mono",Menlo,Consolas,"Liberation Mono",monospace;}
.app{display:grid;grid-template-columns:320px minmax(0,1fr);min-height:100vh;}
/* sidebar */
.side{position:sticky;top:0;height:100vh;overflow-y:auto;background:var(--surface);
  border-right:1px solid var(--border);padding:20px 16px 40px;}
.brand{display:flex;align-items:baseline;gap:8px;padding:6px 6px 2px;}
.brand b{font-size:16px;letter-spacing:-.01em;}
.brand .v{color:var(--muted);font-size:12px;}
.sub{color:var(--muted);font-size:12px;padding:0 6px 14px;line-height:1.5;}
.tools{display:flex;gap:8px;padding:0 4px 14px;}
.filter{flex:1;background:var(--ground);border:1px solid var(--border);color:var(--ink);
  border-radius:8px;padding:8px 10px;font-size:13px;outline:none;}
.filter:focus{border-color:var(--accent);box-shadow:0 0 0 3px color-mix(in srgb,var(--accent) 22%,transparent);}
.tbtn{background:var(--ground);border:1px solid var(--border);color:var(--muted);
  border-radius:8px;width:36px;cursor:pointer;font-size:15px;}
.tbtn:hover{color:var(--accent);border-color:var(--accent);}
.grp{margin:14px 0 4px;}
.grp-h{font-size:11px;text-transform:uppercase;letter-spacing:.09em;color:var(--muted);
  margin:0 0 6px;padding:0 6px;font-weight:600;}
.nav-item{display:flex;align-items:center;gap:8px;text-decoration:none;color:var(--ink);
  padding:5px 8px;border-radius:7px;font-size:13px;}
.nav-item .mono{font-size:12.5px;overflow-wrap:anywhere;}
.nav-item:hover{background:var(--chip);}
.nav-item.active{background:color-mix(in srgb,var(--accent) 14%,transparent);color:var(--accent-ink);}
.nav-item.active .mono{color:var(--accent-ink);}
.nav-item .tag{margin-left:auto;font-size:9.5px;letter-spacing:.05em;text-transform:uppercase;
  color:var(--muted);border:1px solid var(--border);border-radius:5px;padding:1px 5px;}
.side .top-links{display:flex;flex-direction:column;gap:2px;margin-bottom:4px;}
/* content */
.main{padding:44px 52px 120px;}
.wrap{max-width:78ch;margin:0 auto;}
.kicker{font-size:11px;text-transform:uppercase;letter-spacing:.1em;color:var(--accent);
  font-weight:600;margin-bottom:10px;}
.doc h1{font-size:30px;letter-spacing:-.02em;line-height:1.15;margin:.1em 0 .5em;text-wrap:balance;}
.doc h2{font-size:20px;letter-spacing:-.01em;margin:2em 0 .6em;padding-bottom:.3em;
  border-bottom:1px solid var(--border);}
.doc h3{font-size:15.5px;margin:1.6em 0 .4em;}
.doc p{margin:.7em 0;}
.doc a{color:var(--accent);text-decoration:none;}
.doc a:hover{text-decoration:underline;}
.doc code{background:var(--code-bg);border:1px solid var(--border);border-radius:5px;
  padding:.08em .38em;font-size:.88em;}
.doc pre{background:var(--code-bg);border:1px solid var(--border);border-radius:10px;
  padding:16px 18px;overflow-x:auto;line-height:1.5;}
.doc pre code{background:none;border:none;padding:0;font-size:13px;}
.doc ul,.doc ol{padding-left:1.3em;}
.doc li{margin:.25em 0;}
.tbl,.doc>table,.doc div>table{width:100%;}
.doc table{border-collapse:collapse;width:100%;font-size:13.5px;margin:1em 0;display:block;overflow-x:auto;}
.doc thead th{text-align:left;font-size:11px;text-transform:uppercase;letter-spacing:.05em;
  color:var(--muted);border-bottom:1px solid var(--border);padding:8px 12px;white-space:nowrap;}
.doc tbody td{border-bottom:1px solid var(--border);padding:9px 12px;vertical-align:top;}
.doc tbody tr:hover{background:var(--chip);}
.doc blockquote{margin:1em 0;padding:.5em 1em;border-left:3px solid var(--accent);
  background:var(--chip);border-radius:0 8px 8px 0;color:var(--ink);}
.empty{color:var(--muted);font-size:13px;padding:10px 6px;}
/* responsive */
.menu-btn{display:none;}
@media (max-width:860px){
  .app{grid-template-columns:1fr;}
  .side{position:fixed;z-index:20;width:300px;transform:translateX(-102%);transition:transform .2s;
    box-shadow:var(--shadow);}
  .side.open{transform:none;}
  .main{padding:64px 22px 100px;}
  .menu-btn{display:inline-flex;position:fixed;top:12px;left:12px;z-index:30;background:var(--surface);
    border:1px solid var(--border);color:var(--ink);border-radius:9px;padding:8px 12px;cursor:pointer;
    box-shadow:var(--shadow);align-items:center;gap:8px;font-size:13px;}
}
@media (prefers-reduced-motion:reduce){*{transition:none!important;}}
:focus-visible{outline:2px solid var(--accent);outline-offset:2px;border-radius:4px;}
"""

JS = r"""
(function(){
  var docs=document.querySelectorAll('[data-doc]');
  var items=document.querySelectorAll('.nav-item, .top-link');
  function show(id){
    var found=false;
    docs.forEach(function(d){var on=d.id===id;d.hidden=!on;if(on)found=true;});
    if(!found){docs.forEach(function(d){d.hidden=d.id!=='overview';});id='overview';}
    items.forEach(function(a){a.classList.toggle('active',a.getAttribute('href')==='#'+id);});
    var m=document.querySelector('.main');if(m)m.scrollTo?m.scrollTo(0,0):0;
    window.scrollTo(0,0);
    var side=document.querySelector('.side');if(side)side.classList.remove('open');
  }
  function fromHash(){var h=location.hash.replace('#','');show(h||'overview');}
  window.addEventListener('hashchange',fromHash);
  fromHash();
  // filter
  var f=document.getElementById('filter');
  f.addEventListener('input',function(){
    var q=f.value.trim().toLowerCase();
    document.querySelectorAll('.nav-item').forEach(function(a){
      a.style.display=(!q||a.getAttribute('data-name').indexOf(q)>-1)?'':'none';
    });
    document.querySelectorAll('[data-grp]').forEach(function(g){
      var any=Array.prototype.some.call(g.querySelectorAll('.nav-item'),function(a){return a.style.display!=='none';});
      g.style.display=any?'':'none';
    });
  });
  // theme toggle
  var tb=document.getElementById('theme');
  function cur(){var s=document.documentElement.getAttribute('data-theme');
    if(s)return s;return matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light';}
  tb.addEventListener('click',function(){
    var n=cur()==='dark'?'light':'dark';
    document.documentElement.setAttribute('data-theme',n);
    tb.textContent=n==='dark'?'☀':'☽';
  });
  tb.textContent=cur()==='dark'?'☀':'☽';
  // mobile menu
  var mb=document.getElementById('menu');
  if(mb)mb.addEventListener('click',function(){document.querySelector('.side').classList.toggle('open');});
})();
"""

top_links = (
    '<div class="top-links">'
    '<a class="nav-item top-link" href="#overview"><span>Provider overview</span></a>'
    + ('<a class="nav-item top-link" href="#guide_best_practices"><span>Best-practices guide</span></a>' if guide_html else '')
    + '</div>'
)

page = f"""<style>{CSS}</style>
<button class="menu-btn" id="menu">☰ Resources</button>
<div class="app">
  <aside class="side">
    <div class="brand"><b>truenas</b><span class="v">Terraform provider</span></div>
    <div class="sub">{n_res} resources &middot; {n_ds} data sources &middot; {len(order)} groups. Preview of the generated Registry documentation.</div>
    <div class="tools">
      <input id="filter" class="filter" type="text" placeholder="Filter resources…" aria-label="Filter resources">
      <button id="theme" class="tbtn" title="Toggle theme" aria-label="Toggle theme"></button>
    </div>
    {top_links}
    <nav>{nav_html}</nav>
  </aside>
  <main class="main"><div class="wrap">{arts_html}</div></main>
</div>
<script>{JS}</script>
"""

# Wrap as a standalone document (the Artifact host supplies head/body, but a
# locally-served file needs its own).
FAVICON = ("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' "
           "viewBox='0 0 100 100'><text y='.9em' font-size='90'>"
           "%F0%9F%93%98</text></svg>")
doc = (
    "<!doctype html>\n<html lang='en'>\n<head>\n"
    "<meta charset='utf-8'>\n"
    "<meta name='viewport' content='width=device-width, initial-scale=1'>\n"
    "<title>TrueNAS Terraform Provider — Docs Preview</title>\n"
    f"<link rel='icon' href=\"{FAVICON}\">\n"
    "</head>\n<body>\n" + page + "\n</body>\n</html>\n"
)

os.makedirs(os.path.dirname(OUT), exist_ok=True)
open(OUT, "w", encoding="utf-8").write(doc)
print("wrote", OUT, "bytes:", len(doc))
print("resources:", n_res, "data sources:", n_ds, "groups:", len(order))
