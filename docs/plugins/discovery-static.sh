#!/bin/sh
# discovery-static.sh — a minimal s3s bucket-discovery plugin.
#
# Emits bucket names from a flat file (one name per line) as a contract-v1
# discovery response. Use it as the template for wiring a real provisioning
# API: replace the `cat "$NAMES_FILE"` source with your API call.
#
# Declare it in ~/.config/s3s/config.yaml:
#
#   plugins:
#     - name: static-discovery
#       capability: bucket-discovery
#       cmd: "/path/to/discovery-static.sh /path/to/buckets.txt"
#       connections: [my-connection]
#
# Exchange (see docs/plugins/README.md for the full contract):
#   stdin  — one JSON request: {"contractVersion":1,"capability":"bucket-discovery",
#            "connection":{"name":...,"endpoint":...,"userLabel":...,"accessKeyId":...}}
#   stdout — one JSON response: {"contractVersion":1,"buckets":["a","b"]}
#   exit 0 — response written (success or soft {"error":"reason"})
#   nonzero — fatal; s3s reports exec_error

NAMES_FILE="${1:?usage: discovery-static.sh <names-file>}"

# The request arrives on stdin. This stub doesn't need it, but a real plugin
# would read it to learn which connection is asking, e.g. (with jq):
#   REQUEST=$(cat)
#   ENDPOINT=$(printf '%s' "$REQUEST" | jq -r .connection.endpoint)
#   ACCESS_KEY=$(printf '%s' "$REQUEST" | jq -r .connection.accessKeyId)
# Template for a provisioning API call (use YOUR OWN token — s3s never passes
# credential material):
#   curl -sf -H "Authorization: Bearer $MY_TOKEN" \
#     "https://provisioning.example/api/buckets?user=$ACCESS_KEY"
cat >/dev/null

if [ ! -r "$NAMES_FILE" ]; then
  printf '{"contractVersion":1,"error":"names file %s is not readable"}\n' "$NAMES_FILE"
  exit 0
fi

# Build {"contractVersion":1,"buckets":["a","b",...]} from the line-per-name file.
printf '{"contractVersion":1,"buckets":['
first=1
while IFS= read -r name; do
  [ -z "$name" ] && continue
  if [ $first -eq 1 ]; then first=0; else printf ','; fi
  printf '"%s"' "$name"
done < "$NAMES_FILE"
printf ']}\n'
