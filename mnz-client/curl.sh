curl -X POST https://licensing-api.milaboratories.com/mnz/run-spec \
  -H "Content-Type: application/json" \
  -d \
'{
  "runSpec": {
    "asdf":"12345"
  },
  "license":"E-URA...",
  "productKey":"GAMLWGMNLAYRMFEGVLSRPRNNODLGJRZMRDDGOZJKAAXOACYF"
}'
