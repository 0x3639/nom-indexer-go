# Authentication

The REST API has not been implemented yet, so there is no runtime
authentication behavior to document.

Current expectation: the first API server will be read-only and deployed
behind infrastructure-level access controls (reverse proxy, VPN, or
private network). If application-level auth is added later, this page
should document the accepted headers, failure responses, and local
development defaults alongside the OpenAPI contract.
