# Bridge Indexing Update - COMPLETED ✅

## Status: Complete
All bridge indexing features have been implemented with SDK v0.1.10.

## What's Done ✅
1. Database models created (`WrapTokenRequest`, `UnwrapTokenRequest`)
2. Migration `003_bridge_requests.up.sql` - base tables
3. Migration `004_bridge_finality_fields.up.sql` - finality tracking fields
4. Repository `internal/repository/bridge.go` - full CRUD with new fields
5. Indexer logic in `internal/indexer/indexer.go` - fetches all bridge data
6. SDK updated to v0.1.10 with `ConfirmationsToFinality` and `RedeemableIn` fields

## Schema
### wrap_token_requests
- `confirmations_to_finality` - tracks confirmations remaining (0 = finalized)

### unwrap_token_requests
- `redeemable_in` - blocks until redeemable (0 = ready)
- `redeemed` - whether tokens have been redeemed
- `revoked` - whether request was revoked

## Finalization Logic
- **Wrap**: finalized when `confirmations_to_finality == 0`
- **Unwrap**: finalized when `redeemed == true` OR `revoked == true`

## Current go.mod SDK version
```
github.com/0x3639/znn-sdk-go v0.1.10
```

## RPC Response Examples (for reference)

### Wrap Token Request
```json
{
    "networkClass": 2,
    "chainId": 1,
    "id": "80d1958e462ed2f84c607bf0dbd37fdf68054c6a42317576f55b73b43f358e28",
    "toAddress": "0xc16f4e6429690de48df9c8bfd565fdcfc6fe97ea",
    "tokenStandard": "zts1znnxxxxxxxxxxxxx9z4ulx",
    "tokenAddress": "0xb2e96a63479c2edd2fd62b382c89d5ca79f572d3",
    "amount": "113248657286",
    "fee": "3397459718",
    "signature": "...",
    "creationMomentumHeight": 11938119,
    "confirmationsToFinality": 0
}
```

### Unwrap Token Request
```json
{
    "registrationMomentumHeight": 4543372,
    "networkClass": 2,
    "chainId": 1,
    "transactionHash": "00149ed5a387f0d8abdb21bd20e334d6d3b046fca08081925f8e34fa3c13534d",
    "logIndex": 74,
    "toAddress": "z1qr9vtwsfr2n0nsxl2nfh6l5esqjh2wfj85cfq9",
    "tokenAddress": "0xb2e96a63479c2edd2fd62b382c89d5ca79f572d3",
    "tokenStandard": "zts1znnxxxxxxxxxxxxx9z4ulx",
    "amount": "485000000",
    "signature": "...",
    "redeemed": 1,
    "revoked": 0,
    "redeemableIn": 0
}
```
