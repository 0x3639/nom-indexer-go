# Pagination

The REST API has not been implemented yet, so pagination parameters and
response envelopes are not part of the public contract.

When endpoints are added, define pagination in `docs/api/openapi.yaml`
first and mirror the rules here. Until then, consumers should query the
database tables directly and use the indexes documented in
[`docs/schema/`](../schema/index.md).
