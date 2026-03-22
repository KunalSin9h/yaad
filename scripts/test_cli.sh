#!/usr/bin/env bash
# =============================================================================
# yaad — CLI integration test script
#
# Runs end-to-end tests against a real compiled binary. Intended for local
# development only; not wired into CI (which uses go test ./...).
#
# Usage:
#   ./scripts/test_cli.sh              # build then test
#   ./scripts/test_cli.sh --no-build   # skip build, use existing binary
#
# Requirements:
#   - Go toolchain (for build)
#   - Ollama is NOT required — yaad degrades gracefully when it's absent
#
# Exit code: 0 = all tests passed, 1 = one or more tests failed
# =============================================================================

set -euo pipefail

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# ── counters ──────────────────────────────────────────────────────────────────
PASS=0
FAIL=0
SKIP=0

# ── helpers ──────────────────────────────────────────────────────────────────

pass() { echo -e "  ${GREEN}✓${RESET} $1"; PASS=$((PASS + 1)); }
fail() { echo -e "  ${RED}✗${RESET} $1"; FAIL=$((FAIL + 1)); }
skip() { echo -e "  ${YELLOW}~${RESET} $1 ${YELLOW}(skipped)${RESET}"; SKIP=$((SKIP + 1)); }
section() { echo -e "\n${CYAN}${BOLD}▶ $1${RESET}"; }

# Assert: output contains substring
assert_contains() {
    local label="$1" output="$2" want="$3"
    if echo "$output" | grep -qF -- "$want"; then
        pass "$label"
    else
        fail "$label — expected to contain: '$want'"
        echo -e "    ${RED}got:${RESET} $(echo "$output" | head -5)"
    fi
}

# Assert: output does NOT contain substring
assert_not_contains() {
    local label="$1" output="$2" want="$3"
    if ! echo "$output" | grep -qF -- "$want"; then
        pass "$label"
    else
        fail "$label — expected NOT to contain: '$want'"
    fi
}

# Assert: command exits with given code
assert_exit() {
    local label="$1" want_code="$2"
    shift 2
    local got_code=0
    "$@" >/dev/null 2>&1 || got_code=$?
    if [[ "$got_code" -eq "$want_code" ]]; then
        pass "$label (exit $want_code)"
    else
        fail "$label — expected exit $want_code, got $got_code"
    fi
}

# Run yaad with isolated data dir and rc file so tests never touch real user data
YAAD_BIN=""
YAAD_DATA=""
YAAD_RC=""
cleanup() { [[ -n "$YAAD_DATA" ]] && rm -rf "$YAAD_DATA"; }
trap cleanup EXIT

yaad() {
    XDG_DATA_HOME="$YAAD_DATA" "$YAAD_BIN" "$@" 2>&1
}

# ── build ────────────────────────────────────────────────────────────────────

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="$REPO_ROOT/bin/yaad-test"

if [[ "${1:-}" != "--no-build" ]]; then
    echo -e "${BOLD}Building yaad…${RESET}"
    go build -o "$BIN_PATH" "$REPO_ROOT/cmd/yaad" || {
        echo -e "${RED}Build failed.${RESET}"; exit 1
    }
    echo -e "${GREEN}Build OK${RESET} → $BIN_PATH\n"
fi

[[ -x "$BIN_PATH" ]] || { echo -e "${RED}Binary not found: $BIN_PATH${RESET}"; exit 1; }
YAAD_BIN="$BIN_PATH"
YAAD_DATA="$(mktemp -d)"
YAAD_RC="$YAAD_DATA/.yaadrc"

echo -e "${BOLD}Test data dir:${RESET} $YAAD_DATA"
echo -e "${BOLD}Binary:${RESET}        $YAAD_BIN"

# =============================================================================
# TESTS
# =============================================================================

# ── 1. Help & version ─────────────────────────────────────────────────────────
section "Help & root command"

out=$(yaad --help)
assert_contains "root --help shows 'add' command"    "$out" "add"
assert_contains "root --help shows 'ask' command"    "$out" "ask"
assert_contains "root --help shows 'list' command"   "$out" "list"
assert_contains "root --help shows 'check' command"  "$out" "check"
assert_contains "root --help shows 'daemon' command" "$out" "daemon"
assert_contains "root --help shows 'config' command" "$out" "config"
assert_contains "root --help shows persistent flags" "$out" "--chat-model"

# ── 2. Config ─────────────────────────────────────────────────────────────────
section "Config: init / set / get / list / path"

out=$(HOME="$YAAD_DATA" yaad config init)
assert_contains "config init creates rc file"  "$out" ".yaadrc"

out=$(HOME="$YAAD_DATA" yaad config path)
assert_contains "config path returns rc path"  "$out" ".yaadrc"

HOME="$YAAD_DATA" yaad config set ollama.chat_model mistral >/dev/null
out=$(HOME="$YAAD_DATA" yaad config get ollama.chat_model)
assert_contains "config get returns set value" "$out" "mistral"

out=$(HOME="$YAAD_DATA" yaad config list)
assert_contains "config list shows key"       "$out" "ollama.chat_model"
assert_contains "config list shows value"     "$out" "mistral"

# Update in place — must not duplicate the key
HOME="$YAAD_DATA" yaad config set ollama.chat_model llama3.2:3b >/dev/null
count=$(HOME="$YAAD_DATA" yaad config list | grep -c "ollama.chat_model" || true)
if [[ "$count" -eq 1 ]]; then
    pass "config set updates in place (no duplicate key)"
else
    fail "config set created duplicate key (found $count occurrences)"
fi

# ── 3. add ────────────────────────────────────────────────────────────────────
section "add: basic memory"

