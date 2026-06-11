#!/bin/sh
# image-storage-meta.sh — an s3s object-metadata plugin template.
#
# Extracts the image id from the object key (the leading 32-hex-char run),
# queries an image-storage service with the CALLER'S OWN token, and maps the
# answer onto contract-v1 fields. This stub fakes the service answer so it
# works out of the box; replace the `lookup_image` body with your API call.
#
# Declare it in ~/.config/s3s/config.yaml:
#
#   plugins:
#     - name: image-storage-meta
#       capability: object-metadata
#       cmd: "/path/to/image-storage-meta.sh"
#       timeout: 3s
#       match:
#         connections: [my-connection]
#         buckets: ["images-*"]
#         keyPattern: "^[0-9a-f]{32}"
#
# Exchange (see docs/plugins/README.md):
#   stdin  — {"contractVersion":1,"capability":"object-metadata",
#             "connection":{...},"target":{"bucket":"...","key":"..."}}
#   stdout — {"contractVersion":1,"fields":[{"name":"...","value":"..."}]}
#            or {"contractVersion":1,"error":"reason"}
#            or {"contractVersion":1,"fields":[]} for "nothing known"

REQUEST=$(cat)

# Pull target.key out of the request. jq is the robust way:
#   KEY=$(printf '%s' "$REQUEST" | jq -r .target.key)
# This dependency-free fallback handles the simple flat envelope:
KEY=$(printf '%s' "$REQUEST" | sed -n 's/.*"key"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')

IMAGE_ID=$(printf '%s' "$KEY" | sed -n 's/^\([0-9a-f]\{32\}\).*/\1/p')
if [ -z "$IMAGE_ID" ]; then
  printf '{"contractVersion":1,"fields":[]}\n' # key carries no image id — nothing known
  exit 0
fi

# Replace with the real lookup, authenticating with YOUR token (s3s passes no
# credential material):
#   curl -sf -H "Authorization: Bearer $IMAGE_STORAGE_TOKEN" \
#     "https://image-storage.example/api/v1/images/$IMAGE_ID"
lookup_image() {
  printf '{"width":1024,"height":768,"moderation":"approved","owner":"avito-images"}'
}

INFO=$(lookup_image) || {
  printf '{"contractVersion":1,"error":"image-storage lookup failed"}\n'
  exit 0
}

# Map the service answer onto display fields (order is preserved by s3s).
WIDTH=$(printf '%s' "$INFO" | sed -n 's/.*"width"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
HEIGHT=$(printf '%s' "$INFO" | sed -n 's/.*"height"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p')
MOD=$(printf '%s' "$INFO" | sed -n 's/.*"moderation"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
OWNER=$(printf '%s' "$INFO" | sed -n 's/.*"owner"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')

printf '{"contractVersion":1,"fields":[
  {"name":"Image ID","value":"%s"},
  {"name":"Dimensions","value":"%sx%s"},
  {"name":"Moderation","value":"%s"},
  {"name":"Owner service","value":"%s"}
]}\n' "$IMAGE_ID" "$WIDTH" "$HEIGHT" "$MOD" "$OWNER"
