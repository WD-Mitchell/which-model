#!/usr/bin/env python3
"""Verify SDD artifacts: front matter uniformity, placeholders, dep graph, counts.

Usage: python3 specs/verify_sdd.py
Exit 0 = clean, 1 = violations found.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.abspath(__file__))
FEATURES_DIR = os.path.join(ROOT, "features")
GLOBAL_DIR = os.path.join(ROOT, "global")

PLACEHOLDER_RE = re.compile(
    r"\bTODO\b|\bFIXME\b|\bTBD\b|NEEDS CLARIFICATION|None yet|\[pending|\bXXX\b|\?\?\?",
    re.IGNORECASE,
)
FRONTMATTER_RE = re.compile(r"\A---\n(.*?)\n---\n", re.DOTALL)
DEP_ROW_RE = re.compile(
    r"^\| (F\d{2}) \| ([^|]+) \| (M\d) \| ([^|]*) \| ([^|]*) \|$", re.MULTILINE
)

errors = []
warnings = []


def parse_frontmatter(text):
    m = FRONTMATTER_RE.match(text)
    if not m:
        return None, text
    fm = {}
    for line in m.group(1).splitlines():
        if ":" in line:
            k, v = line.split(":", 1)
            fm[k.strip()] = v.strip().strip('"')
    return fm, text[m.end():]


def main():
    features = sorted(
        d for d in os.listdir(FEATURES_DIR)
        if os.path.isdir(os.path.join(FEATURES_DIR, d))
    )
    if len(features) != 30:
        errors.append(f"expected 30 feature dirs, found {len(features)}")

    # --- global artifacts ---
    global_fm = {}
    for fn in ("SPEC.md", "CONTRACTS.md", "TASK-FORMAT.md"):
        path = os.path.join(GLOBAL_DIR, fn)
        if not os.path.exists(path):
            errors.append(f"missing global artifact: {fn}")
            continue
        fm, body = parse_frontmatter(open(path).read())
        if fm is None:
            errors.append(f"{fn}: missing front matter")
        else:
            global_fm[fn] = set(fm.keys())

    # --- per-feature files ---
    feature_deps = {}  # F-id -> set(dep F-ids) from TASKS/CONTRACTS references
    task_counts = {}
    for feat in features:
        fdir = os.path.join(FEATURES_DIR, feat)
        fid = feat.split("-")[0]
        for fn in ("SPEC.md", "CONTRACTS.md", "TASKS.md"):
            path = os.path.join(fdir, fn)
            if not os.path.exists(path):
                errors.append(f"{feat}/{fn}: MISSING")
                continue
            text = open(path).read()
            fm, body = parse_frontmatter(text)
            if fm is None:
                errors.append(f"{feat}/{fn}: missing front matter")
            else:
                if fm.get("version") != "1.0":
                    errors.append(f"{feat}/{fn}: version != 1.0 ({fm.get('version')!r})")
                if fn == "TASKS.md":
                    feat_val = fm.get("feature")
                    if feat_val not in (fid, feat):
                        errors.append(
                            f"{feat}/TASKS.md: front-matter feature={feat_val!r} not in ({fid}, {feat})"
                        )
                    tc = fm.get("task_count")
                    if tc is None:
                        errors.append(f"{feat}/TASKS.md: missing task_count")
                    else:
                        actual = len(re.findall(r"^## Task F\d{2}-T\d+:", body, re.MULTILINE))
                        task_counts[fid] = (int(tc), actual)
                        if int(tc) != actual:
                            errors.append(
                                f"{feat}/TASKS.md: task_count={tc} but {actual} '## Task' sections"
                            )
            # placeholder scan (body only)
            for i, line in enumerate(body.splitlines(), 1):
                if PLACEHOLDER_RE.search(line):
                    # allow the regex table itself in TASK-FORMAT (global, not scanned here)
                    errors.append(f"{feat}/{fn}:{i}: placeholder marker: {line.strip()[:80]}")

        # extract declared depends-on from SPEC.md front matter or a dedicated row
        spec_path = os.path.join(fdir, "SPEC.md")
        if os.path.exists(spec_path):
            spec_text = open(spec_path).read()
            fm_spec, _ = parse_frontmatter(spec_text)
            deps_line = (fm_spec or {}).get("depends_on", "")
            if not deps_line:
                m = re.search(r"^[-*]\s*\*\*Depends on\*\*[:\s]*([^\n]*)", spec_text, re.MULTILINE)
                if not m:
                    m = re.search(r"^\|\s*\*\*Depends on\*\*\s*\|\s*([^|\n]*)", spec_text, re.MULTILINE)
                deps_line = m.group(1) if m else ""
            deps = set(re.findall(r"F\d{2}", deps_line)) - {fid}
            if deps_line or fm_spec:
                feature_deps[fid] = deps

    # --- dependency graph consistency ---
    graph_path = os.path.join(ROOT, "DEPENDENCY-GRAPH.md")
    if os.path.exists(graph_path):
        graph_text = open(graph_path).read()
        rows = DEP_ROW_RE.findall(graph_text)
        graph_deps = {}
        for fid, _title, _ms, deps_raw, _blocks in rows:
            deps = set(re.findall(r"F\d{2}", deps_raw)) - {fid}
            graph_deps[fid] = deps
        if set(graph_deps) != set(features and [f.split("-")[0] for f in features]):
            missing = {f.split("-")[0] for f in features} - set(graph_deps)
            extra = set(graph_deps) - {f.split("-")[0] for f in features}
            if missing:
                errors.append(f"DEPENDENCY-GRAPH missing rows: {sorted(missing)}")
            if extra:
                errors.append(f"DEPENDENCY-GRAPH extra rows: {sorted(extra)}")
        # every dep ref resolves to a known feature
        known = set(graph_deps)
        for fid, deps in graph_deps.items():
            for d in deps:
                if d not in known:
                    errors.append(f"DEPENDENCY-GRAPH: {fid} depends on unknown {d}")
        # cycle check (Kahn)
        indeg = {f: 0 for f in graph_deps}
        adj = {f: [] for f in graph_deps}
        for f, deps in graph_deps.items():
            for d in deps:
                if d in adj:
                    adj[d].append(f)
                    indeg[f] += 1
        queue = [f for f, n in indeg.items() if n == 0]
        seen = 0
        while queue:
            n = queue.pop()
            seen += 1
            for m2 in adj[n]:
                indeg[m2] -= 1
                if indeg[m2] == 0:
                    queue.append(m2)
        if seen != len(graph_deps):
            errors.append(f"DEPENDENCY-GRAPH: cycle detected ({len(graph_deps) - seen} nodes unreachable)")
        # feature specs agree with graph
        for fid, deps in feature_deps.items():
            if fid in graph_deps and deps and deps != graph_deps[fid]:
                warnings.append(
                    f"{fid}: SPEC.md declares deps {sorted(deps)} vs graph {sorted(graph_deps[fid])}"
                )
    else:
        errors.append("DEPENDENCY-GRAPH.md missing")

    # --- task ids unique and sequential ---
    for feat in features:
        tpath = os.path.join(FEATURES_DIR, feat, "TASKS.md")
        if not os.path.exists(tpath):
            continue
        ids = re.findall(r"^## Task (F\d{2}-T\d+):", open(tpath).read(), re.MULTILINE)
        fid = feat.split("-")[0]
        expected = [f"{fid}-T{i}" for i in range(1, len(ids) + 1)]
        if ids != expected:
            errors.append(f"{feat}/TASKS.md: task ids not sequential: {ids}")

    # --- SPEC.md §-anchors cited from TASKS.md resolve ---
    def spec_sections(text):
        ids = set()
        for line in text.splitlines():
            m = re.match(r"^#{2,3} (\d+)(?:\.|\s)", line)
            if m:
                ids.add(m.group(1))
                continue
            m = re.match(r"^(\d+)\. \*\*", line)
            if m:
                ids.add(m.group(1))
            for d in re.findall(r"^\| (D\d+) \|", line):
                ids.add(d)
        return ids

    for feat in features:
        spath = os.path.join(FEATURES_DIR, feat, "SPEC.md")
        tpath = os.path.join(FEATURES_DIR, feat, "TASKS.md")
        if not (os.path.exists(spath) and os.path.exists(tpath)):
            continue
        avail = spec_sections(open(spath).read())
        for m in re.finditer(r"SPEC\.md\s*((?:§[\w.]+(?:,\s*)?)+)", open(tpath).read()):
            for sec in re.findall(r"§([\w.]+)", m.group(1)):
                sec = sec.rstrip(".")
                if sec in avail or sec.split(".")[0] in avail:
                    continue
                errors.append(f"{feat}/TASKS.md: SPEC.md §{sec} does not resolve")

    # --- report ---
    print(f"features: {len(features)}/30")
    print(f"task sections: {sum(a for _, a in task_counts.values())} across {len(task_counts)} features")
    if warnings:
        print(f"\nWARNINGS ({len(warnings)}):")
        for w in warnings:
            print(f"  ~ {w}")
    if errors:
        print(f"\nERRORS ({len(errors)}):")
        for e in errors:
            print(f"  ✗ {e}")
        return 1
    print("\n✓ all checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
