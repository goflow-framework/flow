#!/usr/bin/env python3
"""
inspect_typecheck_artifacts.py

Scan a CI run artifacts directory for all `ci-export-typecheck` folders and
produce a compact human-readable summary that highlights:
 - number of files and total size
 - largest files (top N)
 - presence of important diagnostic files (golangci output, helper stdout/stderr/exit codes)
 - counts of strace files if present
 - small previews (first few lines) of textual diagnostic files

Usage:
  python3 inspect_typecheck_artifacts.py <run_dir> <out_summary_file>

Example:
  python3 inspect_typecheck_artifacts.py \
    /home/dministrator/repos/flow/artifacts/run_21376142314 \
    /home/dministrator/repos/flow/artifacts/run_21376142314/inspect_summary_improved.txt

"""

from __future__ import annotations
import argparse
import os
import sys
from pathlib import Path
from typing import List, Tuple

# humanize is optional; prefer builtin fallback when not installed
try:
    import humanize
    _HAVE_HUMANIZE = True
except Exception:
    _HAVE_HUMANIZE = False

MAX_PREVIEW_BYTES = 4096
MAX_PREVIEW_LINES = 40
TOP_N_LARGEST = 10

KEY_FILES = [
    'golangci_typecheck.out',
    'golangci_typecheck_blocking.out',
    'helper_stdout.txt',
    'helper_stderr.txt',
    'helper_exit_code',
    'golangci_exit_code',
    'golangci_version_preflight.txt',
    'which_golangci_preflight.txt',
    'container_started.txt',
    'run_helper_stat.txt',
]

TEXT_FILE_EXTS = ['.txt', '.out', '.log', '.json', '.md']


def human_size(n: int) -> str:
    if _HAVE_HUMANIZE:
        try:
            return humanize.naturalsize(n, binary=True)
        except Exception:
            pass
    # fallback simple formatter
    for unit in ['B', 'KiB', 'MiB', 'GiB', 'TiB']:
        if n < 1024.0:
            return f"{n:.1f}{unit}"
        n /= 1024.0
    return f"{n:.1f}PiB"


def find_ci_export_dirs(run_dir: Path) -> List[Path]:
    """Return paths to directories named exactly 'ci-export-typecheck' under run_dir."""
    matches: List[Path] = []
    for root, dirs, files in os.walk(run_dir):
        for d in dirs:
            if d == 'ci-export-typecheck':
                matches.append(Path(root) / d)
    return sorted(matches)


def summarize_dir(dpath: Path) -> dict:
    files = []
    total = 0
    for p in dpath.rglob('*'):
        if p.is_file():
            try:
                sz = p.stat().st_size
            except Exception:
                sz = 0
            files.append((p, sz))
            total += sz
    files_sorted = sorted(files, key=lambda x: x[1], reverse=True)
    return {
        'path': dpath,
        'file_count': len(files),
        'total_size': total,
        'largest': files_sorted[:TOP_N_LARGEST],
        'all_files': files_sorted,
    }


def is_text_file(path: Path) -> bool:
    # quick heuristic
    if path.suffix.lower() in TEXT_FILE_EXTS:
        return True
    try:
        with open(path, 'rb') as f:
            chunk = f.read(2048)
            if b'\0' in chunk:
                return False
            # try decode
            try:
                chunk.decode('utf-8')
                return True
            except Exception:
                return False
    except Exception:
        return False


def preview_file(path: Path, max_bytes: int = MAX_PREVIEW_BYTES, max_lines: int = MAX_PREVIEW_LINES) -> str:
    try:
        with open(path, 'rb') as f:
            data = f.read(max_bytes)
        # decode best-effort
        text = None
        for enc in ('utf-8', 'latin-1'):
            try:
                text = data.decode(enc)
                break
            except Exception:
                continue
        if text is None:
            return '<binary or unreadable file>'
        lines = text.splitlines()
        if len(lines) > max_lines:
            lines = lines[:max_lines]
            lines.append('... (truncated)')
        return '\n'.join(lines)
    except Exception as e:
        return f'<error reading file: {e}>'


