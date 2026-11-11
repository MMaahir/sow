"""Fails if the docs site's navigation names a page that doesn't exist.

Zensical treats a navigation entry it can't find as a URL, so
"zensical build --strict" accepts a missing page in the navigation. This
check runs before the build and closes that gap.
"""

import pathlib
import sys
import tomllib

config = tomllib.loads(pathlib.Path("zensical.toml").read_text(encoding="utf-8"))["project"]
docs_dir = pathlib.Path(config["docs_dir"])


def targets(entries):
    """Yields every page path in a navigation list, including nested sections."""
    for entry in entries:
        if isinstance(entry, str):
            yield entry
            continue
        for value in entry.values():
            if isinstance(value, list):
                yield from targets(value)
            else:
                yield value


missing = [
    target
    for target in targets(config.get("nav", []))
    if "://" not in target and not (docs_dir / target).is_file()
]
for target in missing:
    print(f"zensical.toml: the navigation names {target}, but {docs_dir / target} doesn't exist", file=sys.stderr)
sys.exit(1 if missing else 0)
