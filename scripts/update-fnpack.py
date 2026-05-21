import json
import sys


def update_fnpack(app_key, tag, repo):
    app_key = app_key.strip().replace('\r', '')
    tag = tag.strip().replace('\r', '')
    repo = repo.strip().replace('\r', '')

    with open('fnpack.json', 'r') as f:
        data = json.load(f)

    if app_key not in data:
        print(f'错误: fnpack.json 中未找到 {app_key}')
        return False

    version = tag.split('-v', 1)[-1] if '-v' in tag else tag
    base_url = f"https://github.com/{repo}/releases/download/{tag}"

    data[app_key]['version'] = version

    if 'arch_diff' in data[app_key]:
        for arch in data[app_key]['arch_diff']:
            arch_suffix = 'X86' if arch == 'x86' else 'ARM'
            filename = f"{app_key}-{arch_suffix}.fpk"
            data[app_key]['arch_diff'][arch]['download_url'] = f"{base_url}/{filename}"

    with open('fnpack.json', 'w') as f:
        json.dump(data, f, indent=2, ensure_ascii=False)
        f.write('\n')

    print(f'已更新 fnpack.json: {app_key} → v{version}')
    print(f'  x86: {data[app_key]["arch_diff"]["x86"]["download_url"]}')
    print(f'  arm: {data[app_key]["arch_diff"]["arm"]["download_url"]}')
    return True


if __name__ == '__main__':
    if len(sys.argv) != 4:
        print('用法: python3 update-fnpack.py <应用键名> <标签> <仓库>')
        print('示例: python3 update-fnpack.py mediahub mediahub-v1.2.0 cliii-one/FnDepot')
        sys.exit(1)

    success = update_fnpack(sys.argv[1], sys.argv[2], sys.argv[3])
    sys.exit(0 if success else 1)
