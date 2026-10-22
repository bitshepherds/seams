Feature: seams says which version it is
  A developer, or a tool that pins seams, can tell which release is installed.

  Scenario: A developer asks seams for its version
    Given seams is installed
    When a developer runs seams --version
    Then seams prints its version
