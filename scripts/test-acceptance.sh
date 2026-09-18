#!/usr/bin/env bash

set -euo pipefail

test_host="http://localhost:17700"
test_api_key="T35T-M45T3R-K3Y"
meilisearch_image="getmeili/meilisearch:v1.53.2"
terraform_binary="terraform"
test_pattern='^TestAcc'
container_name=""
temp_root=""

usage() {
	cat <<'EOF'
Usage: scripts/test-acceptance.sh [options]

Starts getmeili/meilisearch:v1.53.2 on http://localhost:17700, seeds only
synthetic index/key metadata, runs the provider acceptance tests, and removes
the disposable container and generated Terraform state.

Options:
  --terraform-bin PATH  Terraform CLI executable (default: terraform)
  --run REGEXP          Go test regexp (default: ^TestAcc)
  --help                Show this message
EOF
}

while (($# > 0)); do
	case "$1" in
		--terraform-bin)
			if (($# < 2)); then
				echo "--terraform-bin requires a path" >&2
				exit 2
			fi
			terraform_binary="$2"
			shift 2
			;;
		--run)
			if (($# < 2)); then
				echo "--run requires a regexp" >&2
				exit 2
			fi
			test_pattern="$2"
			shift 2
			;;
		--help)
			usage
			exit 0
			;;
		*)
			echo "unknown option: $1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [[ "$terraform_binary" == */* ]]; then
	if [[ ! -x "$terraform_binary" ]]; then
		echo "Terraform executable is not executable: $terraform_binary" >&2
		exit 2
	fi
else
	terraform_binary="$(command -v "$terraform_binary" || true)"
	if [[ -z "$terraform_binary" || ! -x "$terraform_binary" ]]; then
		echo "Terraform executable not found" >&2
		exit 2
	fi
fi

if ! command -v docker >/dev/null 2>&1; then
	echo "Docker is required for acceptance tests" >&2
	exit 2
fi

if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
	echo "curl and jq are required for acceptance fixture setup" >&2
	exit 2
fi

script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
container_name="terraform-provider-meilisearch-acceptance-$$"
temp_parent="${TMPDIR:-/tmp}"
temp_root="$(mktemp -d "$temp_parent/terraform-provider-meilisearch-acceptance.XXXXXX")"
go_cache="${GOCACHE:-$temp_root/go-cache}"
go_mod_cache="${GOMODCACHE:-$temp_root/go-modcache}"

cleanup() {
	local status=$?
	local cleanup_status=0

	trap - EXIT INT TERM

	if [[ -n "$container_name" ]]; then
		docker rm --force "$container_name" >/dev/null 2>&1 || true
	fi
	if [[ -n "$temp_root" ]]; then
		if ! chmod -R u+w "$temp_root"; then
			echo "failed to make acceptance test temporary directory writable: $temp_root" >&2
			cleanup_status=1
		fi
		if ! rm -f -R "$temp_root"; then
			echo "failed to clean up acceptance test temporary directory: $temp_root" >&2
			cleanup_status=1
		fi
	fi
	if ((status == 0 && cleanup_status != 0)); then
		status=$cleanup_status
	fi
	exit "$status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

: > "$temp_root/terraform.tfrc"
cd -- "$repo_root"

echo "Starting $meilisearch_image on $test_host"
docker run \
	--detach \
	--name "$container_name" \
	--publish 17700:7700 \
	--env "MEILI_MASTER_KEY=$test_api_key" \
	"$meilisearch_image" \
	meilisearch \
	--no-analytics >/dev/null

for attempt in {1..60}; do
	if curl --silent --fail --output /dev/null \
		--header "Authorization: Bearer $test_api_key" \
		"$test_host/health"; then
		break
	fi
	container_status="$(docker inspect --format '{{.State.Status}}' "$container_name" 2>/dev/null || true)"
	if [[ "$container_status" == "exited" || "$container_status" == "dead" ]]; then
		docker logs --tail 80 "$container_name" >&2
		exit 1
	fi
	if ((attempt == 60)); then
		echo "Meilisearch did not become healthy" >&2
		exit 1
	fi
	sleep 1
done

server_version="$(curl --silent --show-error --fail \
	--header "Authorization: Bearer $test_api_key" \
	"$test_host/version")"
if [[ "$(jq --raw-output '.pkgVersion // empty' <<<"$server_version")" != "1.53.2" ]]; then
	echo "unexpected Meilisearch version: $(jq --raw-output '.pkgVersion // empty' <<<"$server_version")" >&2
	exit 1
fi

wait_task() {
	local task_uid="$1"
	local task_json
	local task_status

	for attempt in {1..60}; do
		task_json="$(curl --silent --show-error --fail \
			--header "Authorization: Bearer $test_api_key" \
			"$test_host/tasks/$task_uid")"
		task_status="$(jq --raw-output '.status // empty' <<<"$task_json")"
		case "$task_status" in
			succeeded)
				return 0
				;;
			failed|canceled)
				echo "fixture task $task_uid $task_status: $task_json" >&2
				return 1
				;;
		esac
		sleep 1
	done

	echo "fixture task $task_uid did not complete: $task_json" >&2
	return 1
}

create_index() {
	local payload="$1"
	local response
	local task_uid

	response="$(curl --silent --show-error --fail \
		--request POST \
		--header "Authorization: Bearer $test_api_key" \
		--header 'Content-Type: application/json' \
		--data "$payload" \
		"$test_host/indexes")"
	task_uid="$(jq --raw-output '.taskUid // empty' <<<"$response")"
	if [[ -z "$task_uid" ]]; then
		echo "index fixture creation returned no task UID: $response" >&2
		exit 1
	fi
	wait_task "$task_uid"
}

create_index '{"uid":"test_index","primaryKey":"test_id"}'
create_index '{"uid":"test_index_no_primary_key"}'
create_index '{"uid":"test_index_1"}'
create_index '{"uid":"test_index_2"}'
create_index '{"uid":"products"}'
create_index '{"uid":"users"}'

curl --silent --show-error --fail \
	--request POST \
	--header "Authorization: Bearer $test_api_key" \
	--header 'Content-Type: application/json' \
	--data '{"uid":"11111111-2222-3333-4444-555555555555","name":"test_api_key","description":"Test API key","actions":["documents.add"],"indexes":["products","users"],"expiresAt":"2042-04-02T00:42:42Z"}' \
	--output /dev/null \
	"$test_host/keys"

echo "Running acceptance tests with Terraform $terraform_binary"
	TF_ACC=1 \
	TF_ACC_TERRAFORM_PATH="$terraform_binary" \
	TF_CLI_CONFIG_FILE="$temp_root/terraform.tfrc" \
	TF_DATA_DIR="$temp_root/terraform-data" \
	MEILISEARCH_HOST="$test_host" \
	MEILISEARCH_API_KEY="$test_api_key" \
	GOCACHE="$go_cache" \
	GOMODCACHE="$go_mod_cache" \
	TMPDIR="$temp_root" \
	GOTOOLCHAIN=local \
	go test ./internal/provider -run "$test_pattern" -count=1 -v
