#!/bin/sh -eu
[ -n "${GOPATH:-}" ] && export "PATH=${GOPATH}/bin:${PATH}"

# Don't translate lxc output for parsing in it in tests.
export LC_ALL="C"

# Force UTC for consistency
export TZ="UTC"

# Tell debconf to not be interactive
export DEBIAN_FRONTEND=noninteractive

export LOG_LEVEL_FLAG=""
if [ -n "${VERBOSE:-}" ]; then
	LOG_LEVEL_FLAG="--verbose"
fi

if [ -n "${DEBUG:-}" ]; then
	LOG_LEVEL_FLAG="--debug"
	set -x
fi

import_subdir_files() {
	test "$1"
	# shellcheck disable=SC3043
	local file
	for file in "$1"/*.sh; do
		# shellcheck disable=SC1090
		. "$file"
	done
}

import_subdir_files includes

echo "==> Checking for dependencies"
check_dependencies lxc lxd curl awk jq git python3 xgettext sqlite3 msgmerge msgfmt shuf rsync openssl

cleanup() {
	# Do not exit if commands fail on cleanup. (No need to reset -e as this is only run on test suite exit).
	set -eux
	lxc remote switch local
	lxc project switch microcloud-test
	set +e


	# Allow for inspection
	if [ -n "${CLOUD_INSPECT:-}" ]; then
		if [ "${TEST_RESULT}" != "success" ]; then
			echo "==> TEST DONE: ${TEST_CURRENT_DESCRIPTION}"
		fi
		echo "==> Test result: ${TEST_RESULT}"

		echo "Tests Completed (${TEST_RESULT}): hit enter to continue"
		read -r _
	fi

	if [ -n "${GITHUB_ACTIONS:-}" ]; then
		echo "==> Skipping cleanup (GitHub Action runner detected)"
	else
		echo "==> Cleaning up"

    cleanup_systems
	fi

	echo ""
	echo ""
	if [ "${TEST_RESULT}" != "success" ]; then
		echo "==> TEST DONE: ${TEST_CURRENT_DESCRIPTION}"
	fi
	echo "==> Test result: ${TEST_RESULT}"
}

# Must be set before cleanup()
TEST_CURRENT=setup
TEST_CURRENT_DESCRIPTION=setup
# shellcheck disable=SC2034
TEST_RESULT=failure

trap cleanup EXIT HUP INT TERM

# Import all the testsuites
import_subdir_files suites

CONCURRENT_SETUP=${CONCURRENT_SETUP:-0}
export CONCURRENT_SETUP

SKIP_SETUP_LOG=${SKIP_SETUP_LOG:-0}
export SKIP_SETUP_LOG

LXD_DEBUG_BINARY=${LXD_DEBUG_BINARY:-}
export LXD_DEBUG_BINARY

if [ -z "${MICROCLOUD_SNAP_PATH}" ]; then
  echo TODO: Setup snap build
fi

export MICROCLOUD_SNAP_PATH

run_test() {
	TEST_CURRENT="${1}"
	TEST_CURRENT_DESCRIPTION="${2:-${1}}"

	echo "==> TEST BEGIN: ${TEST_CURRENT_DESCRIPTION}"
	START_TIME="$(date +%s)"
	${TEST_CURRENT}
	END_TIME="$(date +%s)"

	echo "==> TEST DONE: ${TEST_CURRENT_DESCRIPTION} ($((END_TIME - START_TIME))s)"
}

# allow for running a specific set of tests
if [ "$#" -gt 0 ] && [ "$1" != "all" ] && [ "$1" != "cluster" ] && [ "$1" != "standalone" ]; then
	run_test "test_${1}"
	# shellcheck disable=SC2034
	TEST_RESULT=success
	exit
fi

# Create 4 nodes with 3 disks and 3 extra interfaces.
# These nodes should be used across most tests and reset with the `reset_systems` function.
new_systems 4 3 3

if [ "${1:-"all"}" != "cluster" ]; then
  run_test test_interactive "interactive"
  run_test test_service_mismatch "service mismatch"
  run_test test_disk_mismatch "disk mismatch"
  run_test test_interactive_combinations "interactive combinations"
  run_test test_auto "auto"
  run_test test_add_interactive "add interactive"
  run_test test_add_auto "add auto"
fi

# shellcheck disable=SC2034
TEST_RESULT=success
