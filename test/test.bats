#!/usr/bin/env bats

setup() {
    load '../node_modules/bats-support/load.bash'
    load '../node_modules/bats-assert/load.bash'
}


FB_BIN=${FB_BIN:-/opt/td-agent-bit/bin/td-agent-bit}
PLUGIN_SO=${PLUGIN_SO:-../out_gcs.so}
FB_OUTPUT_NAME=${FB_OUTPUT_NAME:-gcs}


@test "fluent-bit is installed" {
    run test -x "$FB_BIN"
    assert_success
}

@test "plugin loads" {
    run "$FB_BIN" -e "$PLUGIN_SO" --dry-run
    assert_success
}

@test "can stream cpu input to output (3s)" {
    run timeout --preserve-status 3 "$FB_BIN" -e "$PLUGIN_SO" -i cpu -o "$FB_OUTPUT_NAME"
    assert_output "hey i did something"
    assert_success
}