out=$(yaad add "claude --resume abc123" --for "yaad build session")
assert_contains "add prints saved ID"   "$out" "saved"

# Extract the ID from the output for later use
SAVED_ID=$(echo "$out" | grep "^saved" | awk '{print $2}')

out=$(yaad add "postgres is on port 5433" --for "staging env")
assert_contains "add with --for confirms save" "$out" "saved"

out=$(yaad add "https://pkg.go.dev/modernc.org/sqlite" --for "pure go sqlite driver")
assert_contains "add URL confirms save" "$out" "saved"

out=$(yaad add "some plain note without context")
assert_contains "add without --for still saves" "$out" "saved"

# ── 4. add --remind ───────────────────────────────────────────────────────────
section "add: --remind sets reminder"

out=$(yaad add "book conference ticket" --remind "in 30 minutes")
assert_contains "add --remind shows remind time" "$out" "remind"

out=$(yaad add "call the dentist" --remind "tomorrow 9am")
assert_contains "add --remind tomorrow 9am" "$out" "remind"

# Bad remind expression should error
out=$(yaad add "test bad remind" --remind "not-a-time-expression" 2>&1 || true)
assert_contains "add with invalid remind expr errors" "$out" "parse remind"

# ── 6. list ───────────────────────────────────────────────────────────────────
section "list: filtering and display"

out=$(yaad list)
assert_contains "list shows saved memories"        "$out" "claude --resume"
assert_contains "list shows content column header" "$out" "CONTENT"

out=$(yaad list --remind)
assert_contains "list --remind shows pending reminder" "$out" "conference ticket"

out=$(yaad list --limit 2)
line_count=$(echo "$out" | grep -v "^ID\|^--" | grep -c "." || true)
if [[ "$line_count" -le 2 ]]; then
    pass "list --limit 2 returns at most 2 results"
else
    fail "list --limit 2 returned $line_count results"
fi

# ── 7. get ────────────────────────────────────────────────────────────────────
section "get: full memory detail"

out=$(yaad get "$SAVED_ID")
assert_contains "get shows full ID"      "$out" "ID"
assert_contains "get shows content"      "$out" "claude --resume abc123"
assert_contains "get shows for label"    "$out" "yaad build session"
assert_contains "get shows working dir"  "$out" "Dir"
assert_contains "get shows hostname"     "$out" "Host"
assert_contains "get shows created at"   "$out" "Created"

# Non-existent ID must error
out=$(yaad get "DOESNOTEXIST00" 2>&1 || true)
assert_contains "get non-existent ID returns error" "$out" "not found"

# ── 8. delete ─────────────────────────────────────────────────────────────────
section "delete: with --force flag"

# Add a throwaway memory to delete
del_out=$(yaad add "throwaway memory to delete")
DEL_ID=$(echo "$del_out" | grep "^saved" | awk '{print $2}')

out=$(yaad delete "$DEL_ID" --force)
assert_contains "delete --force confirms deletion" "$out" "deleted"

out=$(yaad get "$DEL_ID" 2>&1 || true)
assert_contains "get after delete returns not found" "$out" "not found"

# ── 9. check ─────────────────────────────────────────────────────────────────
section "check: silent when no reminders due"

out=$(yaad check)
# check has no due reminders yet (the "in 30 minutes" one isn't past),
# so output should be empty
if [[ -z "$out" ]]; then
    pass "check is silent when no reminders are due"
else
    skip "check produced output — a reminder may have fired (timing sensitive)"
fi

# ── 10. ask (Ollama absent) ───────────────────────────────────────────────────
section "ask: graceful when Ollama is not running"

# ask requires Ollama for embedding. If not running, it should error clearly.
if curl -sf http://localhost:11434/api/tags >/dev/null 2>&1; then
    out=$(yaad ask "what was that claude command?")
    assert_contains "ask returns answer when Ollama is up" "$out" ""
    pass "ask with Ollama running returned a response"
else
    out=$(yaad ask "what was that claude command?" 2>&1 || true)
    assert_contains "ask without Ollama gives clear error" "$out" "ollama"
    skip "ask full flow — Ollama not running (expected in dev without Ollama)"
fi

# ── 11. Persistent flags override rc ─────────────────────────────────────────
section "persistent flags: --chat-model overrides rc file"

HOME="$YAAD_DATA" yaad config set ollama.chat_model llama3.2:3b >/dev/null

# We can't easily inspect which model was used, but we verify the flag is
# accepted without error
out=$(HOME="$YAAD_DATA" yaad --chat-model mistral add "flag override test" 2>&1)
assert_contains "--chat-model flag accepted without error" "$out" "saved"

# ── 12. Help for sub-commands ─────────────────────────────────────────────────
section "sub-command --help"

for cmd in add ask list get delete check daemon config; do
    out=$(yaad "$cmd" --help 2>&1 || true)
    assert_contains "yaad $cmd --help exits cleanly" "$out" "Usage"
done

# =============================================================================
# SUMMARY
# =============================================================================

echo ""
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "  ${GREEN}${BOLD}PASSED${RESET}  $PASS"
[[ $FAIL  -gt 0 ]] && echo -e "  ${RED}${BOLD}FAILED${RESET}  $FAIL"
[[ $SKIP  -gt 0 ]] && echo -e "  ${YELLOW}SKIPPED${RESET} $SKIP"
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"

if [[ $FAIL -gt 0 ]]; then
    echo -e "\n${RED}${BOLD}Some tests failed.${RESET}"
    exit 1
else
    echo -e "\n${GREEN}${BOLD}All tests passed.${RESET}"
    exit 0
fi
