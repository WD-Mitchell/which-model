from __future__ import annotations

import csv
import io
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parent.parent
SCRIPT = (
    REPOSITORY_ROOT
    / ".github/workflows/update_available_model_data/generate_scores.py"
)
RAW_VALUES = REPOSITORY_ROOT / "available_model_raw_values.csv"
GENERATED_SCORES = REPOSITORY_ROOT / "available_model_scores.csv"
RAW_HEADER = (
    "model,reasoning,intelligence_index,time_per_intelligence_index_task_seconds,"
    "cost_per_intelligence_index_task_usd,median_end_to_end_response_time_seconds,"
    "artificial_analysis_coding_index,artificial_analysis_agentic_index,"
    "benchmark:SWE-Bench Verified,benchmark:GPQA Diamond,"
    "benchmark:Terminal-Bench,benchmark:AIME"
)


def raw_with_value(column: str, value: str) -> str:
    source = io.StringIO(RAW_VALUES.read_text(encoding="utf-8"))
    reader = csv.DictReader(source)
    rows = list(reader)
    rows[0][column] = value
    destination = io.StringIO(newline="")
    writer = csv.DictWriter(destination, fieldnames=reader.fieldnames, lineterminator="\n")
    writer.writeheader()
    writer.writerows(rows)
    return destination.getvalue()