def write_summary(run_dir: Path, out_path: Path):
    ci_dirs = find_ci_export_dirs(run_dir)
    with out_path.open('w', encoding='utf-8') as out:
        out.write(f'Inspect summary for run dir: {run_dir}\n')
        out.write(f'Located {len(ci_dirs)} ci-export-typecheck directories\n\n')
        grand_total = 0
        grand_files = 0
        for d in ci_dirs:
            out.write('---\n')
            out.write(f'Directory: {d}\n')
            rel = d.relative_to(run_dir)
            out.write(f'Relative path: {rel}\n')
            s = summarize_dir(d)
            out.write(f'Files: {s["file_count"]}\n')
            out.write(f'Total size: {s["total_size"]} bytes ({human_size(s["total_size"])})\n')
            grand_total += s['total_size']
            grand_files += s['file_count']

            # Key files presence
            out.write('\nKey files presence:\n')
            for k in KEY_FILES:
                p = d / k
                out.write(f'  {k}: {"YES" if p.exists() else "NO"}')
                if p.exists():
                    try:
                        out.write(f' (size={p.stat().st_size} bytes)')
                    except Exception:
                        out.write('')
                out.write('\n')

            # Count strace files
            strace_files = [p for p,sz in s['all_files'] if p.name.startswith('strace_') or p.name.endswith('.strace')]
            out.write(f"Strace-like files: {len(strace_files)}\n")

            # Largest files
            out.write('\nLargest files:\n')
            for p,sz in s['largest']:
                out.write(f'  {p.name:40s} {sz:10d} bytes {human_size(sz)}\n')

            # Show previews of key textual files
            out.write('\nFile previews (first lines):\n')
            for k in KEY_FILES:
                p = d / k
                if p.exists() and p.is_file() and is_text_file(p):
                    out.write(f'--- {k} (first {MAX_PREVIEW_LINES} lines, max {MAX_PREVIEW_BYTES} bytes) ---\n')
                    out.write(preview_file(p))
                    out.write('\n')

            # Show a short listing of small tarballs (if present)
            tarballs = [p for p,sz in s['all_files'] if p.suffix in ('.gz', '.tgz', '.tar') and p.stat().st_size < 5*1024*1024]
            if tarballs:
                out.write('\nSmall tarballs (listing members up to 100 entries):\n')
                import tarfile
                for t in tarballs:
                    out.write(f'Archive: {t.name} size={t.stat().st_size}\n')
                    try:
                        with tarfile.open(t) as tf:
                            for i, m in enumerate(tf.getmembers()):
                                out.write(f'  {m.name} ({m.size} bytes)\n')
                                if i >= 99:
                                    out.write('  ... (truncated)\n')
                                    break
                    except Exception as e:
                        out.write(f'  <failed to read tar: {e}>\n')

            out.write('\n')

        out.write('=== Grand totals ===\n')
        out.write(f'Total ci-export-typecheck dirs: {len(ci_dirs)}\n')
        out.write(f'Total files: {grand_files}\n')
        out.write(f'Total size: {grand_total} bytes ({human_size(grand_total)})\n')


def main(argv: List[str]):
    parser = argparse.ArgumentParser(description='Inspect golangci typecheck ci-export artifacts for a run directory')
    parser.add_argument('run_dir', type=Path, help='Path to the artifacts/run_<id> directory')
    parser.add_argument('out_file', type=Path, help='Path to write the summary to')
    args = parser.parse_args(argv[1:])

    run_dir = args.run_dir
    out_file = args.out_file

    if not run_dir.exists():
        print(f'run_dir not found: {run_dir}', file=sys.stderr)
        sys.exit(2)

    write_summary(run_dir, out_file)
    print(f'Wrote summary to {out_file}')


if __name__ == '__main__':
    main(sys.argv)
