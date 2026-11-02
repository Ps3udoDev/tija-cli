#!/usr/bin/env python3
"""Genera un informe PDF a partir de un JSON de contenido.

Motor 100 % Python (ReportLab). No usa Chromium, navegadores ni LibreOffice.

Uso:
    python build_report.py informe.json [-o salida.pdf] [--template template.json] [--preview DIR]

Dependencias: reportlab (obligatoria), svglib (figuras SVG en vector),
pymupdf (opcional: rasteriza SVG si no hay svglib y genera --preview).
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys
from pathlib import Path

try:
    from reportlab.lib import colors
    from reportlab.lib.enums import TA_CENTER
    from reportlab.lib.styles import ParagraphStyle
    from reportlab.pdfbase import pdfmetrics
    from reportlab.pdfbase.ttfonts import TTFont
    from reportlab.pdfgen import canvas as rl_canvas
    from reportlab.platypus import (BaseDocTemplate, CondPageBreak, Flowable, Frame, HRFlowable, Image,
                                    KeepTogether, ListFlowable, ListItem, PageBreak, PageTemplate,
                                    Paragraph, Spacer, Table, TableStyle)
except ImportError:
    sys.exit("Falta ReportLab. Instala dependencias con:  pip install reportlab svglib")

SKILL_DIR = Path(__file__).resolve().parent.parent
DEFAULT_TEMPLATE = SKILL_DIR / "template.json"


# ---------------------------------------------------------------- fuentes ---

def _font_dirs() -> list[Path]:
    dirs = [SKILL_DIR / "assets" / "fonts"]
    if os.environ.get("INFORME_FONTS_DIR"):
        dirs.insert(0, Path(os.environ["INFORME_FONTS_DIR"]))
    windir = os.environ.get("WINDIR", r"C:\Windows")
    home = Path.home()
    dirs += [Path(windir) / "Fonts", home / "AppData/Local/Microsoft/Windows/Fonts",
             Path("/usr/share/fonts"), Path("/usr/local/share/fonts"), home / ".fonts",
             home / ".local/share/fonts", Path("/Library/Fonts"), Path("/System/Library/Fonts"),
             home / "Library/Fonts"]
    return [d for d in dirs if d.is_dir()]


def _font_index() -> dict[str, Path]:
    index: dict[str, Path] = {}
    for d in _font_dirs():
        for root, _, files in os.walk(d):
            for f in files:
                if f.lower().endswith((".ttf", ".otf")):
                    index.setdefault(f.lower(), Path(root) / f)
    return index


class Fonts:
    """Registra las familias sans/mono de la plantilla, con caida a fuentes PDF base."""

    def __init__(self, spec: dict):
        idx = _font_index()
        self.files: dict[str, str] = {}  # nombre ReportLab -> ruta TTF (para svglib)

        def find(cands):
            for c in cands:
                p = idx.get(c.lower())
                if p:
                    return p
            return None

        sans, mono = spec["sans"], spec["mono"]
        paths = {k: find(sans.get(k, [])) for k in ("regular", "bold", "italic", "bolditalic")}
        if paths["regular"] and paths["bold"]:
            self.ttf = True
            for k, p in paths.items():
                self.files[f"Informe-Sans-{k}"] = str(p or paths["regular"])
                pdfmetrics.registerFont(TTFont(f"Informe-Sans-{k}", self.files[f"Informe-Sans-{k}"]))
            pdfmetrics.registerFontFamily("Informe-Sans", normal="Informe-Sans-regular", bold="Informe-Sans-bold",
                                          italic="Informe-Sans-italic", boldItalic="Informe-Sans-bolditalic")
            self.sans, self.sans_bold = "Informe-Sans-regular", "Informe-Sans-bold"
            self.sans_regular = "Informe-Sans-regular"
        else:
            self.ttf = False
            b = sans.get("builtin", "Helvetica")
            self.sans = self.sans_regular = b
            self.sans_bold = f"{b}-Bold"
            print("AVISO: no se encontro Arial/Liberation/DejaVu; se usa la fuente base "
                  f"{b} (caracteres fuera de WinAnsi pueden no mostrarse).", file=sys.stderr)

        mreg, mbold = find(mono.get("regular", [])), find(mono.get("bold", []))
        if mreg:
            self.files["Informe-Mono"] = str(mreg)
            pdfmetrics.registerFont(TTFont("Informe-Mono", str(mreg)))
            pdfmetrics.registerFont(TTFont("Informe-Mono-bold", str(mbold or mreg)))
            pdfmetrics.registerFontFamily("Informe-Mono", normal="Informe-Mono", bold="Informe-Mono-bold",
                                          italic="Informe-Mono", boldItalic="Informe-Mono-bold")
            self.mono = "Informe-Mono"
        else:
            self.mono = mono.get("builtin", "Courier")

        self._register_svg_fonts()
        # svglib re-registra las familias al mapear fuentes; se restauran para que <b>/<i> funcionen
        if self.ttf:
            pdfmetrics.registerFontFamily("Informe-Sans", normal="Informe-Sans-regular", bold="Informe-Sans-bold",
                                          italic="Informe-Sans-italic", boldItalic="Informe-Sans-bolditalic")
        if self.mono == "Informe-Mono":
            pdfmetrics.registerFontFamily("Informe-Mono", normal="Informe-Mono", bold="Informe-Mono-bold",
                                          italic="Informe-Mono", boldItalic="Informe-Mono-bold")

    def _register_svg_fonts(self):
        """Hace que svglib use las mismas fuentes que el documento para Arial/Helvetica/sans-serif."""
        try:
            from svglib.fonts import register_font
        except ImportError:
            return
        def reg(fam, name, weight="normal"):
            register_font(fam, self.files.get(name), weight=weight, rlgFontName=name)

        for fam in ("Arial", "Helvetica", "sans-serif"):
            reg(fam, self.sans_regular)
            reg(fam, self.sans_bold, "bold")
        for fam in ("Consolas", "monospace", "Courier New"):
            reg(fam, self.mono)


# --------------------------------------------------------------- utilidades ---

def hexcolor(tpl: dict, key_or_hex: str):
    return colors.HexColor(tpl["colors"].get(key_or_hex, key_or_hex))


def xml_escape(s: str) -> str:
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


class Inline:
    """Convierte markdown inline (**negrita**, *cursiva*, `codigo`, [texto](url)) a markup de ReportLab."""

    def __init__(self, tpl: dict, fonts: Fonts):
        self.code_color = tpl["colors"]["inline_code"]
        self.mono = fonts.mono

    def __call__(self, text: str) -> str:
        if text is None:
            return ""
        codes: list[str] = []

        def keep_code(m):
            codes.append(m.group(1))
            return f"\x00{len(codes) - 1}\x00"

        s = re.sub(r"`([^`]+)`", keep_code, str(text))
        s = xml_escape(s)
        s = re.sub(r"\[([^\]]+)\]\(([^)\s]+)\)", r'<link href="\2" color="#2B629E"><u>\1</u></link>', s)
        s = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", s)
        s = re.sub(r"(?<![\w*])\*(?!\s)(.+?)(?<!\s)\*(?![\w*])", r"<i>\1</i>", s)
        s = s.replace("\n", "<br/>")

        def put_code(m):
            c = xml_escape(codes[int(m.group(1))]).replace(" ", "&nbsp;")
            return f'<font face="{self.mono}" color="{self.code_color}">{c}</font>'

        return re.sub(r"\x00(\d+)\x00", put_code, s)


def fmt_fecha(value: str | None) -> str:
    if not value:
        return ""
    try:
        return dt.date.fromisoformat(value).strftime("%d/%m/%Y")
    except ValueError:
        return value


# ------------------------------------------------------------------ builder ---

class ReportBuilder:
    def __init__(self, tpl: dict, content: dict, content_dir: Path):
        self.tpl = tpl
        self.content = content
        self.meta = content.get("meta", {})
        self.base = content_dir
        self.fonts = Fonts(tpl["fonts"])
        self.inline = Inline(tpl, self.fonts)
        pg = tpl["page"]
        self.W, self.H = pg["width"], pg["height"]
        self.frame_w = self.W - pg["margin_left"] - pg["margin_right"]
        self.frame_h = pg["content_bottom"] - pg["content_top"]
        self.section_n = 0
        self.figure_n = 0
        self.styles = self._styles()

    # ---- estilos
    def _ps(self, name, key, **over):
        st = self.tpl["styles"][key]
        kw = dict(fontName=self.fonts.sans_bold if st.get("bold") else self.fonts.sans,
                  fontSize=st["size"], leading=st.get("leading", st["size"] * 1.25),
                  textColor=hexcolor(self.tpl, st.get("color", "text")),
                  spaceBefore=st.get("space_before", 0), spaceAfter=st.get("space_after", 0))
        kw.update(over)
        return ParagraphStyle(name, **kw)

    def _styles(self):
        s = {
            "title": self._ps("title", "title"),
            "subtitle": self._ps("subtitle", "subtitle"),
            "h1": self._ps("h1", "h1", keepWithNext=1),
            "h2": self._ps("h2", "h2", keepWithNext=1),
            "body": self._ps("body", "body"),
            "list": self._ps("list", "list", spaceAfter=self.tpl["styles"]["list"]["item_space"]),
            "table": self._ps("table", "table", textColor=hexcolor(self.tpl, "text")),
            "note": self._ps("note", "note", spaceBefore=0, spaceAfter=0, textColor=hexcolor(self.tpl, "text")),
            "caption": self._ps("caption", "caption", alignment=TA_CENTER),
            "code": ParagraphStyle("code", fontName=self.fonts.mono, fontSize=self.tpl["styles"]["code"]["size"],
                                   leading=self.tpl["styles"]["code"]["leading"],
                                   textColor=hexcolor(self.tpl, "text")),
        }
        s["table_head"] = ParagraphStyle("table_head", parent=s["table"], fontName=self.fonts.sans_bold,
                                         textColor=hexcolor(self.tpl, "table_header_text"))
        s["table_bold"] = ParagraphStyle("table_bold", parent=s["table"], fontName=self.fonts.sans_bold)
        return s

    # ---- encabezado / pie
    def draw_page(self, c: rl_canvas.Canvas, doc):
        H, hd, ft = self.H, self.tpl["header"], self.tpl["footer"]
        c.saveState()
        for key in ("pattern", "logo"):
            im = hd.get(key)
            if im:
                c.drawImage(str(SKILL_DIR / im["src"]), im["x"], H - im["y"] - im["h"], im["w"], im["h"],
                            mask="auto")
        cap = hd.get("caption")
        if cap and cap.get("text"):
            c.setFont(self.fonts.sans_regular, cap["size"])
            c.setFillColor(hexcolor(self.tpl, cap["color"]))
            c.drawCentredString(self.W / 2, H - cap["y"], cap["text"])
        r = ft["rule"]
        c.setStrokeColor(hexcolor(self.tpl, r["color"]))
        c.setLineWidth(r["width"])
        c.line(r["x1"], H - r["y"], r["x2"], H - r["y"])
        c.setFont(self.fonts.sans_regular, ft["size"])
        c.setFillColor(hexcolor(self.tpl, ft["color"]))
        c.drawString(self.tpl["page"]["margin_left"], H - ft["text_y"], ft["text"])
        c.restoreState()

    # ---- bloques
    def title_block(self):
        st = self.tpl["styles"]
        out = [Paragraph(self.inline(self.meta["title"]), self.styles["title"]),
               HRFlowable(width="100%", thickness=st["title_rule"]["width"],
                          color=hexcolor(self.tpl, st["title_rule"]["color"]),
                          spaceBefore=2, spaceAfter=st["title_rule"]["space_after"])]
        if self.meta.get("subtitle"):
            out.append(Paragraph(self.inline(self.meta["subtitle"]), self.styles["subtitle"]))
        return out

    def heading(self, b, level):
        text = b["text"]
        if level == 1 and self.tpl["styles"]["h1"].get("numbered", True) and b.get("numbered", True):
            self.section_n += 1
            text = f"{self.section_n}. {text}"
        return [Paragraph(self.inline(text), self.styles["h1" if level == 1 else "h2"])]

    def paragraph(self, b):
        return [Paragraph(self.inline(b["text"]), self.styles["body"])]

    def _list(self, items, style, level):
        ls = self.tpl["styles"]["list"]
        flow = []
        for it in items:
            if isinstance(it, str):
                it = {"text": it}
            parts = [Paragraph(self.inline(it["text"]), self.styles["list"])]
            if it.get("items"):
                parts.append(self._list(it["items"], it.get("style", "bullet"), level + 1))
            flow.append(ListItem(parts))
        indent = ls["indent"] if level == 0 else ls["nested_indent"]
        common = dict(leftIndent=indent, bulletFontSize=ls["size"], bulletOffsetY=0,
                      spaceBefore=0, spaceAfter=0 if level else 4)
        if style == "numbered":
            return ListFlowable(flow, bulletType="1", bulletFormat="%s.", bulletFontName=self.fonts.sans_bold,
                                bulletColor=hexcolor(self.tpl, ls["number_color"]), bulletDedent=16, **common)
        char = ls["bullet_l1"] if level == 0 else ls["bullet_l2"]
        font = self.fonts.sans_regular
        if not self.fonts.ttf:  # las fuentes base no tienen ● ○
            font, char = "ZapfDingbats", ("●" if level == 0 else "❍")
        return ListFlowable(flow, bulletType="bullet", start=char, bulletFontName=font,
                            bulletFontSize=ls["size"] * 0.95, bulletColor=hexcolor(self.tpl, "bullet"),
                            bulletDedent=14, **{k: v for k, v in common.items() if k != "bulletFontSize"})

    def list_block(self, b):
        return [self._list(b["items"], b.get("style", "bullet"), 0)]

    def code(self, b):
        st = self.tpl["styles"]["code"]
        lines = b["text"].rstrip("\n").split("\n")
        rows = [[Paragraph(xml_escape(l).replace(" ", "&nbsp;") or "&nbsp;", self.styles["code"])] for l in lines]
        t = Table(rows, colWidths=[self.frame_w])
        n = len(rows)
        t.setStyle(TableStyle([
            ("BACKGROUND", (0, 0), (-1, -1), hexcolor(self.tpl, "code_bg")),
            ("LINEBEFORE", (0, 0), (0, -1), st["bar_width"], hexcolor(self.tpl, "code_bar")),
            ("LEFTPADDING", (0, 0), (-1, -1), st["padding"] + 2),
            ("RIGHTPADDING", (0, 0), (-1, -1), st["padding"]),
            ("TOPPADDING", (0, 0), (-1, -1), 0), ("BOTTOMPADDING", (0, 0), (-1, -1), 0),
            ("TOPPADDING", (0, 0), (-1, 0), st["padding"]),
            ("BOTTOMPADDING", (0, n - 1), (-1, n - 1), st["padding"]),
        ]))
        return [Spacer(1, st["space_before"]), t, Spacer(1, st["space_after"])]

    def table(self, b, first_col_bold=None):
        st = self.tpl["styles"]["table"]
        cols = b["columns"]
        pct = b.get("col_widths_pct") or [100 / len(cols)] * len(cols)
        widths = [self.frame_w * p / sum(pct) for p in pct]
        fcb = b.get("first_col_bold", False) if first_col_bold is None else first_col_bold
        data = [[Paragraph(self.inline(c), self.styles["table_head"]) for c in cols]]
        for row in b["rows"]:
            if len(row) != len(cols):
                raise ValueError(f"Fila con {len(row)} celdas, se esperaban {len(cols)}: {row}")
            data.append([Paragraph(self.inline(v), self.styles["table_bold" if (j == 0 and fcb) else "table"])
                         for j, v in enumerate(row)])
        t = Table(data, colWidths=widths, repeatRows=1)
        style = [
            ("BACKGROUND", (0, 0), (-1, 0), hexcolor(self.tpl, "table_header_bg")),
            ("GRID", (0, 0), (-1, -1), 0.6, hexcolor(self.tpl, "table_grid")),
            ("VALIGN", (0, 0), (-1, -1), "TOP"),
            ("LEFTPADDING", (0, 0), (-1, -1), st["padding"] + 1),
            ("RIGHTPADDING", (0, 0), (-1, -1), st["padding"]),
            ("TOPPADDING", (0, 0), (-1, -1), 2),
            ("BOTTOMPADDING", (0, 0), (-1, -1), st["padding"]),
        ]
        for i in range(2, len(data), 2):
            style.append(("BACKGROUND", (0, i), (-1, i), hexcolor(self.tpl, "table_zebra_bg")))
        t.setStyle(TableStyle(style))
        return [Spacer(1, st["space_before"]), t, Spacer(1, st["space_after"])]

    def note(self, b):
        st = self.tpl["styles"]["note"]
        label = b.get("label", "NOTA:")
        text = f"<b>{xml_escape(label)}</b> {self.inline(b['text'])}" if label else self.inline(b["text"])
        t = Table([[Paragraph(text, self.styles["note"])]], colWidths=[self.frame_w])
        t.setStyle(TableStyle([
            ("BACKGROUND", (0, 0), (-1, -1), hexcolor(self.tpl, b.get("bg", "note_bg"))),
            ("LINEBEFORE", (0, 0), (0, -1), st["bar_width"], hexcolor(self.tpl, b.get("bar", "note_bar"))),
            ("LEFTPADDING", (0, 0), (-1, -1), st["padding"] + 2),
            ("RIGHTPADDING", (0, 0), (-1, -1), st["padding"]),
            ("TOPPADDING", (0, 0), (-1, -1), st["padding"]),
            ("BOTTOMPADDING", (0, 0), (-1, -1), st["padding"]),
        ]))
        return [Spacer(1, st["space_before"]), t, Spacer(1, st["space_after"])]

    def _load_figure(self, path: Path, max_w: float, max_h: float) -> Flowable:
        if not path.exists():
            raise FileNotFoundError(f"Figura no encontrada: {path}")
        if path.suffix.lower() == ".svg":
            try:
                from svglib.svglib import svg2rlg
                d = svg2rlg(str(path))
                scale = min(max_w / d.width, max_h / d.height)
                d.width, d.height = d.width * scale, d.height * scale
                d.scale(scale, scale)
                d.hAlign = "CENTER"
                return d
            except ImportError:
                pass
            png = path.with_suffix(".png")
            if not png.exists():
                try:
                    import fitz  # pymupdf
                    svg = fitz.open(str(path))
                    svg[0].get_pixmap(dpi=220).save(str(png))
                except ImportError:
                    raise RuntimeError(f"Para usar {path.name} instala svglib (o pymupdf), "
                                       "o entrega un PNG con el mismo nombre.")
            path = png
        img = Image(str(path))
        scale = min(max_w / img.imageWidth, max_h / img.imageHeight)
        img.drawWidth, img.drawHeight = img.imageWidth * scale, img.imageHeight * scale
        img.hAlign = "CENTER"
        return img

    def figure(self, b):
        st = self.tpl["styles"]["figure"]
        pct = b.get("width_pct", st["default_width_pct"])
        fig = self._load_figure((self.base / b["src"]).resolve(), self.frame_w * pct / 100, self.frame_h * 0.85)
        parts = [Spacer(1, st["space_before"]), fig]
        if b.get("caption"):
            self.figure_n += 1
            prefix = self.tpl["styles"]["caption"]["prefix"].format(n=self.figure_n)
            parts += [Spacer(1, 6), Paragraph(self.inline(prefix + b["caption"]), self.styles["caption"])]
        parts.append(Spacer(1, st["space_after"]))
        return [KeepTogether(parts)]

    def image(self, b):  # alias de figure
        return self.figure(b)

    def page_break(self, b):
        return [PageBreak()]

    def spacer(self, b):
        return [Spacer(1, b.get("height", 12))]

    def keep_space(self, b):
        """Salta de pagina si quedan menos de 'height' pt libres."""
        return [CondPageBreak(b.get("height", 120))]

    # ---- ficha tecnica (seccion 1 obligatoria)
    def ficha_tecnica(self):
        ft = self.tpl["ficha_tecnica"]
        if not ft.get("enabled") or self.meta.get("ficha_tecnica") is False:
            return []
        f = dict(self.meta.get("ficha", {}))
        m = self.meta
        ph = ft["placeholder"]
        f.setdefault("nombre", m.get("title"))
        if "version_fecha" not in f and (m.get("version") or m.get("fecha")):
            f["version_fecha"] = f"Versión {m.get('version', '1.0')} — Fecha: {fmt_fecha(m.get('fecha'))}"
        if "aprobadores" not in f:
            f["aprobadores"] = f"Aprobó: {m.get('aprobo') or ph} · Clasificación: {m.get('clasificacion', 'Confidencial')}"
        rows = [[label, f.get(key) or ph] for key, label in ft["fields"]]
        out = self.heading({"text": ft["heading"]}, 1)
        out += [Paragraph(self.inline(ft["intro"]), self.styles["body"])]
        out += self.table({"columns": ft["columns"], "rows": rows, "col_widths_pct": ft["col_widths_pct"]},
                          first_col_bold=False)
        note = ft.get("compliance_note")
        if note:
            out += self.note(note)
        return out

    # ---- ensamblado
    BLOCKS = {"h1": "h1", "heading": "h1", "h2": "h2", "subheading": "h2", "paragraph": "paragraph",
              "p": "paragraph", "list": "list_block", "code": "code", "table": "table", "note": "note",
              "figure": "figure", "image": "image", "page_break": "page_break", "spacer": "spacer",
              "keep_space": "keep_space"}

    def story(self):
        if not self.meta.get("title"):
            raise ValueError("meta.title es obligatorio")
        story = self.title_block() + self.ficha_tecnica()
        for i, b in enumerate(self.content.get("blocks", [])):
            t = b.get("type")
            if t not in self.BLOCKS:
                raise ValueError(f"Bloque #{i} tipo '{t}' desconocido. Validos: {sorted(self.BLOCKS)}")
            if t in ("h1", "heading"):
                story += self.heading(b, 1)
            elif t in ("h2", "subheading"):
                story += self.heading(b, 2)
            else:
                story += getattr(self, self.BLOCKS[t])(b)
        return story

    def build(self, out: Path):
        pg = self.tpl["page"]
        frame = Frame(pg["margin_left"], self.H - pg["content_bottom"], self.frame_w, self.frame_h,
                      leftPadding=0, rightPadding=0, topPadding=0, bottomPadding=0, id="body")
        doc = BaseDocTemplate(str(out), pagesize=(self.W, self.H), title=self.meta.get("title", ""),
                              author=self.meta.get("autor", "Tija"), subject=self.meta.get("subtitle", ""),
                              creator="generar-informe-pdf (ReportLab)")
        doc.addPageTemplates([PageTemplate(id="pagina", frames=[frame], onPage=self.draw_page)])
        ft = self.tpl["footer"]
        canvasmaker = rl_canvas.Canvas
        if ft.get("page_numbers"):
            canvasmaker = _numbered_canvas(self, ft)
        doc.build(self.story(), canvasmaker=canvasmaker)
        return doc.page


def _numbered_canvas(builder: ReportBuilder, ft: dict):
    class NumberedCanvas(rl_canvas.Canvas):
        def __init__(self, *a, **kw):
            super().__init__(*a, **kw)
            self._pages = []

        def showPage(self):
            self._pages.append(dict(self.__dict__))
            self._startPage()

        def save(self):
            total = len(self._pages)
            for st in self._pages:
                self.__dict__.update(st)
                self.setFont(builder.fonts.sans_regular, ft["size"])
                self.setFillColor(hexcolor(builder.tpl, ft["color"]))
                self.drawRightString(builder.tpl["footer"]["rule"]["x2"], builder.H - ft["text_y"],
                                     ft["page_number_format"].format(page=self._pageNumber, total=total))
                super().showPage()
            super().save()

    return NumberedCanvas


# ----------------------------------------------------------------------- cli ---

def default_output(meta: dict, tpl: dict, json_path: Path) -> Path:
    if meta.get("output"):
        return (json_path.parent / meta["output"]).resolve()
    try:
        fecha = dt.date.fromisoformat(meta["fecha"])
        name = tpl["naming"]["pattern"].format(cliente=meta["cliente"], nombre_corto=meta["nombre_corto"],
                                               fecha=fecha, version=meta.get("version", "1.0"))
        return json_path.parent / name
    except (KeyError, ValueError):
        return json_path.with_suffix(".pdf")


def preview(pdf: Path, out_dir: Path, dpi: int = 70):
    try:
        import fitz
    except ImportError:
        print("Preview omitido: instala pymupdf para generar PNGs de revision.", file=sys.stderr)
        return
    out_dir.mkdir(parents=True, exist_ok=True)
    for i, page in enumerate(fitz.open(str(pdf))):
        page.get_pixmap(dpi=dpi).save(str(out_dir / f"pagina-{i + 1:02d}.png"))
    print(f"Preview: {out_dir}")


def main(argv=None):
    ap = argparse.ArgumentParser(description="Genera un informe PDF (sin navegador).")
    ap.add_argument("content", type=Path, help="JSON de contenido (ver references/content-schema.md)")
    ap.add_argument("-o", "--output", type=Path, help="Ruta del PDF de salida")
    ap.add_argument("--template", type=Path, default=DEFAULT_TEMPLATE, help="Plantilla de marca/layout")
    ap.add_argument("--preview", type=Path, help="Carpeta donde guardar PNG de cada pagina (requiere pymupdf)")
    args = ap.parse_args(argv)

    tpl = json.loads(args.template.read_text(encoding="utf-8"))
    content = json.loads(args.content.read_text(encoding="utf-8"))
    out = args.output or default_output(content.get("meta", {}), tpl, args.content.resolve())
    out.parent.mkdir(parents=True, exist_ok=True)

    builder = ReportBuilder(tpl, content, args.content.resolve().parent)
    pages = builder.build(out)
    print(f"PDF generado: {out}  ({pages} paginas)")
    if args.preview:
        preview(out, args.preview)


if __name__ == "__main__":
    main()
