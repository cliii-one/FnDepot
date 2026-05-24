import re
import sys


DISPLAY_NAMES = {
    "clashlite": "ClashLite",
    "clashlite-dev": "ClashLite Dev",
    "substore": "SubStore",
    "fnnas-notes": "fnnas-notes",
    "mediahub": "MediaHub",
    "oneserver": "OneServer",
}


def update_readme_time(app_name, build_time):
    app_name = app_name.strip().replace('\r', '')
    build_time = build_time.strip().replace('\r', '')
    time_only = re.sub(r'\s*CST$', '', build_time)
    display = DISPLAY_NAMES.get(app_name, app_name)

    with open('README.md', 'r') as f:
        content = f.read()

    pattern = (
        r'(\|\s*\[' + re.escape(display) + r'\]\([^)]*\)\s*—\s*[^|]+'
        r'\|\s*\[!\[' + re.escape(app_name) + r'\][^|]+\|\s*)'
        r'\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}'
    )

    replacement = r'\g<1>' + time_only

    new_content, count = re.subn(pattern, replacement, content)

    if count == 0:
        print(f'警告: 未找到 {app_name} 的行，跳过更新')
        return False

    with open('README.md', 'w') as f:
        f.write(new_content)

    print(f'已更新 {app_name} 编译时间: {time_only}')
    return True


if __name__ == '__main__':
    if len(sys.argv) != 3:
        print('用法: python3 update-readme-time.py <应用名> <构建时间>')
        print('示例: python3 update-readme-time.py clashlite "2026-05-21 08:45 CST"')
        sys.exit(1)

    app_name = sys.argv[1]
    build_time = sys.argv[2]
    success = update_readme_time(app_name, build_time)
    sys.exit(0 if success else 1)
