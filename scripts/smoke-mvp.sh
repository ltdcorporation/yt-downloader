#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"
ADMIN_USER="${ADMIN_BASIC_AUTH_USER:-}"
ADMIN_PASS="${ADMIN_BASIC_AUTH_PASS:-}"
SMOKE_URL="${SMOKE_TEST_YOUTUBE_URL:-}"
POLL_MAX="${SMOKE_POLL_MAX:-30}"
POLL_SLEEP_SECONDS="${SMOKE_POLL_SLEEP_SECONDS:-2}"

echo "[smoke] api base: ${API_BASE_URL}"

echo "[smoke] 1/4 healthz"
health_json="$(curl -fsS "${API_BASE_URL}/healthz")"
if ! echo "${health_json}" | grep -q '"ok":true'; then
	echo "[smoke] healthz failed: ${health_json}"
	exit 1
fi

echo "[smoke] 2/4 resolve validation (invalid host should return 400)"
tmp_body="$(mktemp)"
invalid_code="$(curl -sS -o "${tmp_body}" -w "%{http_code}" \
	-X POST "${API_BASE_URL}/v1/youtube/resolve" \
	-H "Content-Type: application/json" \
	-d '{"url":"https://example.com/watch?v=invalid"}')"
if [[ "${invalid_code}" != "400" ]]; then
	echo "[smoke] expected 400, got ${invalid_code}"
	cat "${tmp_body}"
	rm -f "${tmp_body}"
	exit 1
fi
rm -f "${tmp_body}"

echo "[smoke] 3/4 admin endpoint auth check"
if [[ -n "${ADMIN_USER}" && -n "${ADMIN_PASS}" ]]; then
	admin_code="$(curl -sS -o /dev/null -w "%{http_code}" \
		-u "${ADMIN_USER}:${ADMIN_PASS}" \
		"${API_BASE_URL}/admin/jobs?limit=1")"
	if [[ "${admin_code}" != "200" ]]; then
		echo "[smoke] admin auth failed, status=${admin_code}"
		exit 1
	fi
	echo "[smoke] admin ok"
else
	echo "[smoke] skip admin auth check (ADMIN_BASIC_AUTH_USER/PASS not set)"
fi

echo "[smoke] 4/4 mp3 queue flow"
if [[ -z "${SMOKE_URL}" ]]; then
	echo "[smoke] skip mp3 flow (set SMOKE_TEST_YOUTUBE_URL to enable)"
	exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
	echo "[smoke] jq is required for full mp3 smoke flow"
	exit 1
fi

create_response="$(curl -fsS \
	-X POST "${API_BASE_URL}/v1/jobs/mp3" \
	-H "Content-Type: application/json" \
	-d "$(printf '{"url":"%s"}' "${SMOKE_URL}")")"
job_id="$(echo "${create_response}" | jq -r '.job_id // empty')"
if [[ -z "${job_id}" ]]; then
	echo "[smoke] failed to create job: ${create_response}"
	exit 1
fi
echo "[smoke] job created: ${job_id}"

for attempt in $(seq 1 "${POLL_MAX}"); do
	job_json="$(curl -fsS "${API_BASE_URL}/v1/jobs/${job_id}")"
	status="$(echo "${job_json}" | jq -r '.status // empty')"
	download_url="$(echo "${job_json}" | jq -r '.download_url // empty')"
	echo "[smoke] poll ${attempt}/${POLL_MAX}: status=${status}"

	if [[ "${status}" == "done" ]]; then
		if [[ -n "${download_url}" ]]; then
			echo "[smoke] mp3 done + download url ready"
			exit 0
		fi
		echo "[smoke] status done but download_url empty"
		exit 1
	fi
	if [[ "${status}" == "failed" ]]; then
		echo "[smoke] job failed: ${job_json}"
		exit 1
	fi

	sleep "${POLL_SLEEP_SECONDS}"
done

echo "[smoke] timeout waiting job completion"
exit 1
