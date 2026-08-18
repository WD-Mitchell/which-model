"""Secure shared HTTP transport for model-data collectors."""

from __future__ import annotations

import ssl
import time
import urllib.error
import urllib.request
from typing import Callable, Mapping

from model_types import UpdateError


REQUEST_TIMEOUT_SECONDS = 20
MAX_RETRIES = 2


class RejectRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Reject redirects so authenticated headers can never cross origins."""

    def _reject(self, request, response, code, message, headers) -> None:
        raise urllib.error.HTTPError(
            request.full_url, code, "redirects are not allowed", headers, response
        )

    http_error_301 = _reject
    http_error_302 = _reject
    http_error_303 = _reject
    http_error_307 = _reject
    http_error_308 = _reject


def _retry_delay(retry_after: str | None, attempt: int) -> float:
    if retry_after:
        try:
            parsed = float(retry_after)
        except ValueError:
            parsed = -1
        if 0 <= parsed <= 10:
            return parsed
    return float(2**attempt)


class HttpClient:
    def __init__(
        self,
        opener: Callable[..., object] | None = None,
        sleeper: Callable[[float], None] = time.sleep,
        timeout: int = REQUEST_TIMEOUT_SECONDS,
        max_retries: int = MAX_RETRIES,
    ) -> None:
        self.opener = opener
        self.sleeper = sleeper
        self.timeout = timeout
        self.max_retries = max_retries
        self.tls_context = ssl.create_default_context()
        if hasattr(ssl, "VERIFY_X509_STRICT"):
            self.tls_context.verify_flags &= ~ssl.VERIFY_X509_STRICT
        self.network_opener = urllib.request.build_opener(
            RejectRedirectHandler(),
            urllib.request.HTTPSHandler(context=self.tls_context),
        )

    def get_text(
        self, url: str, *, headers: Mapping[str, str] | None = None, purpose: str
    ) -> str:
        request_headers = {
            "Accept": "application/json, text/html;q=0.9",
            "User-Agent": "centree-model-metrics-updater/1.0",
        }
        if headers:
            request_headers.update(headers)
        request = urllib.request.Request(url, headers=request_headers, method="GET")
        for attempt in range(self.max_retries + 1):
            try:
                opener = self.network_opener.open if self.opener is None else self.opener
                with opener(request, timeout=self.timeout) as response:
                    payload = response.read()
                    charset = response.headers.get_content_charset() or "utf-8"
                    return payload.decode(charset)
            except urllib.error.HTTPError as error:
                status = error.code
                retry_after = error.headers.get("Retry-After")
                error.close()
                if (status == 429 or 500 <= status <= 599) and attempt < self.max_retries:
                    self.sleeper(_retry_delay(retry_after, attempt))
                    continue
                messages = {
                    401: "API key was rejected (HTTP 401)",
                    403: "API access was forbidden for this key (HTTP 403)",
                    429: "rate limit exceeded (HTTP 429)",
                }
                if 300 <= status <= 399:
                    message = f"redirects are not allowed (HTTP {status})"
                elif 500 <= status <= 599:
                    message = f"server error persisted (HTTP {status})"
                else:
                    message = messages.get(status, f"HTTP {status}")
                raise UpdateError(f"failed to fetch {purpose}: {message}") from error
            except (urllib.error.URLError, TimeoutError) as error:
                if attempt < self.max_retries:
                    self.sleeper(float(2**attempt))
                    continue
                raise UpdateError(
                    f"failed to fetch {purpose}: {getattr(error, 'reason', error)}"
                ) from error
            except UnicodeDecodeError as error:
                raise UpdateError(f"failed to decode {purpose} as text") from error
        raise AssertionError("unreachable")
