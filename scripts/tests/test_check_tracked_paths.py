"""Portability checks use synthetic names, so they run on every host OS."""

import subprocess
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from check_tracked_paths import check_paths


class TrackedPathsTests(unittest.TestCase):
    def test_portable_names(self):
        self.assertEqual(check_paths([".github/ci.yml", "src/CONSOLE.go", "data/café.csv"]), [])

    def test_reserved_characters_and_controls(self):
        for path in ["xd:/lsp", 'a/quo"te', "a/back\\slash", "a/q?", "a/star*", "a/pipe|", "a/<x>", "a/new\nline", "a/\x01"]:
            with self.subTest(path=path):
                self.assertTrue(check_paths([path]))

    def test_device_names_with_extensions(self):
        for name in ["CON", "nul.txt", "AuX.json", "PRN", "COM1.log", "LPT9", "COM¹.txt", "LPT²", "NUL .txt"]:
            with self.subTest(name=name):
                self.assertTrue(check_paths(["dir/" + name]))

    def test_trailing_spaces_periods_and_empty_components(self):
        for path in ["a.", "a /file", "a/file ", "/absolute", "a//b", "a/../b", "a/./b"]:
            with self.subTest(path=path):
                self.assertTrue(check_paths([path]))

    def test_case_collisions_include_directory_components(self):
        for paths in [["A.go", "a.go"], ["Src/a.go", "src/b.go"], ["A", "a/file"]]:
            with self.subTest(paths=paths):
                self.assertTrue(check_paths(paths))

    def test_shared_directories_are_allowed(self):
        self.assertEqual(check_paths(["src/a.go", "src/b.go", "src/sub/c.go"]), [])

    def test_exact_file_directory_collision(self):
        self.assertTrue(check_paths(["src", "src/a.go"]))

    def test_input_order_does_not_change_diagnostics(self):
        paths = ["Src/a.go", "src/b.go", "xd:/lsp"]
        self.assertEqual(check_paths(paths), check_paths(list(reversed(paths))))

    def test_nul_input_and_exit_status(self):
        script = Path(__file__).resolve().parents[1] / "check_tracked_paths.py"
        result = subprocess.run([sys.executable, str(script), "--stdin"], input=b"xd:/lsp\0", capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn(b"xd:/lsp", result.stderr)
        self.assertEqual(result.stdout, b"")
        result = subprocess.run([sys.executable, str(script), "--stdin"], input=b"src/ok.go\0", capture_output=True)
        self.assertEqual(result.returncode, 0)
        self.assertIn(b"OK", result.stdout)


if __name__ == "__main__":
    unittest.main()
