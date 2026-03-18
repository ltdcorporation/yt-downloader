#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"
ADMIN_USER="${ADMIN_BASIC_AUTH_USER:-}"
ADMIN_PASS="${ADMIN_BASIC_AUTH_PASS:-}"
SMOKE_URL="${SMOKE_TEST_YOUTUBE_URL:-}"
POLL_MAX="${SMOKE_POLL_MAX:-30}"
POLL_SLEEP_SECONDS="${SMOKE_POLL_SLEEP_SECONDS:-2}"

json_field() {
	local field="$1"
	if command -v jq >/dev/null 2>&1; then
		jq -r ".${field} // empty"
		return 0
	fi

	python3 -c '
import json
import sys

field = sys.argv[1]
raw = sys.stdin.read()

try:
    payload = json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)

value = payload
for part in field.split("."):
    if isinstance(value, dict):
        value = value.get(part)
    else:
        value = None
        break

if value is None:
    print("")
elif isinstance(value, (dict, list)):
    print(json.dumps(value))
else:
    print(value)
' "$field"
}

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
	admin_code=""
	for admin_attempt in $(seq 1 5); do
		admin_code="$(curl -sS -o /dev/null -w "%{http_code}" \
			-u "${ADMIN_USER}:${ADMIN_PASS}" \
			"${API_BASE_URL}/admin/jobs?limit=1")"

		if [[ "${admin_code}" == "200" ]]; then
			echo "[smoke] admin ok"
			break
		fi

		if [[ "${admin_code}" == "429" ]]; then
			wait_seconds=$((admin_attempt * 2))
			echo "[smoke] admin check rate-limited (429), retry ${admin_attempt}/5 in ${wait_seconds}s"
			sleep "${wait_seconds}"
			continue
		fi

		echo "[smoke] admin auth failed, status=${admin_code}"
		exit 1
	done

	if [[ "${admin_code}" != "200" ]]; then
		echo "[smoke] admin auth failed after retries, status=${admin_code}"
		exit 1
	fi
else
	echo "[smoke] skip admin auth check (ADMIN_BASIC_AUTH_USER/PASS not set)"
fi

echo "[smoke] 4/4 mp3 queue flow"
if [[ -z "${SMOKE_URL}" ]]; then
	echo "[smoke] skip mp3 flow (set SMOKE_TEST_YOUTUBE_URL to enable)"
	exit 0
fi

create_tmp_body="$(mktemp)"
create_response=""
create_ok="false"

for create_attempt in $(seq 1 5); do
	create_code="$(curl -sS -o "${create_tmp_body}" -w "%{http_code}" \
		-X POST "${API_BASE_URL}/v1/jobs/mp3" \
		-H "Content-Type: application/json" \
		-d "$(printf '{"url":"%s"}' "${SMOKE_URL}")")"
	create_response="$(cat "${create_tmp_body}")"

	if [[ "${create_code}" == "202" || "${create_code}" == "200" ]]; then
		create_ok="true"
		break
	fi

	if [[ "${create_code}" == "429" ]]; then
		wait_seconds=$((create_attempt * 2))
		echo "[smoke] mp3 create rate-limited (429), retry ${create_attempt}/5 in ${wait_seconds}s"
		sleep "${wait_seconds}"
		continue
	fi

	echo "[smoke] failed to create job (status=${create_code}): ${create_response}"
	rm -f "${create_tmp_body}"
	exit 1

done

rm -f "${create_tmp_body}"

if [[ "${create_ok}" != "true" ]]; then
	echo "[smoke] failed to create job after retries: ${create_response}"
	exit 1
fi

job_id="$(echo "${create_response}" | json_field "job_id")"
if [[ -z "${job_id}" ]]; then
	echo "[smoke] failed to parse job_id from response: ${create_response}"
	exit 1
fi
echo "[smoke] job created: ${job_id}"

poll_tmp_body="$(mktemp)"

for attempt in $(seq 1 "${POLL_MAX}"); do
	poll_code="$(curl -sS -o "${poll_tmp_body}" -w "%{http_code}" "${API_BASE_URL}/v1/jobs/${job_id}")"
	job_json="$(cat "${poll_tmp_body}")"

	if [[ "${poll_code}" == "429" ]]; then
		echo "[smoke] poll ${attempt}/${POLL_MAX}: rate-limited (429), retrying"
		sleep "${POLL_SLEEP_SECONDS}"
		continue
	fi

	if [[ "${poll_code}" != "200" ]]; then
		echo "[smoke] poll ${attempt}/${POLL_MAX}: failed (status=${poll_code}) ${job_json}"
		rm -f "${poll_tmp_body}"
		exit 1
	fi

	status="$(echo "${job_json}" | json_field "status")"
	download_url="$(echo "${job_json}" | json_field "download_url")"
	echo "[smoke] poll ${attempt}/${POLL_MAX}: status=${status}"

	if [[ "${status}" == "done" ]]; then
		if [[ -n "${download_url}" ]]; then
			echo "[smoke] mp3 done + download url ready"
			rm -f "${poll_tmp_body}"
			exit 0
		fi
		echo "[smoke] status done but download_url empty"
		rm -f "${poll_tmp_body}"
		exit 1
	fi
	if [[ "${status}" == "failed" ]]; then
		echo "[smoke] job failed: ${job_json}"
		rm -f "${poll_tmp_body}"
		exit 1
	fi

	sleep "${POLL_SLEEP_SECONDS}"
done

rm -f "${poll_tmp_body}"

echo "[smoke] timeout waiting job completion"
exit 1
