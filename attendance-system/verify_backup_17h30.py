from pathlib import Path

root = Path('e:/Project/attendance-system')
backup = root / 'backup_17h30'

if not backup.exists():
    print('ERROR: backup_17h30 does not exist')
    raise SystemExit(1)

b_files = {p.relative_to(backup).as_posix(): p.stat().st_size for p in backup.rglob('*') if p.is_file()}
cur_files = {p.relative_to(root).as_posix(): p.stat().st_size for p in root.rglob('*') if p.is_file() and 'backup_17h30' not in p.parts}
missing = sorted(f for f in b_files if f not in cur_files)
extra = sorted(f for f in cur_files if f not in b_files)
diff = sorted(f for f in b_files if f in cur_files and cur_files[f] != b_files[f])

print(f'backup_files={len(b_files)}')
print(f'current_files={len(cur_files)}')
print(f'missing={len(missing)}')
for f in missing[:20]:
    print(f'MISSING: {f}')
print(f'extra={len(extra)}')
for f in extra[:20]:
    print(f'EXTRA: {f}')
print(f'size_diff={len(diff)}')
for f in diff[:20]:
    print(f'SIZE_DIFF: {f} backup={b_files[f]} current={cur_files[f]}')
