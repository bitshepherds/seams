Feature: seams keeps a registry of a repo's seams
  Each seam has an id that names it everywhere, one provider, and its
  consumers, so every tool that reads the registry judges the same seams.

  Scenario: A registry with a malformed id is refused, naming the seam
    Given a registry with a seam whose id is quote-accepted
    When a developer runs seams validate
    Then it fails, naming quote-accepted and the form kind:provider/name

  Scenario: A developer registers a seam and finds it in the list
    Given an empty registry
    When a developer adds the seam event:backend/quote-accepted, provided by backend and consumed by billing
    Then seams list shows event:backend/quote-accepted, with backend as its provider and billing as its consumer

  Scenario: A tool reads a seam as JSON
    Given a registry with the seam http:stripe/payments
    When a tool runs seams show http:stripe/payments --json
    Then it gets the seam as JSON matching the published schema
