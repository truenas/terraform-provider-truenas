#!/usr/bin/env bash
# Shared helpers for scripts/hunt/*.sh. Source it; do not run it.
#
#   hunt_repo_root          print the repo root (works from any cwd)
#   hunt_env_dir            directory holding the git-ignored env files
#                           ($TFHUNT_ENV_DIR, default <repo>/.hunt-env)
#   hunt_work_dir           scratch dir for builds/runs/logs
#                           ($TFHUNT_WORK_DIR, default <repo>/.hunt-work)
#   hunt_help "$0"          print the script's leading comment block
#   hunt_want_help "$@"     true when the first arg is -h/--help
#   hunt_source_env <ip>    export every env file for one box (see README)
#   hunt_require VAR ...    fail unless each variable is set and non-empty
#
# Env files (never committed; .hunt-env/ is git-ignored):
#   env.<ip>.sh        TRUENAS_ENDPOINT TRUENAS_API_KEY TRUENAS_USERNAME TRUENAS_TEST_POOL   (required)
#   env.s3.sh          TF_ACC_S3_ENDPOINT TF_ACC_S3_ACCESS TF_ACC_S3_SECRET TF_ACC_S3_BUCKET  (optional)
#   env.keytab.sh      TRUENAS_DS_KEYTAB_B64                                                  (optional)
#   env.registry.sh    TRUENAS_TEST_REGISTRY_* TFHUNT_REG_URI                                 (optional)
#   env.ro.<ip>.sh     TRUENAS_RO_* TFHUNT_REG_ID (written by build-ro-box.sh)               (optional)
# If env.<ip>.sh is missing but TRUENAS_ENDPOINT is already exported, the
# exported environment is used as-is.

hunt_repo_root() {
  git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel
}

hunt_env_dir() {
  printf '%s\n' "${TFHUNT_ENV_DIR:-$(hunt_repo_root)/.hunt-env}"
}

hunt_work_dir() {
  local d="${TFHUNT_WORK_DIR:-$(hunt_repo_root)/.hunt-work}"
  mkdir -p "$d"
  printf '%s\n' "$d"
}

# Print the comment block that follows the shebang of the given script.
hunt_help() {
  sed -n '2,/^[^#]/p' "$1" | grep '^#' | sed -E 's/^# ?//'
}

hunt_want_help() {
  [ "${1:-}" = -h ] || [ "${1:-}" = --help ]
}

hunt_require() {
  local v
  for v in "$@"; do
    if [ -z "${!v:-}" ]; then
      echo "error: $v is not set" >&2
      return 1
    fi
  done
}

# Export every env file that applies to one box. Only env.<ip>.sh is
# required (unless TRUENAS_ENDPOINT is already in the environment).
hunt_source_env() {
  local ip=$1 d f
  d=$(hunt_env_dir)
  set -a
  if [ -f "$d/env.$ip.sh" ]; then
    # shellcheck disable=SC1090
    . "$d/env.$ip.sh"
  elif [ -z "${TRUENAS_ENDPOINT:-}" ]; then
    echo "error: $d/env.$ip.sh not found and TRUENAS_ENDPOINT is not set" >&2
    echo "       (see scripts/hunt/README.md for the env file layout)" >&2
    set +a
    return 1
  fi
  for f in env.s3.sh env.keytab.sh env.registry.sh "env.ro.$ip.sh"; do
    # shellcheck disable=SC1090
    [ -f "$d/$f" ] && . "$d/$f"
  done
  set +a
  return 0
}
