#!/usr/bin/env bash
# Local AIStor provisioning only; never print credentials or license contents.
set +x
set -euo pipefail
umask 077
cd "$(dirname "$0")/.."
secrets_dir=../.config
mkdir -p "$secrets_dir"
chmod 700 "$secrets_dir"
license="${STORAGE_LICENSE_FILE:-$secrets_dir/minio.license}"
[[ -f "$license" && ! -L "$license" ]] || { echo 'Missing regular license file: set STORAGE_LICENSE_FILE' >&2; exit 1; }
chmod 600 "$license"
for file in storage-root-user storage-root-password; do
  [[ ! -L "$secrets_dir/$file" ]] || { echo 'Refusing symlink credential file' >&2; exit 1; }
  if [[ ! -e "$secrets_dir/$file" ]]; then openssl rand -hex 24 > "$secrets_dir/$file"; fi
  chmod 600 "$secrets_dir/$file"
done
app_env="$secrets_dir/storage-app.env"
[[ ! -L "$app_env" ]] || { echo 'Refusing symlink credential file' >&2; exit 1; }
if [[ ! -e "$app_env" ]]; then
  printf 'TEST_STORAGE_ACCESS_KEY=%s\nTEST_STORAGE_SECRET_KEY=%s\n' "$(openssl rand -hex 16)" "$(openssl rand -hex 32)" > "$app_env"
fi
chmod 600 "$app_env"
# This file is generated locally, never sourced from an upload or request.
source "$app_env"
export STORAGE_PORT="${STORAGE_PORT:-9100}" STORAGE_CONSOLE_PORT="${STORAGE_CONSOLE_PORT:-9101}"
docker compose --profile storage up -d --wait storage
# Pass application credentials on stdin, not argv or container configuration.
{
  printf '%s\n%s\n' "$TEST_STORAGE_ACCESS_KEY" "$TEST_STORAGE_SECRET_KEY"
} | docker compose exec -T storage sh -c '
  set +x
  set -eu
  read -r app_user
  read -r app_password
  root_user=$(cat /run/secrets/storage_root_user)
  root_password=$(cat /run/secrets/storage_root_password)
  export MC_HOST_local="http://$root_user:$root_password@localhost:9000"
  policy=$(mktemp)
  trap '\''rm -f "$policy"'\'' EXIT
  cat > "$policy" << '\''POLICY'\''
{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:ListBucket"],"Resource":["arn:aws:s3:::materials"]},{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject","s3:DeleteObject","s3:AbortMultipartUpload","s3:ListMultipartUploadParts"],"Resource":["arn:aws:s3:::materials/*"]}]}
POLICY
  mc mb --ignore-existing local/materials >/dev/null
  mc anonymous set none local/materials >/dev/null
  mc admin policy create local materials-app "$policy" >/dev/null
  printf '\''%s\n%s\n'\'' "$app_user" "$app_password" | mc admin user add local >/dev/null
  mc admin policy attach local materials-app --user "$app_user" >/dev/null
' >/dev/null 2>&1 || { echo 'Storage provisioning failed; check license/network/permissions without publishing credentials' >&2; exit 1; }
export TEST_STORAGE_ENDPOINT="127.0.0.1:$STORAGE_PORT" TEST_STORAGE_BUCKET=materials
export TEST_STORAGE_ACCESS_KEY TEST_STORAGE_SECRET_KEY
go test -count=1 -v ./internal/file
