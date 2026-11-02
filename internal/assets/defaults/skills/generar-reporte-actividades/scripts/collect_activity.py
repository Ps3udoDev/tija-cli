#!/usr/bin/env python3
"""Recolecta la actividad de un autor en varios repositorios para un periodo.

Lee un manifest JSON (ver references/report-schema.md) y genera:
  - actividad.json: commits, PRs, trabajo sin commitear y horas estimadas por repo/dia.
  - actividad.md:   el mismo contenido en formato legible, para redactar el reporte.

Uso:
    python collect_activity.py manifest.json [--out-dir DIR]

Solo lee: no modifica los repositorios (ejecuta `git fetch` unicamente con --fetch).
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

SEP = "\x1f"
SESSION_GAP_MIN = 90   # commits separados por mas de esto pertenecen a sesiones distintas
SESSION_BASE_MIN = 30  # trabajo previo al primer commit de cada sesion
INTEGRATION_BRANCHES = ("main", "master", "dev", "develop")


def run(cmd: list[str], cwd: str | None = None) -> tuple[int, str]:
    p = subprocess.run(cmd, cwd=cwd, capture_output=True, encoding="utf-8", errors="replace")
    return p.returncode, p.stdout


def git(path: str, *args: str) -> str:
    code, out = run(["git", "-C", path, "-c", "i18n.logOutputEncoding=utf-8", *args])
    return out if code == 0 else ""


def default_identities() -> list[str]:
    """Identidades del usuario actual: git config + cuenta de gh."""
    ids = set()
    for key in ("user.name", "user.email"):
        _, out = run(["git", "config", key])
        if out.strip():
            ids.add(out.strip())
    code, out = run(["gh", "api", "user", "--jq", "[.login, .name] | @tsv"])
    if code == 0:
        ids.update(x for x in out.strip().split("\t") if x)
    return sorted(ids)


def matches_identity(identities: list[str], *values: str) -> bool:
    low = {i.lower() for i in identities}
    return any(v and v.lower() in low for v in values)


def round_quarter(hours: float) -> float:
    return round(hours * 4) / 4


def sessions(times: list[dt.datetime]) -> list[tuple[dt.datetime, dt.datetime]]:
    """Agrupa instantes en sesiones; cada una inicia SESSION_BASE_MIN antes del primer commit."""
    out: list[list[dt.datetime]] = []
    for t in sorted(times):
        if out and (t - out[-1][1]).total_seconds() <= SESSION_GAP_MIN * 60:
            out[-1][1] = t
        else:
            out.append([t - dt.timedelta(minutes=SESSION_BASE_MIN), t])
    return [(a, b) for a, b in out]


def hours_of(times: list[dt.datetime]) -> float:
    return sum((b - a).total_seconds() for a, b in sessions(times)) / 3600


def integrated_hashes(path: str) -> set[str]:
    hashes: set[str] = set()
    for b in INTEGRATION_BRANCHES:
        hashes.update(git(path, "log", f"origin/{b}", "--format=%H", "--since=1 year ago").split())
    return hashes


def collect_commits(path: str, identities: list[str], since: dt.date, until: dt.date) -> list[dict]:
    fmt = SEP.join(["%H", "%aI", "%an", "%ae", "%s"])
    # --since filtra por fecha de commit y, con solo una fecha, git completa la hora actual: se pide
    # un dia de margen. El rango real se aplica abajo con la fecha de autor, que conserva cuando se
    # hizo el trabajo aunque luego haya rebase.
    margin = since - dt.timedelta(days=1)
    raw = git(path, "log", "--all", "--no-merges", f"--since={margin.isoformat()}T00:00:00", f"--format={fmt}")
    integrated = integrated_hashes(path)
    commits = []
    for line in raw.splitlines():
        parts = line.split(SEP)
        if len(parts) != 5:
            continue
        sha, when, name, email, subject = parts
        moment = dt.datetime.fromisoformat(when)
        if not (since <= moment.date() <= until) or not matches_identity(identities, name, email):
            continue
        commits.append({"sha": sha[:9], "fecha": moment.date().isoformat(), "hora": moment.strftime("%H:%M"),
                        "instante": moment.isoformat(), "asunto": subject, "integrado": sha in integrated})
    commits.sort(key=lambda c: c["instante"])

    # Un rebase deja copias del mismo commit en ramas viejas: si existe la version integrada,
    # la copia no integrada se descarta.
    integrated_subjects = {c["asunto"] for c in commits if c["integrado"]}
    commits = [c for c in commits if c["integrado"] or c["asunto"] not in integrated_subjects]
    for c in commits:
        if not c["integrado"]:
            refs = git(path, "branch", "-r", "--contains", c["sha"], "--format=%(refname:short)").split()
            c["ramas"] = [r for r in refs if not r.endswith("/HEAD")][:3]
    return commits


def collect_prs(path: str, identities: list[str], since: dt.date, until: dt.date) -> tuple[str, list[dict], str | None]:
    url = git(path, "remote", "get-url", "origin").strip()
    m = re.search(r"github\.com[:/](.+?)(?:\.git)?$", url)
    if not m:
        return url, [], "origin no es un repositorio de GitHub"
    # Los repos renombrados o movidos devuelven listas vacias con el nombre viejo: se usa el canonico.
    code, canon = run(["gh", "repo", "view", m.group(1), "--json", "nameWithOwner", "--jq", ".nameWithOwner"])
    if code != 0:
        return m.group(1), [], "gh no pudo acceder al repositorio"
    slug = canon.strip()
    code, out = run(["gh", "pr", "list", "-R", slug, "--state", "all", "--limit", "500", "--json",
                     "number,title,state,createdAt,mergedAt,author,headRefName,baseRefName"])
    if code != 0:
        return slug, [], "gh pr list fallo"

    def in_period(stamp: str | None) -> bool:
        return bool(stamp) and since <= dt.date.fromisoformat(stamp[:10]) <= until

    prs = []
    for pr in json.loads(out or "[]"):
        author = pr.get("author") or {}
        if not matches_identity(identities, author.get("login", ""), author.get("name", "")):
            continue
        if not (in_period(pr["createdAt"]) or in_period(pr.get("mergedAt"))):
            continue
        prs.append({"numero": pr["number"], "titulo": pr["title"], "estado": pr["state"],
                    "creado": pr["createdAt"][:10], "fusionado": (pr.get("mergedAt") or "")[:10] or None,
                    "rama": pr["headRefName"], "base": pr["baseRefName"]})
    prs.sort(key=lambda p: p["numero"])
    return slug, prs, None


def collect_uncommitted(path: str, since: dt.date, until: dt.date) -> list[dict]:
    """Archivos sin commitear modificados en el periodo, resumidos por carpeta (2 niveles)."""
    listed = git(path, "ls-files", "--others", "--modified", "--exclude-standard").splitlines()
    folders: dict[str, list[dt.date]] = defaultdict(list)
    for rel in listed:
        if rel.startswith(".tija/"):  # archivos de la herramienta, no trabajo del usuario
            continue
        try:
            day = dt.date.fromtimestamp(os.path.getmtime(os.path.join(path, rel)))
        except OSError:
            continue
        if since <= day <= until:
            parts = rel.split("/")
            folders["/".join(parts[:2]) if len(parts) > 2 else rel].append(day)
    return [{"ruta": k, "archivos": len(v), "desde": min(v).isoformat(), "hasta": max(v).isoformat()}
            for k, v in sorted(folders.items())]


def collect(manifest: dict) -> dict:
    period = manifest["periodo"]
    p_since = dt.date.fromisoformat(period["desde"])
    p_until = dt.date.fromisoformat(period["hasta"])
    identities = manifest.get("autores") or default_identities()
    if not identities:
        sys.exit("No se pudo determinar el autor: define 'autores' en el manifest.")

    result = {"periodo": period, "autores": identities, "grupos": []}
    all_times: dict[str, list[dt.datetime]] = defaultdict(list)
    for group in manifest["grupos"]:
        g = {"nombre": group["nombre"], "repos": []}
        group_times: dict[str, list[dt.datetime]] = defaultdict(list)
        for repo in group["repos"]:
            path = repo["path"]
            if not git(path, "rev-parse", "--show-toplevel"):
                print(f"AVISO: {repo['nombre']}: {path} no es un repositorio git", file=sys.stderr)
                g["repos"].append({"nombre": repo["nombre"], "error": "no es un repositorio git"})
                continue
            top = git(path, "rev-parse", "--show-toplevel").strip()
            since = dt.date.fromisoformat(repo.get("desde", period["desde"]))
            until = dt.date.fromisoformat(repo.get("hasta", period["hasta"]))
            commits = collect_commits(top, identities, since, until)
            slug, prs, pr_error = collect_prs(top, identities, since, until)
            uncommitted = collect_uncommitted(top, since, until)
            by_day: dict[str, list[dt.datetime]] = defaultdict(list)
            for c in commits:
                moment = dt.datetime.fromisoformat(c["instante"])
                by_day[c["fecha"]].append(moment)
                all_times[c["fecha"]].append(moment)
                group_times[c["fecha"]].append(moment)
            r = {"nombre": repo["nombre"], "path": top, "github": slug, "desde": since.isoformat(),
                 "hasta": until.isoformat(), "commits": commits, "prs": prs, "sin_commitear": uncommitted,
                 "horas_por_dia": {d: round_quarter(hours_of(t)) for d, t in sorted(by_day.items())}}
            r["horas_estimadas"] = round_quarter(sum(hours_of(t) for t in by_day.values()))
            if pr_error:
                r["aviso_prs"] = pr_error
            g["repos"].append(r)
            print(f"{repo['nombre']}: {len(commits)} commits, {len(prs)} PRs, "
                  f"{r['horas_estimadas']} h estimadas, {len(uncommitted)} rutas sin commitear")
        g["tope_horas"] = round_quarter(sum(hours_of(t) for t in group_times.values()))
        g["tope_horas_por_dia"] = {d: round_quarter(hours_of(t)) for d, t in sorted(group_times.items())}
        result["grupos"].append(g)

    # Trabajar en dos repos a la vez cuenta una sola vez: tope real de horas por dia entre todos los repos.
    result["tope_horas_por_dia"] = {d: round_quarter(hours_of(t)) for d, t in sorted(all_times.items())}
    result["tope_horas_total"] = round_quarter(sum(hours_of(t) for t in all_times.values()))
    return result


def to_markdown(data: dict) -> str:
    lines = [f"# Actividad {data['periodo']['desde']} a {data['periodo']['hasta']}",
             f"Autores: {', '.join(data['autores'])}",
             f"Tope de horas sin contar solapes entre repos: {data['tope_horas_total']} h", ""]
    for g in data["grupos"]:
        lines.append(f"# Grupo: {g['nombre']} (tope sin solapes: {g['tope_horas']} h)")
        for r in g["repos"]:
            if r.get("error"):
                lines += [f"## {r['nombre']}: ERROR {r['error']}", ""]
                continue
            lines.append(f"## {r['nombre']} ({r['github']}) {r['desde']} a {r['hasta']}: "
                         f"{len(r['commits'])} commits, {r['horas_estimadas']} h estimadas")
            if r.get("aviso_prs"):
                lines.append(f"AVISO PRs: {r['aviso_prs']}")
            current = None
            for c in r["commits"]:
                if c["fecha"] != current:
                    current = c["fecha"]
                    lines.append(f"### {current} (~{r['horas_por_dia'][current]} h)")
                flag = "" if c["integrado"] else f" [sin integrar: {', '.join(c.get('ramas', []))}]"
                lines.append(f"- {c['hora']} {c['asunto']}{flag}")
            if r["prs"]:
                lines.append("### PRs")
                for p in r["prs"]:
                    merged = f"fusionado {p['fusionado']}" if p["fusionado"] else p["estado"]
                    lines.append(f"- #{p['numero']} {p['titulo']} ({p['rama']} -> {p['base']}; "
                                 f"creado {p['creado']}; {merged})")
            if r["sin_commitear"]:
                lines.append("### Sin commitear (por fecha de modificacion)")
                for u in r["sin_commitear"]:
                    lines.append(f"- {u['ruta']}: {u['archivos']} archivos, {u['desde']} a {u['hasta']}")
            lines.append("")
    return "\n".join(lines)


def main(argv=None):
    ap = argparse.ArgumentParser(description="Recolecta commits, PRs y trabajo local de un autor en varios repos.")
    ap.add_argument("manifest", type=Path)
    ap.add_argument("--out-dir", type=Path, help="Carpeta de salida (por defecto, la del manifest)")
    ap.add_argument("--fetch", action="store_true", help="Ejecuta git fetch --all antes de leer cada repo")
    args = ap.parse_args(argv)

    manifest = json.loads(args.manifest.read_text(encoding="utf-8-sig"))  # PowerShell puede agregar BOM
    if args.fetch:
        for group in manifest["grupos"]:
            for repo in group["repos"]:
                run(["git", "-C", repo["path"], "fetch", "--all", "--quiet"])
    data = collect(manifest)
    out_dir = args.out_dir or args.manifest.resolve().parent
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "actividad.json").write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")
    (out_dir / "actividad.md").write_text(to_markdown(data), encoding="utf-8")
    print(f"Tope de horas: {data['tope_horas_total']} h  |  Salida: {out_dir}")


if __name__ == "__main__":
    main()
