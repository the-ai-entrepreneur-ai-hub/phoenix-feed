#!/usr/bin/env python3
"""
phoenix-feed - PDF build pipeline
Generates a 1990s-style technical-documentation PDF from compliance-memo.md.
Renders Graphviz dot files to SVG, embeds them, then uses Chrome headless to print to PDF.
"""

import subprocess
import sys
from pathlib import Path

# Paths
ROOT = Path(r"D:/serveless-apps-2026/abusedMindset/phoenix-feed")
PDF_BUILD = ROOT / "pdf-build"
DIAGRAMS_DIR = PDF_BUILD / "diagrams"
SVG_DIR = PDF_BUILD / "svg"
OUTPUT_DIR = PDF_BUILD / "output"
SOURCE_DOCS = [ROOT / "docs" / "compliance-memo.md"]

# Tools (same install paths as peregrine build)
PANDOC = r"C:\Users\Administrator\AppData\Local\Pandoc\pandoc.exe"
DOT = r"C:\Program Files\Graphviz\bin\dot.exe"
CHROME = r"C:\Program Files\Google\Chrome\Application\chrome.exe"

# Diagram -> section anchor mapping (where each diagram is inserted in the doc)
DIAGRAM_INSERTS = {
    "01-data-flow.svg":      ("System architecture",                "Data flow: Phoenix sources to our cache to end user"),
    "02-legal-trigger.svg":  ("Arizona Revised Statutes",           "ARS section 39-121.03 trigger and outcomes"),
    "03-decision-gates.svg": ("Free version: ship with these",      "Free tier and paid tier readiness gates"),
}


def run(cmd, **kwargs):
    res = subprocess.run(cmd, capture_output=True, text=True, encoding="utf-8", **kwargs)
    if res.returncode != 0:
        sys.stderr.write(f"FAILED: {' '.join(cmd) if isinstance(cmd, list) else cmd}\n")
        sys.stderr.write(f"stdout: {res.stdout}\nstderr: {res.stderr}\n")
        sys.exit(1)
    return res.stdout


def render_diagrams():
    SVG_DIR.mkdir(exist_ok=True)
    for dot_file in sorted(DIAGRAMS_DIR.glob("*.dot")):
        svg_file = SVG_DIR / (dot_file.stem + ".svg")
        run([DOT, "-Tsvg", str(dot_file), "-o", str(svg_file)])
        print(f"  rendered {dot_file.name} -> {svg_file.name}")


def read_and_combine_markdown():
    parts = []
    for doc in SOURCE_DOCS:
        text = doc.read_text(encoding="utf-8")
        parts.append(text)
        parts.append("\n\n<div class='section-divider'>* * *</div>\n\n")
    return "\n".join(parts)


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
        sys.stderr.write(f"WARN: did not insert {len(pending)} diagrams (heading not found): {[p[0] for p in pending]}\n")
    return "\n".join(out)


def build_html(md_combined):
    md_path = PDF_BUILD / "_combined.md"
    md_path.write_text(md_combined, encoding="utf-8")
    body_html = run([PANDOC, "-f", "markdown+raw_html", "-t", "html5", str(md_path)])

    css_path = (PDF_BUILD / "style.css").resolve().as_posix()
    title_page = """
<div class="title-page">
  <div class="doc-id">DOCUMENT &middot; PHX-MEMO &middot; V1.0</div>
  <h1>phoenix-feed</h1>
  <div class="subtitle">Compliance Memo &mdash; Free Tier Readiness &amp; Paid Tier Gating</div>
  <div class="ornament">&middot; &middot; &middot;</div>
  <div class="meta">
    <div><span class="label">Author</span> &nbsp; George Kioko</div>
    <div><span class="label">For</span> &nbsp; Dan</div>
    <div><span class="label">Date</span> &nbsp; 3 May 2026</div>
    <div><span class="label">Status</span> &nbsp; Memo, version 1.0</div>
  </div>
  <div class="footer-rule"></div>
</div>
"""
    full_html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>phoenix-feed - Compliance Memo v1.0</title>
<link rel="stylesheet" href="file:///{css_path}">
</head>
<body>
{title_page}
{body_html}
</body>
</html>
"""
    html_path = PDF_BUILD / "_combined.html"
    html_path.write_text(full_html, encoding="utf-8")
    print(f"  wrote {html_path.name}")
    return html_path


def html_to_pdf(html_path):
    OUTPUT_DIR.mkdir(exist_ok=True)
    pdf_path = OUTPUT_DIR / "phoenix-feed-compliance-memo-v1.0.pdf"
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

    print("[2/4] Reading + combining markdown sources")
    md = read_and_combine_markdown()
    md_with_diagrams = insert_diagrams(md)

    print("[3/4] Pandoc: markdown -> HTML")
    html_path = build_html(md_with_diagrams)

    print("[4/4] Chrome headless: HTML -> PDF")
    pdf = html_to_pdf(html_path)

    size_kb = pdf.stat().st_size // 1024
    print(f"\nDone. {pdf} ({size_kb} KB)")


if __name__ == "__main__":
    main()
