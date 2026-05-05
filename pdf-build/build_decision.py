#!/usr/bin/env python3
"""
phoenix-feed - PDF build pipeline (architecture decision brief)
Generates a 1990s-style technical-documentation PDF from architecture-decision.md.
Uses the same toolchain as build.py (Pandoc + Graphviz + Chrome headless + style.css).
"""

import subprocess
import sys
from pathlib import Path

ROOT = Path(r"D:/serveless-apps-2026/abusedMindset/phoenix-feed")
PDF_BUILD = ROOT / "pdf-build"
DIAGRAMS_DIR = PDF_BUILD / "diagrams"
SVG_DIR = PDF_BUILD / "svg"
OUTPUT_DIR = PDF_BUILD / "output"
SOURCE_DOC = ROOT / "docs" / "architecture-decision.md"

PANDOC = r"C:\Users\Administrator\AppData\Local\Pandoc\pandoc.exe"
DOT = r"C:\Program Files\Graphviz\bin\dot.exe"
CHROME = r"C:\Program Files\Google\Chrome\Application\chrome.exe"

# v2.0: text-only PDF. Diagrams stay on disk (referenced in Appendix C) but are
# deliberately NOT embedded — visually showing the "recommended end-state with
# workstation" while the body argues "do not buy the workstation yet" would
# create cognitive dissonance for the reader.
DIAGRAM_INSERTS = {}


def run(cmd, **kwargs):
    res = subprocess.run(cmd, capture_output=True, text=True, encoding="utf-8", **kwargs)
    if res.returncode != 0:
        sys.stderr.write(f"FAILED: {' '.join(cmd) if isinstance(cmd, list) else cmd}\n")
        sys.stderr.write(f"stdout: {res.stdout}\nstderr: {res.stderr}\n")
        sys.exit(1)
    return res.stdout


def render_diagrams():
    SVG_DIR.mkdir(exist_ok=True)
    for svg_name in DIAGRAM_INSERTS:
        dot_file = DIAGRAMS_DIR / (Path(svg_name).stem + ".dot")
        svg_file = SVG_DIR / svg_name
        if not dot_file.exists():
            sys.stderr.write(f"FAILED: missing dot file {dot_file}\n")
            sys.exit(1)
        run([DOT, "-Tsvg", str(dot_file), "-o", str(svg_file)])
        print(f"  rendered {dot_file.name} -> {svg_file.name}")


def insert_diagrams(md_text):
    lines = md_text.split("\n")
    out = []
    pending = list(DIAGRAM_INSERTS.items())

    for line in lines:
        out.append(line)
        if line.startswith("#"):
            heading_text = line.lstrip("#").strip().lower()
            for i, (svg_name, (match, caption)) in enumerate(pending):
                if match.lower() in heading_text:
                    svg_path = (SVG_DIR / svg_name).resolve().as_posix()
                    out.append("")
                    out.append(f'<figure class="diagram"><img src="file:///{svg_path}" alt="{caption}"/><figcaption>{caption}</figcaption></figure>')
                    out.append("")
                    pending.pop(i)
                    break
    if pending:
        sys.stderr.write(f"WARN: did not insert {len(pending)} diagrams: {[p[0] for p in pending]}\n")
    return "\n".join(out)


def build_html(md_text):
    md_path = PDF_BUILD / "_decision.md"
    md_path.write_text(md_text, encoding="utf-8")
    body_html = run([PANDOC, "-f", "markdown+raw_html", "-t", "html5", str(md_path)])

    css_path = (PDF_BUILD / "style.css").resolve().as_posix()
    title_page = """
<div class="title-page">
  <div class="doc-id">DOCUMENT &middot; CW-ARCH &middot; V2.0</div>
  <h1>cactus watch<br/>decision brief</h1>
  <div class="subtitle">What Dan should spend before the first customer pays</div>
  <div class="ornament">&middot; &middot; &middot;</div>
  <div class="meta">
    <div><span class="label">Author</span> &nbsp; George Kioko, with Codex counter-review</div>
    <div><span class="label">For</span> &nbsp; Dan</div>
    <div><span class="label">Date</span> &nbsp; 5 May 2026</div>
    <div><span class="label">Status</span> &nbsp; Decision brief, version 2.0 (replaces v1.0)</div>
  </div>
  <div class="footer-rule"></div>
</div>
"""
    full_html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Cactus Watch - Decision Brief v2.0</title>
<link rel="stylesheet" href="file:///{css_path}">
<style>
  /* Per-document override: header text */
  @page {{
    @top-right {{
      content: "DECISION BRIEF - version 2.0";
      font-family: 'Times New Roman', serif;
      font-size: 9pt;
      font-variant: small-caps;
      letter-spacing: 0.05em;
    }}
  }}
  /* Slightly tighter tables for the comparison matrices */
  table {{ font-size: 10pt; }}
  th, td {{ padding: 4px 6px; }}
</style>
</head>
<body>
{title_page}
{body_html}
</body>
</html>
"""
    html_path = PDF_BUILD / "_decision.html"
    html_path.write_text(full_html, encoding="utf-8")
    print(f"  wrote {html_path.name}")
    return html_path


def html_to_pdf(html_path):
    OUTPUT_DIR.mkdir(exist_ok=True)
    pdf_path = OUTPUT_DIR / "cactus-watch-decision-v2.0.pdf"
    html_uri = "file:///" + html_path.resolve().as_posix()

    cmd = [
        CHROME,
        "--headless=new",
        "--disable-gpu",
        "--no-pdf-header-footer",
        "--no-margins=false",
        f"--print-to-pdf={pdf_path}",
        html_uri,
    ]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if not pdf_path.exists():
        sys.stderr.write(f"FAILED: chrome did not produce {pdf_path}\n")
        sys.stderr.write(f"stdout: {res.stdout}\nstderr: {res.stderr}\n")
        sys.exit(1)
    print(f"  wrote {pdf_path}")
    return pdf_path


def main():
    print("[1/4] Rendering Graphviz diagrams -> SVG")
    render_diagrams()

    print("[2/4] Reading source markdown + inserting diagrams")
    md = SOURCE_DOC.read_text(encoding="utf-8")
    md = insert_diagrams(md)

    print("[3/4] Pandoc: markdown -> HTML")
    html_path = build_html(md)

    print("[4/4] Chrome headless: HTML -> PDF")
    pdf = html_to_pdf(html_path)

    size_kb = pdf.stat().st_size // 1024
    print(f"\nDone. {pdf} ({size_kb} KB)")


if __name__ == "__main__":
    main()
