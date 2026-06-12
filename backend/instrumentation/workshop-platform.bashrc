# Workshop Platform Shell Instrumentation
# /etc/workshop-platform.bashrc
# Container modes: sourced by /etc/bash.bashrc (Ubuntu/Debian) or /etc/bashrc (Rocky)
# Standalone mode: included in the generated rcfile passed to ttyd's shell

__workshop_log_command() {
    local exit_code=$?
    local runtime_dir="${WORKSHOP_ROOT:-/workshop}/runtime"
    local cmd
    cmd=$(history 1 | sed 's/^[ ]*[0-9]*[ ]*//')
    if [ -n "$cmd" ] && [ -d "$runtime_dir" ]; then
        local escaped_cmd
        escaped_cmd=$(printf '%s' "$cmd" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g')
        printf '{"ts":"%s","cmd":"%s","exit":%d}\n' \
            "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
            "$escaped_cmd" \
            "$exit_code" \
            >> "$runtime_dir/command-log.jsonl" 2>/dev/null || true
    fi
    return $exit_code
}

PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND; }__workshop_log_command"
export PROMPT_COMMAND
