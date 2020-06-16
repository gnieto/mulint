#!/bin/bash

set -ex

current_out=$(mktemp)
expected_out=$(mktemp)
go run main.go ./tests | sort  > $current_out
cat tests/expected.stderr | sort > $expected_out

diff $current_out $expected_out > /dev/null