class GenerateAvailableModelScoresTests(unittest.TestCase):
    def run_generator(self, input_path: Path, output_path: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--input", str(input_path), "--output", str(output_path)],
            check=False,
            capture_output=True,
            text=True,
        )

    def read_csv(self, path: Path) -> list[dict[str, str]]:
        with path.open(encoding="utf-8", newline="") as source:
            return list(csv.DictReader(source))

    def test_dynamic_benchmarks_normalize_independently_with_exact_headers(self) -> None:
        columns = [
            "model",
            "reasoning",
            "intelligence_index",
            "time_per_intelligence_index_task_seconds",
            "cost_per_intelligence_index_task_usd",
            "median_end_to_end_response_time_seconds",
            "artificial_analysis_coding_index",
            "artificial_analysis_agentic_index",
            "benchmark:SWE-Bench, Verified",
            "benchmark:τ³ Banking",
        ]
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "raw.csv"
            output = Path(directory) / "scores.csv"
            with source.open("w", encoding="utf-8", newline="") as stream:
                writer = csv.DictWriter(stream, fieldnames=columns, lineterminator="\n")
                writer.writeheader()
                writer.writerows(
                    [
                        {
                            "model": f"M{index}",
                            "reasoning": "high",
                            "intelligence_index": str(10 + index),
                            "time_per_intelligence_index_task_seconds": str(3 - index),
                            "cost_per_intelligence_index_task_usd": str(3 - index),
                            "median_end_to_end_response_time_seconds": str(30 - index),
                            "artificial_analysis_coding_index": "",
                            "artificial_analysis_agentic_index": "",
                            "benchmark:SWE-Bench, Verified": str(value),
                            "benchmark:τ³ Banking": str(100 - value),
                        }
                        for index, value in enumerate((10, 40, 90))
                    ]
                )
            result = self.run_generator(source, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = self.read_csv(output)
            with output.open(encoding="utf-8", newline="") as stream:
                headers = next(csv.reader(stream))
        self.assertEqual(
            headers[-2:],
            ["benchmark:SWE-Bench, Verified", "benchmark:τ³ Banking"],
        )
        self.assertEqual([row[headers[-2]] for row in rows], ["0", "38", "100"])
        self.assertEqual([row[headers[-1]] for row in rows], ["100", "63", "0"])

    def test_committed_raw_values_map_endpoints_and_current_optional_coverage(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "scores.csv"
            result = self.run_generator(RAW_VALUES, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = self.read_csv(output)

        self.assertTrue(rows)
        self.assertTrue(
            all(
                row[column]
                for row in rows
                for column in (
                    "intelligence_index_score",
                    "median_end_to_end_response_time_seconds_score",
                    "cost_per_intelligence_index_task_usd_score",
                )
            )
        )

        for score_column in (
            "intelligence_index_score",
            "median_end_to_end_response_time_seconds_score",
            "cost_per_intelligence_index_task_usd_score",
        ):
            populated = [int(row[score_column]) for row in rows if row[score_column]]
            self.assertEqual(min(populated), 0)
            self.assertEqual(max(populated), 100)

        for benchmark_score in (
            "benchmark:SWE-Bench Verified",
            "benchmark:SWE-Bench Pro",
            "benchmark:Terminal-Bench",
        ):
            self.assertTrue(any(row[benchmark_score] != "" for row in rows))

    def test_every_populated_score_is_rendered_as_a_whole_integer(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "scores.csv"
            result = self.run_generator(RAW_VALUES, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = self.read_csv(output)

        score_columns = [column for column in rows[0] if column not in {"model", "reasoning"}]
        for row in rows:
            for column in score_columns:
                value = row[column]
                if value:
                    self.assertIsNotNone(
                        re.fullmatch(r"\d+", value),
                        f"{row['model']} / {row['reasoning']} / {column}: {value!r}",
                    )

    def test_rows_without_all_tier_one_metrics_are_omitted(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "scores.csv"
            result = self.run_generator(RAW_VALUES, output)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = self.read_csv(output)

        identities = {(row["model"], row["reasoning"]) for row in rows}
        self.assertNotIn(("Claude Sonnet 5", "low"), identities)
        self.assertIn(("GPT-5.6 Sol", "medium"), identities)

    def test_committed_scores_are_deterministically_regenerated(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            first = Path(directory) / "first.csv"
            second = Path(directory) / "second.csv"
            self.assertEqual(self.run_generator(RAW_VALUES, first).returncode, 0)
            self.assertEqual(self.run_generator(RAW_VALUES, second).returncode, 0)
            self.assertEqual(first.read_bytes(), second.read_bytes())
            self.assertEqual(first.read_bytes(), GENERATED_SCORES.read_bytes())

    def test_rejects_non_numeric_populated_values(self) -> None:
        content = raw_with_value("intelligence_index", "not-a-number")
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "invalid.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("must be numeric", result.stderr)

    def test_rejects_negative_time(self) -> None:
        content = raw_with_value("time_per_intelligence_index_task_seconds", "-46")
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "negative-time.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "time_per_intelligence_index_task_seconds must not be negative",
            result.stderr,
        )

    def test_rejects_negative_cost(self) -> None:
        content = raw_with_value("cost_per_intelligence_index_task_usd", "-0.24")
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "negative-cost.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "cost_per_intelligence_index_task_usd must not be negative",
            result.stderr,
        )

    def test_rejects_malformed_rows(self) -> None:
        lines = RAW_VALUES.read_text(encoding="utf-8").splitlines()
        first_row = next(csv.reader([lines[1]]))
        lines[1] = ",".join(first_row[:-1])
        content = "\n".join(lines) + "\n"
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "malformed.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("too few values", result.stderr)

    def test_rejects_degenerate_metric_ranges(self) -> None:
        content = "\n".join(
            [
                RAW_HEADER,
                "Model A,low,1,10,0.5,30,,,,,,",
                "Model B,high,2,20,0.5,40,,,,,,",
                "",
            ]
        )
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "degenerate.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("cost_per_intelligence_index_task_usd has a degenerate range", result.stderr)

    def test_rejects_degenerate_mandatory_response_time_range(self) -> None:
        content = "\n".join(
            [
                RAW_HEADER,
                "Model A,low,1,10,0.5,30,,,,,,",
                "Model B,high,2,20,0.6,30,,,,,,",
                "",
            ]
        )
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "degenerate-response-time.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "median_end_to_end_response_time_seconds has a degenerate range",
            result.stderr,
        )

    def test_optional_benchmarks_normalize_partial_coverage_and_leave_singletons_blank(self) -> None:
        content = "\n".join(
            [
                RAW_HEADER,
                "Model A,low,1,10,0.1,30,,,10,,55,",
                "Model B,medium,2,20,0.2,40,,,20,, ,",
                "Model C,high,3,30,0.3,50,,,,,,",
                "",
            ]
        )
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "partial.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = self.read_csv(output_path)
        self.assertEqual(
            [row["benchmark:SWE-Bench Verified"] for row in rows],
            ["0", "100", ""],
        )
        self.assertEqual(
            [row["benchmark:Terminal-Bench"] for row in rows],
            ["", "", ""],
        )
        for column in (
            "benchmark:GPQA Diamond",
            "benchmark:AIME",
        ):
            self.assertEqual([row[column] for row in rows], ["", "", ""])

    def test_dynamic_benchmark_allows_non_percentage_scales(self) -> None:
        content = raw_with_value("benchmark:SWE-Bench Verified", "100.1")
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "invalid.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_artificial_analysis_optional_metrics_use_correct_direction_and_singletons_blank(self) -> None:
        content = "\n".join(
            [
                RAW_HEADER,
                "Model A,low,1,10,0.1,10,10,30,,,,",
                "Model B,medium,2,20,0.2,20,20,,,,,",
                "Model C,high,3,30,0.3,30,,,,,,",
                "",
            ]
        )
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "partial-aa.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
            self.assertEqual(result.returncode, 0, result.stderr)
            rows = self.read_csv(output_path)
        self.assertEqual(
            [row["median_end_to_end_response_time_seconds_score"] for row in rows],
            ["100", "50", "0"],
        )
        self.assertEqual(
            [row["artificial_analysis_coding_index_score"] for row in rows],
            ["0", "100", ""],
        )
        self.assertEqual(
            [row["artificial_analysis_agentic_index_score"] for row in rows],
            ["", "", ""],
        )

    def test_rejects_negative_median_response_time(self) -> None:
        content = raw_with_value("median_end_to_end_response_time_seconds", "-0.1")
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "negative-response-time.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(content, encoding="utf-8")
            result = self.run_generator(input_path, output_path)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("median_end_to_end_response_time_seconds must not be negative", result.stderr)

    def test_individual_missing_median_response_time_and_cost_omit_that_row(self) -> None:
        source = io.StringIO(RAW_VALUES.read_text(encoding="utf-8"))
        reader = csv.DictReader(source)
        rows = list(reader)
        target = next(
            row
            for row in rows
            if all(
                row[column].strip()
                for column in (
                    "intelligence_index",
                    "median_end_to_end_response_time_seconds",
                    "cost_per_intelligence_index_task_usd",
                )
            )
        )
        missing_identity = (target["model"], target["reasoning"])
        target["median_end_to_end_response_time_seconds"] = ""
        target["cost_per_intelligence_index_task_usd"] = ""
        destination = io.StringIO(newline="")
        writer = csv.DictWriter(
            destination, fieldnames=reader.fieldnames, lineterminator="\n"
        )
        writer.writeheader()
        writer.writerows(rows)
        with tempfile.TemporaryDirectory() as directory:
            input_path = Path(directory) / "missing-task-metrics.csv"
            output_path = Path(directory) / "scores.csv"
            input_path.write_text(destination.getvalue(), encoding="utf-8")
            result = self.run_generator(input_path, output_path)
            self.assertEqual(result.returncode, 0, result.stderr)
            scored = self.read_csv(output_path)
        identities = {(row["model"], row["reasoning"]) for row in scored}
        self.assertNotIn(missing_identity, identities)


if __name__ == "__main__":
    unittest.main()
