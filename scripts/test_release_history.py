# Exercise the legacy-tag migration and metadata-only releases with real Git.
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
import importlib.util

spec = importlib.util.spec_from_file_location("release_plan", Path(__file__).with_name("release-plan.py"))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseHistoryTests(unittest.TestCase):
    def test_migrate_legacy_tag_and_skip_documentation_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], text=True, stderr=subprocess.DEVNULL).strip()
            def commit(message):
                git("add", ".")
                git("-c", "commit.gpgsign=false", "commit", "-m", message)
            git("init", "-b", "main")
            git("config", "tag.gpgsign", "false")
            git("config", "user.name", "Release test")
            git("config", "user.email", "release@example.invalid")
            (root / "src").mkdir()
            (root / "src/main.go").write_text("legacy application")
            commit("Legacy release")
            git("tag", "v0.1.0")  # Existing url-finder tag predates VERSION.
            (root / "src/VERSION").write_text("0.1.0\n")
            (root / "src/main.go").write_text("maintained application")
            commit("Maintenance")
            previous = Path.cwd()
            try:
                os.chdir(directory)
                self.assertEqual(release.plan("refs/heads/main", "patch", (0, 1, 0)), ("v0.1.1", False))
                git("checkout", "--detach")
                (root / "src/VERSION").write_text("0.1.1\n")
                commit("Release v0.1.1")
                git("tag", "v0.1.1")
                release.validate_history("refs/tags/v0.1.1", "main")
                self.assertEqual(release.plan("refs/tags/v0.1.1", "patch", (0, 1, 1)), ("v0.1.1", False))
                git("checkout", "main")
                self.assertEqual(release.plan("refs/heads/main", "patch", (0, 1, 0)), ("v0.1.1", False))
                (root / "README.md").write_text("Documentation only")
                commit("Docs")
                self.assertEqual(release.plan("refs/heads/main", "patch", (0, 1, 0)), ("", True))
                (root / "src/main.go").write_text("next change")
                commit("New application content")
                self.assertEqual(release.plan("refs/heads/main", "patch", (0, 1, 0)), ("v0.1.2", False))
            finally:
                os.chdir(previous)


if __name__ == "__main__":
    unittest.main()
