"""Downloader unit tests with explicitly synthetic archives; no network calls."""
import hashlib
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location('fetch_engine', Path(__file__).resolve().parents[1] / 'scripts' / 'fetch_engine.py')
assert spec and spec.loader
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def archive_bytes(entries):
    output = io.BytesIO()
    with zipfile.ZipFile(output, 'w') as archive:
        for name, data in entries:
            archive.writestr(name, data)
    return output.getvalue()


class DownloaderTests(unittest.TestCase):
    def test_pinned_archive_extracts_only_allowlisted_names(self):
        data = archive_bytes([('fixture/core', b'unit-test-only'), ('fixture/../escape', b'excluded')])
        descriptor = ('fixture', hashlib.sha256(data).hexdigest(), ('core',))
        with tempfile.TemporaryDirectory() as tmp, patch.dict(module.ARTIFACTS, {'linux': descriptor}), patch.object(module.urllib.request, 'urlopen', return_value=io.BytesIO(data)):
            target = Path(tmp) / 'engine'
            self.assertEqual(module.fetch('linux', target), 1)
            self.assertEqual([p.name for p in target.iterdir()], ['core'])
            self.assertFalse((Path(tmp) / 'escape').exists())

    def test_bad_checksum_never_creates_destination(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(module.urllib.request, 'urlopen', return_value=io.BytesIO(b'tampered fixture')):
            target = Path(tmp) / 'engine'
            with self.assertRaises(ValueError):
                module.fetch('linux', target)
            self.assertFalse(target.exists())

    def test_existing_install_is_never_overwritten_or_downloaded(self):
        with tempfile.TemporaryDirectory() as tmp, patch.object(module.urllib.request, 'urlopen') as request:
            target = Path(tmp) / 'engine'
            target.mkdir()
            (target / 'existing').write_text('preserve')
            with self.assertRaises(ValueError):
                module.fetch('linux', target)
            request.assert_not_called()
            self.assertEqual((target / 'existing').read_text(), 'preserve')

    def test_missing_binary_removes_only_own_staging(self):
        data = archive_bytes([('fixture/unexpected', b'unit-test')])
        descriptor = ('fixture', hashlib.sha256(data).hexdigest(), ('core',))
        with tempfile.TemporaryDirectory() as tmp, patch.dict(module.ARTIFACTS, {'linux': descriptor}), patch.object(module.urllib.request, 'urlopen', return_value=io.BytesIO(data)):
            target = Path(tmp) / 'engine'
            with self.assertRaises(ValueError):
                module.fetch('linux', target)
            self.assertEqual(list(Path(tmp).iterdir()), [])


if __name__ == '__main__':
    unittest.main()
