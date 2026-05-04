#!/usr/bin/env python3
"""
phoenix-feed - PDF build pipeline (commercial-purpose letter template)
Generates a 1990s-style technical-documentation PDF from commercial-purpose-letter-template.md.
Uses the same toolchain as build.py (Pandoc + Chrome headless + style.css).
"""

import subprocess
import sys
from pathlib import Path

ROOT = Path(r"D:/serveless-apps-2026/abusedMindset/phoenix-feed")
PDF_BUILD = ROOT / "pdf-build"
OUTPUT_DIR = PDF_BUILD / "output"
SOURCE_DOCS = [ROOT / "docs" / "commercial-purpose-letter-template.md"]

PANDOC = r"C:\Users\Administrator\AppData\Local\Pandoc\pandoc.exe"
CHROME = r"C:\Program Files\Google\Chrome\Application\chrome.exe"


def run(cmd, **kwargs):
    res = subprocess.run(cmd, capture_output=True, text=True, encoding="utf-8", **kwargs)
    if res.returncode != 0:
        sys.stderr.write(f"FAILED: {' '.join(cmd) if isinstance(cmd, list) else cmd}\n")
        sys.stderr.write(f"stdout: {res.stdout}\nstderr: {res.stderr}\n")
        sys.exit(1)
    return res.stdout


def read_and_combine_markdown():
    parts = []
    for doc in SOURCE_DOCS:
        text = doc.read_text(encoding="utf-8")
        parts.append(text)
    return "\n".join(parts)


def build_html(md_combined):
    md_path = PDF_BUILD / "_letter_template.md"
    md_path.write_text(md_combined, encoding="utf-8")
    body_html = run([PANDOC, "-f", "markdown+raw_html", "-t", "html5", str(md_path)])

    css_path = (PDF_BUILD / "style.css").resolve().as_posix()
    title_page = """
<div class="title-page">
  <div class="doc-id">DOCUMENT &middot; PHX-LETTER &middot; V1.0</div>
  <h1>commercial purpose<br/>letter template</h1>
  <div class="subtitle">A.R.S. &sect; 39&ndash;121.03(A) &mdash; Reusable Boilerplate for Arizona Public Records Custodians</div>
  <div class="ornament">&middot; &middot; &middot;</div>
  <div class="meta">
    <div><span class="label">Author</span> &nbsp; George Kioko</div>
    <div><span class="label">For</span> &nbsp; Dan</div>
    <div><span class="label">Date</span> &nbsp; 4 May 2026</div>
    <div><span class="label">Status</span> &nbsp; Template, version 1.0 &mdash; not legal advice</div>
  </div>
  <div class="footer-rule"></div>
</div>
"""
    full_html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>phoenix-feed - Commercial Purpose Letter Template v1.0</title>
<link rel="stylesheet" href="file:///{css_path}">
<style>
  /* Per-document override: header text */
  @page {{
    @top-right {{
      content: "LETTER TEMPLATE - version 1.0";
      font-family: 'Times New Roman', serif;
      font-size: 9pt;
      font-variant: small-caps;
      letter-spacing: 0.05em;
    }}
  }}
  /* Make the fillable letter (the long pre block) print nicely */
  pre {{
    font-size: 9.2pt;
    line-height: 1.4;
    background: #fff;
  }}
</style>
</head>
<body>
{title_page}
{body_html}
</body>
</html>
"""
    html_path = PDF_BUILD / "_letter_template.html"
    html_path.write_text(full_html, encoding="utf-8")
    print(f"  wrote {html_path.name}")
    return html_path


def html_to_pdf(html_path):
    OUTPUT_DIR.mkdir(exist_ok=True)
    pdf_path = OUTPUT_DIR / "phoenix-feed-letter-template-v1.0.pdf"
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
    print("[1/3] Reading template markdown")
    md = read_and_combine_markdown()

    print("[2/3] Pandoc: markdown -> HTML")
    html_path = build_html(md)

    print("[3/3] Chrome headless: HTML -> PDF")
    pdf = html_to_pdf(html_path)

    size_kb = pdf.stat().st_size // 1024
    print(f"\nDone. {pdf} ({size_kb} KB)")


if __name__ == "__main__":
    main()
