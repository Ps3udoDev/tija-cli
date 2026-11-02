#!/usr/bin/env python3
"""Genera el PDF de un reporte de actividades a partir de un JSON.

Reutiliza el motor, la marca y los assets de la skill generar-informe-pdf (ReportLab, sin navegador).

Uso:
    python build_activity_report.py reporte.json [-o salida.pdf] [--preview DIR]
"""
from __future__ import annotations

import argparse
import copy
import datetime as dt
import importlib.util
import json
import sys
from pathlib import Path

SKILL_DIR = Path(__file__).resolve().parent.parent
BASE_SKILL = SKILL_DIR.parent / "generar-informe-pdf"
BASE_ENGINE = BASE_SKILL / "scripts" / "build_report.py"

if not BASE_ENGINE.exists():
    sys.exit(f"No se encontro la skill base generar-informe-pdf en {BASE_SKILL}")
sys.dont_write_bytecode = True  # no dejar __pycache__ dentro de .tija/skills
_spec = importlib.util.spec_from_file_location("build_report", BASE_ENGINE)
base = importlib.util.module_from_spec(_spec)
sys.modules["build_report"] = base
_spec.loader.exec_module(base)

from reportlab.lib import colors  # noqa: E402
from reportlab.lib.enums import TA_CENTER, TA_RIGHT  # noqa: E402
from reportlab.lib.styles import ParagraphStyle  # noqa: E402
from reportlab.platypus import CondPageBreak, HRFlowable, Paragraph, Spacer, Table, TableStyle  # noqa: E402

MESES = ["ene.", "feb.", "mar.", "abr.", "may.", "jun.", "jul.", "ago.", "sep.", "oct.", "nov.", "dic."]
FUENTES = {"estimado", "reportado", "archivos"}
COLUMNAS = ["Fecha", "Actividad", "Descripción", "Tiempo", "Estado"]
ANCHOS_PCT = [12, 24, 40, 9, 15]


def merge(base_dict: dict, override: dict) -> dict:
    out = copy.deepcopy(base_dict)
    for key, value in override.items():
        if isinstance(value, dict) and isinstance(out.get(key), dict):
            out[key] = merge(out[key], value)
        else:
            out[key] = value
    return out


def fmt_rango(desde: str, hasta: str | None) -> str:
    a = dt.date.fromisoformat(desde)
    b = dt.date.fromisoformat(hasta) if hasta else a
    if b < a:
        raise ValueError(f"Rango de fechas invertido: {desde} a {hasta}")
    if a == b:
        return f"{a.day} {MESES[a.month - 1]}"
    if a.month == b.month:
        return f"{a.day}-{b.day} {MESES[a.month - 1]}"
    return f"{a.day} {MESES[a.month - 1]}-{b.day} {MESES[b.month - 1]}"


def fmt_horas(horas: float) -> str:
    return f"{horas:g} h"


