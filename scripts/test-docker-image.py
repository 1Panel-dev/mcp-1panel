#!/usr/bin/env python3
"""Verify local Docker build/export commands without starting a Docker daemon."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[1]
SCRIPT = REPO / 'scripts/docker-image.sh'
LINUX_ARCHITECTURES = {'386', 'amd64', 'arm64', 'armv7', 's390x', 'ppc64le', 'riscv64'}
MOCK_DOCKER = r"""#!/usr/bin/env python3
import hashlib
import io
import json
import os
from pathlib import Path
import sys
import tarfile
args = sys.argv[1:]
with Path(os.environ['MOCK_LOG']).open('a') as handle:
    handle.write(json.dumps(args) + '\n')
def config(platform, version):
    system, arch = platform.split('/')[:2]
    return {'os': system, 'architecture': arch,
            'config': {'Labels': {'org.opencontainers.image.version': version}}}
platform = os.environ.get('MOCK_PLATFORM', 'linux/amd64')
version = os.environ.get('MOCK_VERSION', '1.2.3')
original = config(platform, version)
# A deliberately inconsistent inspect/config pair lets tests verify that the
# saved archive's architecture is checked independently of inspect metadata.
identity = config(os.environ.get('MOCK_IDENTITY_PLATFORM', platform), version)
original_raw = json.dumps(identity, sort_keys=True).encode()
image_id = 'sha256:' + hashlib.sha256(original_raw).hexdigest()
if args[:2] == ['image', 'inspect']:
    if args[2:4] != ['--format', '{{json .}}']:
        sys.exit(3)
    print(json.dumps({'Id': image_id, 'Os': original['os'],
                      'Architecture': original['architecture'],
                      'Variant': os.environ.get('MOCK_VARIANT', ''),
                      'Config': original['config']}))
elif args and args[0] == 'save':
    output = Path(args[args.index('--output') + 1])
    saved = config(os.environ.get('MOCK_ARCHIVE_PLATFORM', platform),
                   os.environ.get('MOCK_ARCHIVE_VERSION', version))
    if os.environ.get('MOCK_RETAG') == '1':
        saved['config']['Labels']['retagged'] = 'true'
    saved_raw = json.dumps(saved, sort_keys=True).encode()
    config_name = hashlib.sha256(saved_raw).hexdigest() + '.json'
    repo_tag = os.environ.get('MOCK_ARCHIVE_TAG', args[-1])
    manifest = [{'Config': config_name, 'RepoTags': [repo_tag], 'Layers': []}]
    if os.environ.get('MOCK_EXTRA_IMAGE') == '1':
        manifest.append(dict(manifest[0]))
    with tarfile.open(output, 'w') as archive:
        for name, raw in [('manifest.json', json.dumps(manifest).encode()),
                          (config_name, saved_raw)]:
            info = tarfile.TarInfo(name)
            info.size = len(raw)
            archive.addfile(info, io.BytesIO(raw))
    if os.environ.get('MOCK_SAVE_FAIL') == '1':
        sys.exit(1)
elif args[:2] != ['buildx', 'build']:
    sys.exit(3)
