Feature: seams draws the register of a repo's seams
  The team sees every seam the system has, internal and external, with each
  one's characteristics and how well it's covered, in one place that stays up
  to date, so good practice reaches every seam, not only the familiar ones.

  Scenario: A team reads its register, gaps first
    Given a registry of three seams, and a status file that puts one of them at no coverage
    When a developer runs seams show --status status.json
    Then the uncovered seam is listed first, and each seam shows its rung

  Scenario: The register is written for the team to read
    Given a registry with internal seams and one that crosses out of the system
    When a developer runs seams render
    Then docs/SEAMS.md lists the internal seams apart from the external one, under a diagram of the components and their seams

  Scenario: A seam whose sides deploy together tops out at rung 5
    Given a registry whose seam http:backend/quotes deploys independently: false, and a status file that rates it 5
    When a developer runs seams render --status status.json
    Then docs/SEAMS.md shows http:backend/quotes at 5 of 5
    And seams show --json gives http:backend/quotes a top of 5

  Scenario: A seam whose sides deploy apart tops out at rung 6
    Given a registry whose seam http:backend/quotes deploys independently: true, and a status file that rates it 5
    When a developer runs seams render --status status.json
    Then docs/SEAMS.md shows http:backend/quotes at 5 of 6
    And seams show --json gives http:backend/quotes a top of 6

  Scenario: A seam not known to deploy apart or together tops out at rung 6
    Given a registry whose seam http:backend/quotes deploys independently: not known, and a status file that rates it 5
    When a developer runs seams render --status status.json
    Then docs/SEAMS.md shows http:backend/quotes at 5 of 6

  Scenario: A seam rated above its top shows the rung as given
    Given a registry whose seam http:backend/quotes deploys independently: false, and a status file that rates it 6
    When a developer runs seams render --status status.json
    Then docs/SEAMS.md shows http:backend/quotes at 6 of 5