class ActivityReport(base.ReportBuilder):
    def __init__(self, tpl: dict, content: dict, content_dir: Path):
        super().__init__(tpl, content, content_dir)
        self.custom = tpl["reporte"]
        self.estados = self.custom["estados"]
        sans, bold = self.fonts.sans_regular, self.fonts.sans_bold
        primary = base.hexcolor(tpl, "primary")
        muted = base.hexcolor(tpl, "muted")
        text = base.hexcolor(tpl, "text")
        self.st = {
            "titulo": ParagraphStyle("r-titulo", fontName=bold, fontSize=20, leading=25, textColor=primary,
                                     alignment=TA_CENTER, spaceAfter=4),
            "periodo": ParagraphStyle("r-periodo", fontName=bold, fontSize=11, leading=15, textColor=text,
                                      alignment=TA_CENTER, spaceBefore=4),
            "responsable": ParagraphStyle("r-resp", parent=self.styles["body"], fontName=sans, fontSize=10,
                                          leading=13, textColor=muted, alignment=TA_CENTER, spaceAfter=8),
            "resumen": ParagraphStyle("r-resumen", fontName=sans, fontSize=10, leading=13.5, textColor=text),
            "banda": ParagraphStyle("r-banda", fontName=bold, fontSize=13, leading=16, textColor=colors.white),
            "banda_der": ParagraphStyle("r-banda-d", fontName=sans, fontSize=9.5, leading=16, textColor=colors.white,
                                        alignment=TA_RIGHT),
            "proyecto": ParagraphStyle("r-proy", fontName=bold, fontSize=11.5, leading=14, textColor=primary),
            "proyecto_der": ParagraphStyle("r-proy-d", fontName=bold, fontSize=9.5, leading=14, textColor=text,
                                           alignment=TA_RIGHT),
            "repo": ParagraphStyle("r-repo", fontName=sans, fontSize=8.5, leading=11, textColor=muted),
            "th": ParagraphStyle("r-th", fontName=bold, fontSize=8.5, leading=11, textColor=colors.white,
                                 alignment=TA_CENTER),
            "fecha": ParagraphStyle("r-fecha", fontName=bold, fontSize=8.5, leading=11, textColor=primary,
                                    alignment=TA_CENTER),
            "actividad": ParagraphStyle("r-act", fontName=bold, fontSize=8.8, leading=11.2, textColor=text),
            "desc": ParagraphStyle("r-desc", fontName=sans, fontSize=8.8, leading=11.2, textColor=text),
            "centro": ParagraphStyle("r-centro", fontName=sans, fontSize=8.8, leading=11.2, textColor=text,
                                     alignment=TA_CENTER),
            "h_resumen": ParagraphStyle("r-hres", fontName=bold, fontSize=11, leading=14, textColor=text,
                                        spaceBefore=8, spaceAfter=3),
            "nota": ParagraphStyle("r-nota", fontName=sans, fontSize=8.3, leading=10.8, textColor=muted),
        }

    # ---- calculo
    def _validar(self, actividad: dict, ctx: str):
        for campo in ("desde", "actividad", "descripcion", "estado"):
            if not actividad.get(campo):
                raise ValueError(f"{ctx}: falta '{campo}' en {actividad}")
        if actividad["estado"] not in self.estados:
            raise ValueError(f"{ctx}: estado '{actividad['estado']}' invalido. Validos: {sorted(self.estados)}")
        if actividad.get("horas") is not None and actividad.get("fuente") not in FUENTES:
            raise ValueError(f"{ctx}: 'fuente' debe ser {sorted(FUENTES)} cuando hay horas: {actividad['actividad']}")

    @staticmethod
    def _horas(actividades: list[dict], fuentes=None) -> float:
        return sum(a["horas"] for a in actividades if a.get("horas") is not None
                   and (fuentes is None or a.get("fuente") in fuentes))

    def _proyecto_horas(self, proyecto: dict) -> float:
        return self._horas(proyecto["actividades"])

    def _grupo_horas(self, grupo: dict) -> float:
        return sum(self._proyecto_horas(p) for p in grupo["proyectos"])

    def _aproximado(self, actividades: list[dict]) -> bool:
        return any(a.get("fuente") in ("estimado", "archivos") for a in actividades)

    def _revisar_tope(self, grupo: dict):
        """Las horas basadas en commits no deben superar el tope sin solapes del recolector."""
        tope = grupo.get("tope_horas")
        if tope is None:
            return
        todas = [a for p in grupo["proyectos"] for a in p["actividades"]]
        horas = self._horas(todas, {"estimado", "reportado"})
        if horas > tope:
            print(f"AVISO: {grupo['nombre']}: {horas:g} h superan el tope de {tope:g} h del recolector; "
                  "hay trabajo simultaneo contado dos veces.", file=sys.stderr)

    # ---- bloques
    def _portada(self) -> list:
        m = self.meta
        out = [Paragraph(self.inline(m["title"]), self.st["titulo"]),
               HRFlowable(width="100%", thickness=1.5, color=base.hexcolor(self.tpl, "accent"), spaceAfter=4),
               Paragraph(self.inline(f"Periodo: {m['periodo']}"), self.st["periodo"])]
        if m.get("responsable"):
            out.append(Paragraph(self.inline(m["responsable"]), self.st["responsable"]))
        if self.content.get("resumen"):
            box = Table([[Paragraph(f"<b>Resumen.</b> {self.inline(self.content['resumen'])}", self.st["resumen"])]],
                        colWidths=[self.frame_w])
            box.setStyle(TableStyle([
                ("BACKGROUND", (0, 0), (-1, -1), colors.HexColor(self.custom["resumen_bg"])),
                ("LEFTPADDING", (0, 0), (-1, -1), 9), ("RIGHTPADDING", (0, 0), (-1, -1), 9),
                ("TOPPADDING", (0, 0), (-1, -1), 7), ("BOTTOMPADDING", (0, 0), (-1, -1), 8),
            ]))
            out += [box, Spacer(1, 6)]
        return out

    def _tabla_resumen(self) -> list:
        filas = [[Paragraph(c, self.st["th"]) for c in ("Grupo", "Proyecto", "Actividades", "Tiempo")]]
        estilos = []
        total, aproximado = 0.0, False
        for grupo in self.content["grupos"]:
            inicio = len(filas)
            for i, proyecto in enumerate(grupo["proyectos"]):
                horas = self._proyecto_horas(proyecto)
                total += horas
                aprox = self._aproximado(proyecto["actividades"])
                aproximado = aproximado or aprox
                filas.append([Paragraph(self.inline(grupo["nombre"]) if i == 0 else "", self.st["actividad"]),
                              Paragraph(self.inline(proyecto["nombre"]), self.st["desc"]),
                              Paragraph(str(len(proyecto["actividades"])), self.st["centro"]),
                              Paragraph(("~" if aprox else "") + fmt_horas(horas), self.st["centro"])])
            estilos.append(("SPAN", (0, inicio), (0, len(filas) - 1)))
        filas.append([Paragraph("Total", self.st["actividad"]), "", "",
                      Paragraph(("~" if aproximado else "") + fmt_horas(total), self.st["actividad"])])
        ultimo = len(filas) - 1
        anchos = [self.frame_w * p for p in (0.26, 0.44, 0.15, 0.15)]
        t = Table(filas, colWidths=anchos, repeatRows=1)
        t.setStyle(TableStyle(estilos + [
            ("BACKGROUND", (0, 0), (-1, 0), base.hexcolor(self.tpl, "table_header_bg")),
            ("LINEBELOW", (0, 0), (-1, -1), 0.5, base.hexcolor(self.tpl, "table_grid")),
            ("BACKGROUND", (0, ultimo), (-1, ultimo), base.hexcolor(self.tpl, "table_zebra_bg")),
            ("SPAN", (0, ultimo), (2, ultimo)),
            ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
            ("TOPPADDING", (0, 0), (-1, -1), 2.5), ("BOTTOMPADDING", (0, 0), (-1, -1), 3.5),
        ]))
        return [Paragraph("Resumen por proyecto", self.st["h_resumen"]), t, Spacer(1, 8)]

    def _banda(self, grupo: dict) -> list:
        derecha = []
        if grupo.get("periodo"):
            derecha.append(grupo["periodo"])
        todas = [a for p in grupo["proyectos"] for a in p["actividades"]]
        derecha.append(("~" if self._aproximado(todas) else "") + fmt_horas(self._grupo_horas(grupo)))
        t = Table([[Paragraph(self.inline(grupo["nombre"]), self.st["banda"]),
                    Paragraph(self.inline("  |  ".join(derecha)), self.st["banda_der"])]],
                  colWidths=[self.frame_w * 0.45, self.frame_w * 0.55])
        t.setStyle(TableStyle([
            ("BACKGROUND", (0, 0), (-1, -1), base.hexcolor(self.tpl, "primary")),
            ("LINEBELOW", (0, 0), (-1, -1), 2.5, base.hexcolor(self.tpl, "accent")),
            ("LEFTPADDING", (0, 0), (-1, -1), 9), ("RIGHTPADDING", (0, 0), (-1, -1), 9),
            ("TOPPADDING", (0, 0), (-1, -1), 4), ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
            ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ]))
        return [CondPageBreak(170), Spacer(1, 6), t, Spacer(1, 4)]

    def _encabezado_proyecto(self, proyecto: dict) -> list:
        nombre = [Paragraph(self.inline(proyecto["nombre"]), self.st["proyecto"])]
        if proyecto.get("repo"):
            nombre.append(Paragraph(self.inline(proyecto["repo"]), self.st["repo"]))
        aprox = "~" if self._aproximado(proyecto["actividades"]) else ""
        t = Table([[nombre, Paragraph(aprox + fmt_horas(self._proyecto_horas(proyecto)), self.st["proyecto_der"])]],
                  colWidths=[self.frame_w * 0.8, self.frame_w * 0.2])
        t.setStyle(TableStyle([
            ("LINEBEFORE", (0, 0), (0, -1), 3, base.hexcolor(self.tpl, "accent")),
            ("LEFTPADDING", (0, 0), (0, -1), 7), ("RIGHTPADDING", (-1, 0), (-1, -1), 2),
            ("TOPPADDING", (0, 0), (-1, -1), 1), ("BOTTOMPADDING", (0, 0), (-1, -1), 2),
            ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ]))
        return [CondPageBreak(120), Spacer(1, 7), t, Spacer(1, 4)]

    def _celda_tiempo(self, a: dict) -> Paragraph:
        if a.get("horas") is None:
            return Paragraph("—", self.st["centro"])
        texto = fmt_horas(a["horas"])
        return Paragraph(f"~{texto}" if a.get("fuente") != "reportado" else texto, self.st["centro"])

    def _tabla_actividades(self, proyecto: dict) -> list:
        filas = [[Paragraph(c, self.st["th"]) for c in COLUMNAS]]
        for a in sorted(proyecto["actividades"], key=lambda x: (x["desde"], x.get("hasta") or x["desde"])):
            color = self.estados[a["estado"]]
            filas.append([
                Paragraph(fmt_rango(a["desde"], a.get("hasta")), self.st["fecha"]),
                Paragraph(self.inline(a["actividad"]), self.st["actividad"]),
                Paragraph(self.inline(a["descripcion"]), self.st["desc"]),
                self._celda_tiempo(a),
                Paragraph(f'<font color="{color}"><b>{base.xml_escape(a["estado"])}</b></font>', self.st["centro"]),
            ])
        anchos = [self.frame_w * p / 100 for p in ANCHOS_PCT]
        t = Table(filas, colWidths=anchos, repeatRows=1)
        t.setStyle(TableStyle([
            ("BACKGROUND", (0, 0), (-1, 0), base.hexcolor(self.tpl, "table_header_bg")),
            ("LINEBELOW", (0, 1), (-1, -1), 0.5, base.hexcolor(self.tpl, "table_grid")),
            ("VALIGN", (0, 0), (-1, 0), "MIDDLE"), ("VALIGN", (0, 1), (-1, -1), "MIDDLE"),
            ("LEFTPADDING", (0, 0), (-1, -1), 4), ("RIGHTPADDING", (0, 0), (-1, -1), 4),
            ("TOPPADDING", (0, 0), (-1, -1), 3), ("BOTTOMPADDING", (0, 0), (-1, -1), 4.5),
        ]))
        return [t, Spacer(1, 6)]

    def story(self):
        if not self.meta.get("title") or not self.meta.get("periodo"):
            raise ValueError("meta.title y meta.periodo son obligatorios")
        story = self._portada() + self._tabla_resumen()
        for grupo in self.content["grupos"]:
            self._revisar_tope(grupo)
            story += self._banda(grupo)
            for proyecto in grupo["proyectos"]:
                for a in proyecto["actividades"]:
                    self._validar(a, f"{grupo['nombre']}/{proyecto['nombre']}")
                story += self._encabezado_proyecto(proyecto)
                if proyecto.get("destacado"):
                    story += self.note({"label": "DESTACADO:", "text": proyecto["destacado"]})
                story += self._tabla_actividades(proyecto)
        notas = self.content.get("notas") or []
        if notas:
            story += [CondPageBreak(70), Spacer(1, 6)]
            story += [Paragraph(self.inline(n), self.st["nota"]) for n in notas]
        return story


def main(argv=None):
    ap = argparse.ArgumentParser(description="Genera un reporte de actividades en PDF (sin navegador).")
    ap.add_argument("content", type=Path, help="JSON del reporte (ver references/report-schema.md)")
    ap.add_argument("-o", "--output", type=Path, help="Ruta del PDF de salida")
    ap.add_argument("--preview", type=Path, help="Carpeta para PNG de cada pagina (requiere pymupdf)")
    args = ap.parse_args(argv)

    template = merge(json.loads((BASE_SKILL / "template.json").read_text(encoding="utf-8")),
                     json.loads((SKILL_DIR / "activity-template.json").read_text(encoding="utf-8")))
    content = json.loads(args.content.read_text(encoding="utf-8-sig"))  # PowerShell puede agregar BOM
    out = args.output or base.default_output(content.get("meta", {}), template, args.content.resolve())
    out.parent.mkdir(parents=True, exist_ok=True)

    pages = ActivityReport(template, content, args.content.resolve().parent).build(out)
    print(f"PDF generado: {out}  ({pages} paginas)")
    if args.preview:
        base.preview(out, args.preview, dpi=75)


if __name__ == "__main__":
    main()
