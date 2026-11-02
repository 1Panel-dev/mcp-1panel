#!/usr/bin/env python3
"""Validate Docker inspect metadata and the exact image exported by docker save."""
import hashlib
import json
from pathlib import Path
import re
import sys
import tarfile

LABEL = 'org.opencontainers.image.version'
MAX_METADATA = 1 << 20


def check_config(config, platform, version):
    if not isinstance(config, dict):
        raise ValueError('image metadata must be an object')
    expected_arch = 'arm' if platform == 'linux/arm/v7' else platform.split('/')[1]
    if config.get('os', config.get('Os')) != 'linux' or config.get('architecture', config.get('Architecture')) != expected_arch:
        raise ValueError('image platform does not match DOCKER_PLATFORM; build the requested platform first')
    variant = config.get('variant', config.get('Variant', ''))
    if expected_arch == 'arm' and variant not in ('', 'v7'):
        raise ValueError('image ARM variant does not match linux/arm/v7')
    settings = config.get('config', config.get('Config')) or {}
    if not isinstance(settings, dict):
        raise ValueError('image config metadata must be an object')
    labels = settings.get('Labels') or {}
    if not isinstance(labels, dict) or labels.get(LABEL) != version:
        raise ValueError('image version metadata does not match VERSION; run make docker_build first')


def read_member(archive, name):
    member = archive.getmember(name)
    if not member.isfile() or member.size > MAX_METADATA:
        raise ValueError('image archive metadata is not a bounded regular file')
    handle = archive.extractfile(member)
    if handle is None:
        raise ValueError('image archive metadata is missing')
    with handle:
        return handle.read(MAX_METADATA + 1)


def main():
    operation, platform, version = sys.argv[1:4]
    if operation == 'inspect':
        raw = sys.stdin.buffer.read(MAX_METADATA + 1)
        if len(raw) > MAX_METADATA:
            raise ValueError('Docker inspect metadata is too large')
        metadata = json.loads(raw)
        if not isinstance(metadata, dict):
            raise ValueError('Docker inspect metadata must be an object')
        image_id = metadata.get('Id', '')
        if not re.fullmatch(r'sha256:[a-f0-9]{64}', image_id):
            raise ValueError('Docker did not return an immutable image ID')
        check_config(metadata, platform, version)
        print(image_id)
    elif operation == 'archive':
        image_id, image_ref, archive_path = sys.argv[4:7]
        with tarfile.open(Path(archive_path), mode='r:') as archive:
            manifest = json.loads(read_member(archive, 'manifest.json'))
            if not isinstance(manifest, list) or len(manifest) != 1:
                raise ValueError('Docker archive must contain exactly one image')
            image = manifest[0]
            if not isinstance(image, dict):
                raise ValueError('Docker archive manifest entry must be an object')
            if image_ref not in (image.get('RepoTags') or []):
                raise ValueError('Docker archive does not preserve the requested version tag')
            raw = read_member(archive, image['Config'])
            if 'sha256:' + hashlib.sha256(raw).hexdigest() != image_id:
                raise ValueError('image tag changed during export; rebuild or retry docker_save')
            check_config(json.loads(raw), platform, version)
    else:
        raise ValueError('unsupported Docker verification operation')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, TypeError, OSError, tarfile.TarError) as error:
        print(f'Docker image verification failed: {error}', file=sys.stderr)
        sys.exit(2)
