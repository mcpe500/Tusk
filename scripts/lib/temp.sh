#!/usr/bin/env bash

# Shared temporary path helpers for Termux-compatible environments.

tusk_temp_dir() {
	local candidates="${TMPDIR:-} ${PREFIX:+$PREFIX/tmp} ${HOME}/tmp /tmp"

	for dir in $candidates; do
		[ -z "$dir" ] && continue
		if mkdir -p "$dir" 2>/dev/null && [ -w "$dir" ]; then
			echo "$dir"
			return 0
		fi
	done

	if [ -n "$HOME" ]; then
		echo "$HOME"
		return 0
	fi

	echo "/tmp"
}

tusk_temp_file() {
	local prefix="${1:-tusk}"
	local suffix="${2:-}"
	local dir
	local path

	dir="$(tusk_temp_dir)"

	if command -v mktemp >/dev/null 2>&1; then
		if [ -n "$suffix" ]; then
			path=$(mktemp "$dir/${prefix}.XXXXXX${suffix}")
		else
			path=$(mktemp "$dir/${prefix}.XXXXXX")
		fi
	else
		path="$dir/${prefix}_$$$RANDOM${suffix}"
		: > "$path"
	fi

	echo "$path"
}

tusk_temp_fifo() {
	local prefix="${1:-tusk-fifo}"
	local path
	path="$(tusk_temp_file "$prefix")"
	rm -f "$path"
	echo "$path"
}
