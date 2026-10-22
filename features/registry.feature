Feature: seams keeps a registry of a repo's seams
  Each seam has an id that names it everywhere, one provider, and its
  consumers, so every tool that reads the registry judges the same seams.

  Scenario: A registry with a malformed id is refused, naming the seam
    Given a registry with a seam whose id is quote-accepted
    When a developer runs seams validate
    Then it fails, naming quote-accepted and the form kind:provider/name
