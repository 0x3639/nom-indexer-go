# Projects & Votes

Accelerator-Z projects, phases, and pillar votes.

## List projects — `GET /api/v1/projects`

Paginated; ordered by `creation_timestamp DESC`.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/projects | jq
```

## Get project — `GET /api/v1/projects/{id}`

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/projects/<project-id> | jq
```

## Project phases — `GET /api/v1/projects/{id}/phases`

Returns every phase in creation order (ascending). Not paginated.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/projects/<project-id>/phases | jq
```

## Project votes — `GET /api/v1/projects/{id}/votes`

Paginated. Returns votes targeting either the project directly or
any of its phases, ordered by `momentum_height DESC`.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/projects/<project-id>/votes | jq
```

`vote` values: `0` = yes, `1` = no, `2` = abstain.

## Voting report — `GET /api/v1/projects/{id}/voting-report`

Server-aggregated. Returns the project's own proposal vote AND every
phase's vote pre-tallied against the current active pillar set
(`is_revoked = false`). Each tally lists `yes_pillars`, `no_pillars`,
`abstain_pillars`, and `no_vote_pillars` by name — `no_vote_pillars`
captures active pillars who never voted on that specific proposal.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/projects/<project-id>/voting-report | jq
```

One call replaces the enumerate-pillars + paginate-`votes` +
filter-by-pillar pattern. Phases are returned in creation order.
Pillar lists are alphabetized for stable diffs.