"""


class DockerCommands(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='mcp-docker-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        docker = self.bin / 'docker'
        docker.write_text(MOCK_DOCKER)
        docker.chmod(0o755)
        self.log = self.root / 'calls.jsonl'
        self.output = self.root / 'output with spaces'
        self.env = dict(os.environ, PATH=f'{self.bin}:{os.environ["PATH"]}',
                        MOCK_LOG=str(self.log), VERSION='1.2.3',
                        DOCKER_IMAGE='example/mcp', DOCKER_PLATFORM='linux/amd64')

    def call(self, operation, **overrides):
        return subprocess.run(['sh', str(SCRIPT), operation, str(REPO), str(self.output)],
                              env=dict(self.env, **overrides), text=True, capture_output=True)

    def calls(self):
        if not self.log.exists():
            return []
        return [json.loads(line) for line in self.log.read_text().splitlines()]

    def archive(self, arch='amd64'):
        return self.output / f'mcp-1panel-linux-{arch}-1.2.3.tar'

    def workflow_job(self, name):
        workflow = (REPO / '.github/workflows/release-and-docker.yml').read_text()
        jobs = re.search(r'(?ms)^jobs:\n(.*)\Z', workflow)
        self.assertIsNotNone(jobs, 'missing workflow jobs')
        match = re.search(rf'(?ms)^  {name}:\n(.*?)(?=^  [A-Za-z_][\w-]*:\n|\Z)', jobs.group(1))
        self.assertIsNotNone(match, f'missing {name} job')
        return match.group(1)

    def test_build_normalizes_supported_platforms_and_injects_version(self):
        for selected, expected in [('linux/386', 'linux/386'),
                                   ('linux/amd64', 'linux/amd64'),
                                   ('linux/arm64/v8', 'linux/arm64'),
                                   ('linux/arm', 'linux/arm/v7'),
                                   ('linux/arm/v7', 'linux/arm/v7'),
                                   ('linux/s390x', 'linux/s390x'),
                                   ('linux/ppc64le', 'linux/ppc64le'),
                                   ('linux/riscv64', 'linux/riscv64')]:
            with self.subTest(platform=selected):
                result = self.call('build', DOCKER_PLATFORM=selected)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.calls()[-1], ['buildx', 'build', '--load',
                                 '--platform', expected, '--build-arg', 'VERSION=1.2.3',
                                 '--tag', 'example/mcp:1.2.3', str(REPO)])
                self.assertNotIn('--push', self.calls()[-1])

    def test_dockerfile_keeps_entrypoint_and_legacy_defaults(self):
        lines = (REPO / 'Dockerfile').read_text().splitlines()
        entrypoint = next(line.removeprefix('ENTRYPOINT ') for line in lines if line.startswith('ENTRYPOINT '))
        command = next(line.removeprefix('CMD ') for line in lines if line.startswith('CMD '))
        self.assertEqual(json.loads(entrypoint), ['/usr/local/bin/mcp-1panel'])
        self.assertEqual(json.loads(command), ['--mode=legacy', '--transport=stdio'])
        self.assertTrue(any('COPY --from=build' in line and '/usr/local/bin/mcp-1panel' in line for line in lines))

    def test_go_module_docker_and_release_toolchains_match(self):
        version = next(line.split()[1] for line in (REPO / 'go.mod').read_text().splitlines()
                       if line.startswith('go '))
        self.assertEqual(version, '1.27.3')
        self.assertIn(f'golang:{version} AS build', (REPO / 'Dockerfile').read_text())
        self.assertIn(f"go-version: '{version}'", (REPO / '.github/workflows/release-and-docker.yml').read_text())

    def test_release_workflow_only_runs_on_publication_and_checks_out_event_commit(self):
        workflow = (REPO / '.github/workflows/release-and-docker.yml').read_text()
        trigger = re.search(r'(?ms)^on:\n(.*?)(?=^\S|\Z)', workflow)
        self.assertIsNotNone(trigger, 'missing workflow trigger')
        events = re.findall(r'(?m)^  ([A-Za-z_][\w-]*):', trigger.group(1))
        self.assertEqual(events, ['release'], 'push, pull requests and manual runs must not publish')
        self.assertRegex(trigger.group(1), r'(?m)^    types: \[published\]$')
        self.assertNotRegex(workflow, r'\b(?:github\.event\.)?inputs\.version\b')

        checkout = r'(?m)^        uses: actions/checkout@v4\n        with:\n          ref: \$\{\{ github\.sha \}\}$'
        for name in ('build', 'docker'):
            with self.subTest(job=name):
                job = self.workflow_job(name)
                self.assertEqual(job.count('uses: actions/checkout@v4'), 1)
                self.assertRegex(job, checkout)
        self.assertEqual(workflow.count('uses: actions/checkout@v4'), 2)

    def test_release_workflow_uploads_assets_without_rewriting_release_metadata(self):
        build = self.workflow_job('build')
        release = self.workflow_job('release')
        docker = self.workflow_job('docker')
        tag = '${{ github.event.release.tag_name }}'
        self.assertIn(f'          RELEASE_VERSION: {tag}\n', build)
        self.assertIn(f'            VERSION={tag}\n', docker)
        self.assertRegex(release, r'(?m)^    permissions:\n      contents: write$')
        for name, job in [('release', release), ('docker', docker)]:
            with self.subTest(job=name):
                self.assertRegex(job, r'(?m)^    needs: build$')

        upload = re.search(r'(?ms)^        uses: softprops/action-gh-release@v2\n        with:\n'
                           r'(.*?)(?=^      - |\Z)', release)
        self.assertIsNotNone(upload, 'missing asset upload to the published release')
        options = re.findall(r'(?m)^          ([a-z_]+): (.+)$', upload.group(1))
        self.assertEqual(dict(options), {'tag_name': tag, 'files': 'final-release/*'})
        self.assertEqual(len(options), 2, 'release title, body, draft and prerelease state must be preserved')

        tags = re.search(r'(?ms)^          tags: \|\n(.*?)(?=^          \S|\Z)', docker)
        self.assertIsNotNone(tags, 'missing Docker release tags')
        actual = [re.sub(r'\s+', '', line) for line in tags.group(1).splitlines() if line.strip()]
        expected = ['${{ env.IMAGE_NAME }}:' + tag,
                    "${{ !github.event.release.prerelease && !contains(github.event.release.tag_name, '-') && format('{0}:latest', env.IMAGE_NAME) || '' }}"]
        self.assertEqual(actual, [re.sub(r'\s+', '', line) for line in expected])
        self.assertRegex(docker, r'(?m)^          push: true$')

    def test_linux_build_and_release_targets_cover_the_same_seven_architectures(self):
        makefile = (REPO / 'Makefile').read_text()
        recipes = re.search(r'(?m)^build_linux_all:\n((?:\t.*\n)+)', makefile)
        self.assertIsNotNone(recipes, 'build_linux_all must invoke each supported target')
        make_arches = re.findall(r'\bGOARCH=([a-z0-9]+)\b', recipes.group(1))
        self.assertEqual(set(make_arches), LINUX_ARCHITECTURES)
        self.assertEqual(len(make_arches), len(LINUX_ARCHITECTURES))

        workflow = (REPO / '.github/workflows/release-and-docker.yml').read_text()
        entries = re.findall(r'(?m)^\s*- goos: ([^\n]+)\n\s+goarch: ([^\n]+)'
                             r'(?:\n\s+goarm: ([^\n]+))?', workflow)
        release_arches = []
        for system, arch, arm in entries:
            system, arch, arm = (value.strip().strip("'\"") for value in (system, arch, arm))
            self.assertEqual(system, 'linux')
            if arch == 'arm':
                self.assertEqual(arm, '7')
                arch = 'armv7'
            release_arches.append(arch)
        self.assertEqual(set(release_arches), LINUX_ARCHITECTURES)
        self.assertEqual(len(release_arches), len(LINUX_ARCHITECTURES))

        platforms = re.findall(r'(?m)^\s*platforms: (linux/[^\n]+)', workflow)
        self.assertEqual(len(platforms), 1, 'release must have one explicit Docker platform list')
        docker_platforms = [value.strip() for value in platforms[0].split(',')]
        expected = {'linux/arm/v7' if arch == 'armv7' else f'linux/{arch}'
                    for arch in LINUX_ARCHITECTURES}
        self.assertEqual(set(docker_platforms), expected)
        self.assertEqual(len(docker_platforms), len(expected))

    def test_dockerfile_build_step_cross_compiles_all_seven_platforms(self):
        # Execute the actual Dockerfile build RUN with a fake Go compiler. This
        # verifies TARGETARCH/VARIANT and the shared binary builder without Docker.
        lines = (REPO / 'Dockerfile').read_text().splitlines()
        start = next(index for index, line in enumerate(lines)
                     if line.startswith('RUN --mount=') and 'set -eu;' in ''.join(lines[index:index + 4]))
        selected = []
        for line in lines[start:]:
            selected.append(line.removesuffix('\\').strip())
            if not line.endswith('\\'):
                break
        command = ' '.join(selected).removeprefix('RUN ')
        command = ' '.join(part for part in command.split(' ') if not part.startswith('--mount='))
        build_output = self.root / 'cross-output'
        command = command.replace('/out', str(build_output))
        fake_go = self.bin / 'go'
        fake_go.write_text("#!/usr/bin/env python3\nimport json, os, pathlib, sys\n"
                           "args = sys.argv[1:]\n"
                           "if args == ['env', 'GOROOT']:\n    print(pathlib.Path(__file__).resolve().parents[1]); sys.exit(0)\n"
                           "pathlib.Path(os.environ['GO_LOG']).write_text(json.dumps({'args': args, 'arch': os.environ['GOARCH'], 'os': os.environ['GOOS'], 'arm': os.environ['GOARM']}))\n"
                           "pathlib.Path(args[args.index('-o') + 1]).write_bytes(b'mock binary')\n")
        fake_go.chmod(0o755)
        log = self.root / 'go.json'
        for arch, variant, expected in [('386', '', '386'), ('amd64', '', 'amd64'),
                                        ('arm64', 'v8', 'arm64'),
                                        ('arm', 'v7', 'arm'), ('s390x', '', 's390x'),
                                        ('ppc64le', '', 'ppc64le'), ('riscv64', '', 'riscv64')]:
            with self.subTest(arch=arch):
                result = subprocess.run(['sh', '-c', command], cwd=REPO,
                                        env=dict(self.env, TARGETOS='linux', TARGETARCH=arch,
                                                 TARGETVARIANT=variant, MCP_GO=str(fake_go), GO_LOG=str(log)),
                                        text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                record = json.loads(log.read_text())
                self.assertEqual(record['arch'], expected)
                self.assertEqual(record['os'], 'linux')
                self.assertEqual(record['arm'], '7')
                self.assertIn('-s -w -X github.com/1Panel-dev/mcp-1panel/utils.Version=1.2.3', record['args'])
                self.assertEqual((build_output / 'mcp-1panel').read_bytes(), b'mock binary')
        for target_os, arch, variant in [('darwin', 'arm64', ''), ('linux', 'arm', 'v6')]:
            with self.subTest(rejected=(target_os, arch, variant)):
                log.unlink(missing_ok=True)
                result = subprocess.run(['sh', '-c', command], cwd=REPO,
                                        env=dict(self.env, TARGETOS=target_os, TARGETARCH=arch,
                                                 TARGETVARIANT=variant, MCP_GO=str(fake_go), GO_LOG=str(log)),
                                        text=True, capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(log.exists())

    def test_linux_builder_resolves_selected_toolchain_before_forcing_local(self):
        selected_root = self.root / 'selected toolchain'
        selected_bin = selected_root / 'bin'
        selected_bin.mkdir(parents=True)
        selected_go = selected_bin / 'go'
        log = self.root / 'selected-go.json'
        selected_go.write_text(
            "#!/usr/bin/env python3\nimport json, os, pathlib, sys\n"
            "args = sys.argv[1:]\n"
            "assert args[0] == 'build'\n"
            "pathlib.Path(os.environ['GO_LOG']).write_text(json.dumps({'args': args, 'toolchain': os.environ['GOTOOLCHAIN'], 'proxy': os.environ['GOPROXY'], 'arch': os.environ['GOARCH']}))\n"
            "pathlib.Path(args[args.index('-o') + 1]).write_bytes(b'selected toolchain binary')\n")
        selected_go.chmod(0o755)
        launcher = self.bin / 'go-launcher'
        launcher.write_text(
            "#!/usr/bin/env python3\nimport os, sys\n"
            "assert sys.argv[1:] == ['env', 'GOROOT']\n"
            "assert os.environ['GOTOOLCHAIN'] == 'auto'\n"
            "assert os.environ['GOPROXY'] == 'off'\n"
            "print(os.environ['SELECTED_GO_ROOT'])\n")
        launcher.chmod(0o755)
        result = subprocess.run(['sh', str(REPO / 'scripts/build-linux.sh'), str(self.output), 'amd64'],
                                env=dict(self.env, MCP_GO=str(launcher), GOTOOLCHAIN='auto',
                                         SELECTED_GO_ROOT=str(selected_root), GO_LOG=str(log)),
                                text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        record = json.loads(log.read_text())
        self.assertEqual(record['toolchain'], 'local')
        self.assertEqual(record['proxy'], 'off')
        self.assertEqual(record['arch'], 'amd64')
        self.assertEqual((self.output / 'mcp-1panel-linux-amd64').read_bytes(), b'selected toolchain binary')

    def test_linux_builder_uses_only_extracted_toolchain_when_launcher_cannot_verify(self):
        module_cache = self.root / 'module cache'
        selected_root = module_cache / 'golang.org/toolchain@v0.0.1-go1.27.3.darwin-arm64'
        selected_bin = selected_root / 'bin'
        selected_bin.mkdir(parents=True)
        selected_go = selected_bin / 'go'
        selected_go.write_text(
            "#!/usr/bin/env python3\nimport os, pathlib, sys\n"
            "assert sys.argv[1] == 'build'\n"
            "assert os.environ['GOTOOLCHAIN'] == 'local'\n"
            "assert os.environ['GOPROXY'] == os.environ['GOSUMDB'] == 'off'\n"
            "pathlib.Path(sys.argv[sys.argv.index('-o') + 1]).write_bytes(b'extracted toolchain binary')\n")
        selected_go.chmod(0o755)
        launcher = self.bin / 'offline-go-launcher'
        launcher.write_text(
            "#!/usr/bin/env python3\nimport os, sys\n"
            "assert sys.argv[1] == 'env'\n"
            "if sys.argv[2] == 'GOROOT': sys.exit(1)\n"
            "assert os.environ['GOTOOLCHAIN'] == 'local'\n"
            "print({'GOMODCACHE': os.environ['SELECTED_MODULE_CACHE'], 'GOHOSTOS': 'darwin', 'GOHOSTARCH': 'arm64'}[sys.argv[2]])\n")
        launcher.chmod(0o755)
        env = dict(self.env, MCP_GO=str(launcher), GOTOOLCHAIN='auto',
                   SELECTED_MODULE_CACHE=str(module_cache))
        command = ['sh', str(REPO / 'scripts/build-linux.sh'), str(self.output), 'amd64']
        output = self.output / 'mcp-1panel-linux-amd64'
        for requested in ['auto', 'go1.25.0+auto']:
            with self.subTest(toolchain=requested):
                result = subprocess.run(command, env=dict(env, GOTOOLCHAIN=requested),
                                        text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output.read_bytes(), b'extracted toolchain binary')
        selected_go.unlink()
        result = subprocess.run(command, env=env, text=True, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(output.read_bytes(), b'extracted toolchain binary')

    def test_version_validation_matches_core(self):
        for version in ['v2.0.0-rc1', 'v2.0.0', '0.0.1-panel-dev', 'v1.2.3', '10.20.30-alpha.1-rc']:
            with self.subTest(version=version):
                result = self.call('build', VERSION=version)
                self.assertEqual(result.returncode, 0, result.stderr)
        for version in ['', 'latest', '../oops', '1.2.3$(touch marker)', 'dev',
                        '01.2.3', '1.02.3', '1.2.03', '1.2.3.4', '1.2.3-',
                        '1.2.3+meta', '1.2.3-a..b', '1.2.3\ninvalid',
                        '1.2.3-' + 'a' * 75]:
            with self.subTest(version=version):
                before = len(self.calls())
                self.assertNotEqual(self.call('build', VERSION=version).returncode, 0)
                self.assertEqual(len(self.calls()), before)

    def test_invalid_platform_and_image_never_invoke_docker(self):
        for changes in [{'DOCKER_PLATFORM': 'linux/amd64,linux/arm64'},
                        {'DOCKER_PLATFORM': 'darwin/arm64'},
                        {'DOCKER_IMAGE': '-malicious'},
                        {'DOCKER_IMAGE': 'example/mcp:latest'},
                        {'DOCKER_IMAGE': 'example/mcp@sha256:abc'},
                        {'DOCKER_IMAGE': 'example/mcp whitespace'}]:
            with self.subTest(changes=changes):
                self.assertNotEqual(self.call('build', **changes).returncode, 0)
                self.assertEqual(self.calls(), [])

    def test_save_checks_platform_and_version_before_replacing_archive(self):
        self.output.mkdir()
        self.archive().write_bytes(b'previous archive')
        for changes in [{'MOCK_PLATFORM': 'linux/arm64'}, {'MOCK_VERSION': '9.0.0'}]:
            with self.subTest(changes=changes):
                self.assertNotEqual(self.call('save', **changes).returncode, 0)
                self.assertEqual(self.archive().read_bytes(), b'previous archive')
                self.assertFalse(any(call[0] == 'save' for call in self.calls()))

    def test_successful_save_uses_atomic_output_and_one_identity_inspection(self):
        result = self.call('save')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertGreater(self.archive().stat().st_size, 0)
        self.assertEqual(len([call for call in self.calls() if call[:2] == ['image', 'inspect']]), 1)
        self.assertEqual(self.calls()[-1][0:2], ['save', '--output'])
        self.assertEqual(self.calls()[-1][-1], 'example/mcp:1.2.3')
        self.assertEqual(list(self.output.glob('.docker-save-*')), [])

    def test_archive_identity_metadata_and_tag_changes_preserve_previous_archive(self):
        self.output.mkdir()
        self.archive().write_bytes(b'previous archive')
        for changes in [{'MOCK_RETAG': '1'}, {'MOCK_ARCHIVE_PLATFORM': 'linux/arm64'},
                        {'MOCK_ARCHIVE_VERSION': '9.0.0'}, {'MOCK_ARCHIVE_TAG': 'example/mcp:latest'},
                        {'MOCK_EXTRA_IMAGE': '1'}]:
            with self.subTest(changes=changes):
                result = self.call('save', **changes)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.archive().read_bytes(), b'previous archive')
                self.assertEqual(list(self.output.glob('.docker-save-*')), [])

    def test_failed_save_preserves_previous_archive_and_removes_staging(self):
        self.output.mkdir()
        self.archive().write_bytes(b'previous archive')
        result = self.call('save', MOCK_SAVE_FAIL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.archive().read_bytes(), b'previous archive')
        self.assertEqual(list(self.output.glob('.docker-save-*')), [])

    def test_symbolic_link_output_is_rejected(self):
        self.output.mkdir()
        target = self.root / 'other file'
        target.write_bytes(b'untouched')
        self.archive().symlink_to(target)
        result = self.call('save')
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue(self.archive().is_symlink())
        self.assertEqual(target.read_bytes(), b'untouched')

    def test_arm_archive_keeps_v7_name_and_rejects_other_variants(self):
        result = self.call('save', DOCKER_PLATFORM='linux/arm/v7', MOCK_PLATFORM='linux/arm', MOCK_VARIANT='v7')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertGreater(self.archive('armv7').stat().st_size, 0)
        result = self.call('save', DOCKER_PLATFORM='linux/arm/v7', MOCK_PLATFORM='linux/arm', MOCK_VARIANT='v6')
        self.assertNotEqual(result.returncode, 0)

    def test_386_archive_checks_inspected_and_saved_architecture(self):
        environment = {'DOCKER_PLATFORM': 'linux/386', 'MOCK_PLATFORM': 'linux/386'}
        result = self.call('save', **environment)
        self.assertEqual(result.returncode, 0, result.stderr)
        archive = self.archive('386')
        previous = archive.read_bytes()
        self.assertGreater(len(previous), 0)

        for changes in [{'MOCK_PLATFORM': 'linux/amd64'},
                        {'MOCK_ARCHIVE_PLATFORM': 'linux/amd64',
                         'MOCK_IDENTITY_PLATFORM': 'linux/amd64'}]:
            with self.subTest(changes=changes):
                before = len([call for call in self.calls() if call[0] == 'save'])
                result = self.call('save', **(environment | changes))
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('image platform does not match DOCKER_PLATFORM', result.stderr)
                self.assertEqual(archive.read_bytes(), previous)
                after = len([call for call in self.calls() if call[0] == 'save'])
                self.assertEqual(after - before, int('MOCK_ARCHIVE_PLATFORM' in changes))
                self.assertEqual(list(self.output.glob('.docker-save-*')), [])


if __name__ == '__main__':
    unittest.main(verbosity=2)
