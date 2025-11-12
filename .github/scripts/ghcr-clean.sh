#!/usr/bin/env bash
# GHCR temporary arch-tag cleanup (single-script)
# 删除命名形如 "<sha>-amd64/arm64" 的临时标签版本，保护 KEEP_TAGS 的多架构 manifest 及其子 digest。

set -Eeuxo pipefail

# -------- Config from env --------
: "${REPO:?missing REPO (owner/repo)}"
: "${OWNER:?missing OWNER}"
: "${KEEP_TAGS:?missing KEEP_TAGS}"               # e.g. "latest main"
: "${RETENTION_DAYS:?missing RETENTION_DAYS}"     # e.g. 3
: "${TEMP_TAG_REGEX:?missing TEMP_TAG_REGEX}"     # e.g. '^[0-9a-f]{40}-(amd64|arm64)$'
AUTH="${GITHUB_TOKEN:-${GH_TOKEN:-}}"

if [[ -z "${AUTH}" ]]; then
  echo "ERROR: GITHUB_TOKEN/GH_TOKEN not available. Check workflow permissions." >&2
  exit 1
fi

# lower-case owner/repo (GHCR 标准)
owner="${OWNER,,}"
image="${REPO#*/}"; image="${image,,}"
repo="${owner}/${image}"

# -------- Build protected digest set (from KEEP_TAGS) --------
# 获取 GHCR registry bearer token（用来访问 /v2/ manifests）
token="$(
  curl -fsSL -u "${GITHUB_ACTOR}:${AUTH}" \
    "https://ghcr.io/token?scope=repository:${repo}:pull" \
  | jq -r .token
)"

prot="$(mktemp)"; : > "$prot"
accept_index='application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json'
accept_manifest='application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json'

for tag in ${KEEP_TAGS}; do
  # 如果 tag 是多架构 index，收集其子清单 digests
  if json="$(curl -fsSL \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: ${accept_index}" \
        "https://ghcr.io/v2/${repo}/manifests/${tag}" 2>/dev/null || true)"; then
    [[ -n "$json" ]] && jq -r '.manifests[]?.digest' <<< "$json" | sed '/^$/d' >> "$prot" || true
  fi
  # 保护该 tag 自身的 digest（单架构或 index 顶层 digest）
  head="$(curl -fsSLI \
          -H "Authorization: Bearer ${token}" \
          -H "Accept: ${accept_index}, ${accept_manifest}" \
          "https://ghcr.io/v2/${repo}/manifests/${tag}" | tr -d '\r' || true)"
  digest="$(awk -F': ' '/^Docker-Content-Digest:/ {print $2}' <<< "$head" | tail -n1 || true)"
  [[ -n "$digest" ]] && echo "$digest" >> "$prot"
done

sort -u "$prot" -o "$prot"
PROTECTED_JSON="$(jq -Rsc 'split("\n")|map(select(length>0))' "$prot")"

# -------- Plan: list deletion candidates --------
# 组织 or 用户 命名空间
type="$(gh api "/users/${owner}" -q .type)"
if [[ "$type" == "Organization" ]]; then
  base="/orgs/${owner}"
else
  base="/users/${owner}"
fi

cutoff="$(date -u -d "${RETENTION_DAYS} days ago" +%s)"

IDS_JSON="$(
  gh api --paginate -H "Accept: application/vnd.github+json" \
     "${base}/packages/container/${image}/versions?per_page=100" \
  | jq -r --arg re "${TEMP_TAG_REGEX}" \
         --argjson prot "${PROTECTED_JSON:-[]}" \
         --argjson cutoff "$cutoff" '
      .[] | {
        id,
        digest: .name,
        updated_at,
        tags: (.metadata.container.tags // [])
      }
      | select([ .tags[]? | test($re) ] | any)               # 命中临时标签
      | select(( .updated_at | fromdateiso8601 ) < $cutoff)  # 超过保留期
      | select(($prot | index(.digest)) | not)               # 不在保护集
      | .id
    ' \
  | jq -Rs 'split("\n")|map(select(length>0))'
)"

COUNT="$(jq 'length' <<< "$IDS_JSON")"
echo "Candidates: $COUNT"

# -------- Delete --------
if [[ "$COUNT" -eq 0 ]]; then
  echo "Nothing to delete."
  exit 0
fi

# 逐个删除候选版本（注意：删除的是“版本=某 digest”，会连带其所有 tag 一起删除）
deleted=0
while IFS= read -r id; do
  [[ -n "$id" ]] || continue
  gh api -X DELETE -H "Accept: application/vnd.github+json" \
    "${base}/packages/container/${image}/versions/${id}"
  deleted=$((deleted+1))
done < <(jq -r '.[]' <<< "$IDS_JSON")

echo "Deleted=$deleted"
